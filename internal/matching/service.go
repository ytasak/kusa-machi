package matching

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
	"kusamachi/internal/db/sqlc"
)

// Service は Like・Pass・プロフィール更新のトランザクションを実行する。
// いずれも、市場のルールを強制するのがアプリケーションではなくデータベースに
// なるように書かれている。
type Service struct {
	pool  *pgxpool.Pool
	q     *sqlc.Queries
	clock clock.Clock
}

// NewService は接続プールの上にサービスを構築する。
//
// 時計を受け取るのは時間回復のため。回復の判定は「起点から3時間経ったか」で
// あって、テストでは任意の時刻を入れられる必要がある。
func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, q: sqlc.New(pool), clock: clk}
}

// LikeState は Like の残数まわりで画面が必要とする状態。
type LikeState struct {
	// Remaining は現在の所持数。サーバ側の like_balance がそのまま答えになる。
	Remaining int

	// Capacity は所持上限。残数の分母として画面に出す。定数だが、上限を
	// 決めるのはサーバであってフロントではないので応答に含める。
	Capacity int

	// NextRecoveryAt は次に時間回復が起きる時刻。タイマーを出す状態でなければ nil。
	NextRecoveryAt *time.Time

	// Recovered はこのリクエストの中で時間回復した数。画面はこれが 0 より
	// 大きいときだけ「Likeが回復しました」を出す。
	Recovered int
}

// recoveryStateOf は persona 行から時間回復の判定に必要な部分だけを取り出す。
func recoveryStateOf(p sqlc.Persona) TimeRecoveryState {
	return TimeRecoveryState{
		Balance:  int(p.LikeBalance),
		AnchorAt: p.LikeRecoveryAnchorAt,
	}
}

// likeState は反映済みの persona 行から画面向けの状態を組み立てる。
func (s *Service) likeState(p sqlc.Persona, recovered int) LikeState {
	return LikeState{
		Remaining:      int(p.LikeBalance),
		Capacity:       LikeCap,
		NextRecoveryAt: NextTimeRecoveryAt(recoveryStateOf(p), s.clock.Now()),
		Recovered:      recovered,
	}
}

