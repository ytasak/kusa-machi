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
	"kusamachi/internal/db/sqlc"
)

// Service は Like・Pass・プロフィール更新のトランザクションを実行する。
// いずれも、市場のルールを強制するのがアプリケーションではなくデータベースに
// なるように書かれている。
type Service struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewService は接続プールの上にサービスを構築する。
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: sqlc.New(pool)}
}

// LikeResult は Like を1つ消費した後にクライアントが必要とする情報。
type LikeResult struct {
	RemainingLikes int
	Matched        bool
	MatchID        *uuid.UUID
	Target         sqlc.Persona
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

	// 重複判定を予算判定より先に行う。すでに計上済みの Like をリトライしたとき、
	// もう1つ消費するのではなく AlreadyLiked を返すため。
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

	sent, err := q.CountLikesSent(ctx, actor.ID)
	if err != nil {
		return LikeResult{}, fmt.Errorf("count sent likes: %w", err)
	}
	// 予算そのものではなく残数で判定する。回復ぶんを足した残数で見ないと、
	// 報酬で得た Like を使えないままその日が終わってしまう。
	if RemainingLikes(sent, locked.BonusLikes) <= 0 {
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

	// この Like を消費した後の残数。Match 報酬が出ればこの後で上積みする。
	actorSent := sent + 1
	result := LikeResult{
		RemainingLikes: RemainingLikes(actorSent, locked.BonusLikes),
		Target:         target,
	}

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
			gained, err := grantMatchReward(ctx, q, locked, actorSent)
			if err != nil {
				return LikeResult{}, err
			}
			result.LikesGained = gained
			result.RemainingLikes = RemainingLikes(actorSent, locked.BonusLikes+int16(gained))

			// Match は二人の出来事なので、相手にも同じ報酬を払う。先に Like して
			// 待っていた側が何も受け取れないのはおかしい。ペアの両行を
			// ロックしているので、相手の残数を読んで書き足しても競合しない。
			if err := grantCounterpartMatchReward(ctx, q, target); err != nil {
				return LikeResult{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return LikeResult{}, fmt.Errorf("commit like: %w", err)
	}
	return result, nil
}

// PassResult は Pass の後にクライアントが必要とする情報。
type PassResult struct {
	PassCount        int
	ExcludedForToday bool
}

// Pass は対象への Pass を記録する。1日あたり3回で打ち止め。
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
	if errors.Is(err, pgx.ErrNoRows) {
		// 条件付き upsert が3回を超える更新を拒否した。
		return PassResult{}, apperr.PassLimitReached
	}
	if err != nil {
		return PassResult{}, fmt.Errorf("upsert pass: %w", err)
	}

	if err := q.IncrementExposure(ctx, target.ID); err != nil {
		return PassResult{}, fmt.Errorf("increment exposure: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return PassResult{}, fmt.Errorf("commit pass: %w", err)
	}

	return PassResult{
		PassCount:        int(count),
		ExcludedForToday: int(count) >= MaxPassCount,
	}, nil
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
	Persona        sqlc.Persona
	RemainingLikes int
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

	updated, err := q.UpdatePersonaProfile(ctx, sqlc.UpdatePersonaProfileParams{
		ID:    personaID,
		Name:  name,
		Hobby: hobby,
		Bio:   bio,
	})
	if err != nil {
		return ProfileResult{}, fmt.Errorf("update profile: %w", err)
	}

	sent, err := q.CountLikesSent(ctx, personaID)
	if err != nil {
		return ProfileResult{}, fmt.Errorf("count sent likes: %w", err)
	}

	result := ProfileResult{Persona: updated}

	if !locked.ProfileRewardClaimed && ProfileComplete(updated.Name, updated.Hobby, updated.Bio) {
		current := RemainingLikes(sent, updated.BonusLikes)
		gained := GrantableLikes(current, ProfileCompletionReward)

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

	result.RemainingLikes = RemainingLikes(sent, result.Persona.BonusLikes)

	if err := tx.Commit(ctx); err != nil {
		return ProfileResult{}, fmt.Errorf("commit profile update: %w", err)
	}
	return result, nil
}

// grantMatchReward は Match 1件ぶんの回復を1人に払い、実際に増えた数を返す。
//
// 呼び出し側はこの Persona の行ロックを保持していること。回数の上限に
// 達していれば何もしない。上限に達していない限り、回復量が 0 でも回数は
// 消費する。所持上限で溢れた分は仕様どおり失われる。
func grantMatchReward(ctx context.Context, q *sqlc.Queries, p sqlc.Persona, sent int64) (int, error) {
	if p.MatchRewardCount >= MaxMatchRewards {
		return 0, nil
	}

	gained := GrantableLikes(RemainingLikes(sent, p.BonusLikes), MatchReward)

	if _, err := q.ClaimMatchReward(ctx, sqlc.ClaimMatchRewardParams{
		ID:     p.ID,
		Amount: int16(gained),
	}); err != nil {
		return 0, fmt.Errorf("claim match reward: %w", err)
	}
	return gained, nil
}

// grantCounterpartMatchReward は Match の相手側に報酬を払う。相手は今この
// リクエストの中にいないので、残数を数え直してから grantMatchReward に渡す。
// 増えた数は返さない。相手の画面はこの後ホームを読んだときに新しい残数を得る。
func grantCounterpartMatchReward(ctx context.Context, q *sqlc.Queries, counterpart sqlc.Persona) error {
	sent, err := q.CountLikesSent(ctx, counterpart.ID)
	if err != nil {
		return fmt.Errorf("count counterpart sent likes: %w", err)
	}
	if _, err := grantMatchReward(ctx, q, counterpart, sent); err != nil {
		return err
	}
	return nil
}