// SyncTimeRecovery は時間回復を lazy に評価して反映し、更新後の persona と
// 画面向けの状態を返す。残数を返すエンドポイントはまずこれを通す。
//
// 3時間ごとの Cron や常駐タイマーは持たない。回復は「誰かが見に来た時点で
// 経過時間から計算する」だけで足りる。バックグラウンドで DB を触り続ける
// 仕組みは、この規模のゲームには要らない。
//
// 反映するものが無ければ、書き込みにも入らない。ホームと探索は画面を開く
// たびに呼ばれる一方、状態が動くのは3時間に1回だけなので、毎回
// トランザクションを張るのは無駄。
func (s *Service) SyncTimeRecovery(ctx context.Context, p sqlc.Persona) (sqlc.Persona, LikeState, error) {
	if !EvalTimeRecovery(recoveryStateOf(p), s.clock.Now()).Pending() {
		return p, s.likeState(p, 0), nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return p, LikeState{}, fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // コミット済みなら何もしない

	q := s.q.WithTx(tx)

	// ロックを取ってから状態を読み直す。上の判定に使った p はロックの外で
	// 読んだもので、複数のタブが同時に来ていれば古い。二重付与を防ぐのは
	// この読み直しであって、上の早期リターンではない。
	locked, err := q.LockPersona(ctx, p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, LikeState{}, apperr.PersonaNotGenerated
	}
	if err != nil {
		return p, LikeState{}, fmt.Errorf("lock persona: %w", err)
	}

	updated, recovered, err := s.applyTimeRecovery(ctx, q, locked)
	if err != nil {
		return p, LikeState{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return p, LikeState{}, fmt.Errorf("commit recovery: %w", err)
	}
	return updated, s.likeState(updated, recovered), nil
}

// applyTimeRecovery は行ロックの内側で時間回復を1回だけ評価して反映する。
//
// 呼び出し側はこの persona の行ロックを保持していること。経過した3時間が
// 無ければ渡された行をそのまま返す。
//
// 起点は経過した3時間単位ぶんすべて進める。所持上限で受け取れなかった分も
// ここで消費されるので、満タンの間に過ぎた3時間が後から回復に変わることは
// ない。戻り値の2つ目は実際に増えた数で、これがそのまま画面の通知になる。
func (s *Service) applyTimeRecovery(ctx context.Context, q *sqlc.Queries, locked sqlc.Persona) (sqlc.Persona, int, error) {
	r := EvalTimeRecovery(recoveryStateOf(locked), s.clock.Now())
	if !r.Pending() {
		return locked, 0, nil
	}

	anchor := AdvanceAnchor(*locked.LikeRecoveryAnchorAt, r.Units)
	updated, err := q.ApplyTimeRecovery(ctx, sqlc.ApplyTimeRecoveryParams{
		ID:       locked.ID,
		Amount:   int16(r.Grant),
		AnchorAt: &anchor,
	})
	if err != nil {
		return locked, 0, fmt.Errorf("apply time recovery: %w", err)
	}
	return updated, r.Grant, nil
}

// LikeResult は Like を1つ消費した後にクライアントが必要とする情報。
type LikeResult struct {
	Likes   LikeState
	Matched bool
	MatchID *uuid.UUID
	Target  sqlc.Persona
	// LikesGained は Match 報酬で実際に増えた Like の数。所持上限に達していれば
	// Match が成立しても 0 になり、画面は回復の表示を省く。
	LikesGained int
}

// Like は対象に Like を1つ消費し、相互 Like なら Match を作る。
//
// このトランザクションはペアの両 Persona を正規化した順でロックする。
// この「順序を決める」一点だけで、次の3つが同時に成立する:
//   - 複数タブから同時に Like しても予算を超えない。この実行者の Like は必ず
//     実行者自身の行ロックを先に取る必要があるため
//   - 二人が同じ瞬間に相互 Like しても、双方が相手の Like を見落として
//     Match が成立しないという事態が起きない
//   - ロックは常に id の小さい方から取るため、上記2つのケースが互いに
//     デッドロックすることがない
//
// 時間回復も同じロックの内側で評価する。よって残数0で開いたままの画面から
// でも、3時間が経っていればその場で1つ送れる。
func (s *Service) Like(ctx context.Context, actor sqlc.Persona, targetID uuid.UUID, gameDate time.Time) (LikeResult, error) {
	if actor.ID == targetID {
		return LikeResult{}, apperr.SelfActionNotAllowed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LikeResult{}, fmt.Errorf("begin like transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // コミット済みなら何もしない

	q := s.q.WithTx(tx)

	// 実行者の行はロックの内側で読み直す。呼び出し側が渡してきた actor は
	// トランザクションの外で読んだもので、回復の状態が古い可能性がある。
	locked, err := lockPair(ctx, q, actor.ID, targetID)
	if err != nil {
		return LikeResult{}, err
	}

	locked, recovered, err := s.applyTimeRecovery(ctx, q, locked)
	if err != nil {
		return LikeResult{}, err
	}

	target, err := q.GetActivePersona(ctx, sqlc.GetActivePersonaParams{
		PersonaID: targetID,
		GameDate:  gameDate,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LikeResult{}, apperr.TargetPersonaUnavailable
	}
	if err != nil {
		return LikeResult{}, fmt.Errorf("load target persona: %w", err)
	}

	// 重複判定を残数の判定より先に行う。すでに計上済みの Like をリトライした
	// とき、もう1つ消費するのではなく AlreadyLiked を返すため。
	duplicate, err := q.LikeExists(ctx, sqlc.LikeExistsParams{
		FromPersonaID: actor.ID,
		ToPersonaID:   target.ID,
	})
	if err != nil {
		return LikeResult{}, fmt.Errorf("check duplicate like: %w", err)
	}
	if duplicate {
		return LikeResult{}, apperr.AlreadyLiked
	}

	// 残高そのものが答えなので、送信済み Like を数え直す必要はない。
	// 回復ぶんもここに入っているため、報酬で得た Like を使えないまま
	// その日が終わることはない。
	if locked.LikeBalance <= 0 {
		return LikeResult{}, apperr.LikeLimitExceeded
	}

	if _, err := q.InsertLike(ctx, sqlc.InsertLikeParams{
		ID:            uuid.New(),
		FromPersonaID: actor.ID,
		ToPersonaID:   target.ID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LikeResult{}, apperr.AlreadyLiked
		}
		return LikeResult{}, fmt.Errorf("insert like: %w", err)
	}

	if err := q.IncrementExposure(ctx, target.ID); err != nil {
		return LikeResult{}, fmt.Errorf("increment exposure: %w", err)
	}

	// 残高を1つ減らす。この Like がその日の1つ目なら、ここで時間回復の
	// 起点が入ってタイマーが動き出す。
	now := s.clock.Now()
	actorAfter, err := q.ConsumeLike(ctx, sqlc.ConsumeLikeParams{ID: actor.ID, Now: &now})
	if err != nil {
		return LikeResult{}, fmt.Errorf("consume like: %w", err)
	}

	result := LikeResult{Target: target}

	mutual, err := q.LikeExists(ctx, sqlc.LikeExistsParams{
		FromPersonaID: target.ID,
		ToPersonaID:   actor.ID,
	})
	if err != nil {
		return LikeResult{}, fmt.Errorf("check mutual like: %w", err)
	}
	if mutual {
		low, high := NormalizePair(actor.ID, target.ID)

		// 報酬は「新しく成立した Match 1件」に対して払う。実行者の Like は
		// このトランザクションで新規に入ったものなので、ここに来た時点で Match は
		// 必ず新しい。それでも判定を明示しておくのは、同じ Match への二重付与を
		// 上の推論ではなくコードで否定しておきたいため。
		existed, err := q.MatchExists(ctx, sqlc.MatchExistsParams{
			PersonaLowID:  low,
			PersonaHighID: high,
		})
		if err != nil {
			return LikeResult{}, fmt.Errorf("check existing match: %w", err)
		}

		matchID, err := q.InsertMatch(ctx, sqlc.InsertMatchParams{
			ID:            uuid.New(),
			PersonaLowID:  low,
			PersonaHighID: high,
		})
		if err != nil {
			return LikeResult{}, fmt.Errorf("insert match: %w", err)
		}
		result.Matched = true
		result.MatchID = &matchID

		if !existed {
			rewarded, gained, err := grantMatchReward(ctx, q, actorAfter)
			if err != nil {
				return LikeResult{}, err
			}
			actorAfter = rewarded
			result.LikesGained = gained

			// Match は二人の出来事なので、相手にも同じ報酬を払う。先に Like して
			// 待っていた側が何も受け取れないのはおかしい。ペアの両行を
			// ロックしているので、相手の行を読んで書き足しても競合しない。
			// 相手側の時間回復には触らない。相手が次に画面を開いたときに
			// 同じ lazy 評価が走る。
			if _, _, err := grantMatchReward(ctx, q, target); err != nil {
				return LikeResult{}, err
			}
		}
	}

	result.Likes = s.likeState(actorAfter, recovered)

	if err := tx.Commit(ctx); err != nil {
		return LikeResult{}, fmt.Errorf("commit like: %w", err)
	}
	return result, nil
}

// PassResult は Pass の後にクライアントが必要とする情報。
type PassResult struct {
	PassCount int
}

// Pass は対象への Pass を記録する。回数に上限は無く、何度 Pass しても相手が
// 探すの候補から消えることはない。再表示までの間隔はフロントエンドの
// クールダウンが持つ。
func (s *Service) Pass(ctx context.Context, actor sqlc.Persona, targetID uuid.UUID, gameDate time.Time) (PassResult, error) {
	if actor.ID == targetID {
		return PassResult{}, apperr.SelfActionNotAllowed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PassResult{}, fmt.Errorf("begin pass transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // コミット済みなら何もしない

	q := s.q.WithTx(tx)

	if _, err := lockPair(ctx, q, actor.ID, targetID); err != nil {
		return PassResult{}, err
	}

	target, err := q.GetActivePersona(ctx, sqlc.GetActivePersonaParams{
		PersonaID: targetID,
		GameDate:  gameDate,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PassResult{}, apperr.TargetPersonaUnavailable
	}
	if err != nil {
		return PassResult{}, fmt.Errorf("load target persona: %w", err)
	}

	// Like 済み・Match 済みの相手は、その日はもう Pass の対象にならない。
	liked, err := q.LikeExists(ctx, sqlc.LikeExistsParams{
		FromPersonaID: actor.ID,
		ToPersonaID:   target.ID,
	})
	if err != nil {
		return PassResult{}, fmt.Errorf("check like before pass: %w", err)
	}
	if liked {
		return PassResult{}, apperr.AlreadyLiked
	}

	count, err := q.UpsertPass(ctx, sqlc.UpsertPassParams{
		ID:            uuid.New(),
		FromPersonaID: actor.ID,
		ToPersonaID:   target.ID,
	})
	if err != nil {
		return PassResult{}, fmt.Errorf("upsert pass: %w", err)
	}

	if err := q.IncrementExposure(ctx, target.ID); err != nil {
		return PassResult{}, fmt.Errorf("increment exposure: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return PassResult{}, fmt.Errorf("commit pass: %w", err)
	}

	return PassResult{PassCount: int(count)}, nil
}

// lockPair は2つの Persona 行ロックを正規化した順で取得し、実行者の行を
// ロック後の状態で返す。行が無い場合は対象がそもそも存在しないということ。
func lockPair(ctx context.Context, q *sqlc.Queries, actorID, targetID uuid.UUID) (sqlc.Persona, error) {
	low, high := NormalizePair(actorID, targetID)

	var actor sqlc.Persona
	for _, id := range [2]uuid.UUID{low, high} {
		p, err := q.LockPersona(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return sqlc.Persona{}, apperr.TargetPersonaUnavailable
			}
			return sqlc.Persona{}, fmt.Errorf("lock persona: %w", err)
		}
		if id == actorID {
			actor = p
		}
	}
	return actor, nil
}

// ProfileResult はプロフィール更新後にクライアントが必要とする情報。
type ProfileResult struct {
	Persona sqlc.Persona
	Likes   LikeState
	// LikesGained はプロフィール完成報酬で実際に増えた Like の数。
	// 報酬の条件を満たさない場合も所持上限に達している場合も 0 になる。
	LikesGained int
}

// UpdateProfile は B属性を保存し、プロフィール完成報酬をその場で判定する。
//
// 更新・完成条件の判定・付与・受け取り済みフラグの更新を、ひとつの
// トランザクションで扱う。先に persona の行ロックを取るので、同じ PATCH が
// 二重に届いても後から入った方はフラグが立った後の行を読み、報酬は一度しか出ない。
// 一度受け取った後に項目を消して入れ直しても、フラグは下がらないので同じ。
func (s *Service) UpdateProfile(ctx context.Context, personaID uuid.UUID, name, hobby, bio *string) (ProfileResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProfileResult{}, fmt.Errorf("begin profile transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // コミット済みなら何もしない

	q := s.q.WithTx(tx)

	locked, err := q.LockPersona(ctx, personaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProfileResult{}, apperr.PersonaNotGenerated
	}
	if err != nil {
		return ProfileResult{}, fmt.Errorf("lock persona: %w", err)
	}

	// 時間回復を先に反映する。この後の報酬は所持上限で切り詰めるため、
	// 回復ぶんを入れる前に判定すると上限の当たり方がずれる。
	locked, recovered, err := s.applyTimeRecovery(ctx, q, locked)
	if err != nil {
		return ProfileResult{}, err
	}

	updated, err := q.UpdatePersonaProfile(ctx, sqlc.UpdatePersonaProfileParams{
		ID:    personaID,
		Name:  name,
		Hobby: hobby,
		Bio:   bio,
	})
	if err != nil {
		return ProfileResult{}, fmt.Errorf("update profile: %w", err)
	}

	result := ProfileResult{Persona: updated}

	if !locked.ProfileRewardClaimed && ProfileComplete(updated.Name, updated.Hobby, updated.Bio) {
		gained := GrantableLikes(int(updated.LikeBalance), ProfileCompletionReward)

		rewarded, err := q.ClaimProfileReward(ctx, sqlc.ClaimProfileRewardParams{
			ID:     personaID,
			Amount: int16(gained),
		})
		if err != nil {
			return ProfileResult{}, fmt.Errorf("claim profile reward: %w", err)
		}
		result.Persona = rewarded
		result.LikesGained = gained
	}

	result.Likes = s.likeState(result.Persona, recovered)

	if err := tx.Commit(ctx); err != nil {
		return ProfileResult{}, fmt.Errorf("commit profile update: %w", err)
	}
	return result, nil
}

// grantMatchReward は Match 1件ぶんの回復を1人に払い、更新後の行と実際に
// 増えた数を返す。
//
// 呼び出し側はこの Persona の行ロックを保持していること。回数の上限に
// 達していれば何もしない。上限に達していない限り、回復量が 0 でも回数は
// 消費する。所持上限で溢れた分は仕様どおり失われる。
func grantMatchReward(ctx context.Context, q *sqlc.Queries, p sqlc.Persona) (sqlc.Persona, int, error) {
	if p.MatchRewardCount >= MaxMatchRewards {
		return p, 0, nil
	}

	gained := GrantableLikes(int(p.LikeBalance), MatchReward)

	rewarded, err := q.ClaimMatchReward(ctx, sqlc.ClaimMatchRewardParams{
		ID:     p.ID,
		Amount: int16(gained),
	})
	if err != nil {
		return p, 0, fmt.Errorf("claim match reward: %w", err)
	}
	return rewarded, gained, nil
}
