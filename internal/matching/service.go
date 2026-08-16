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

// Service は Like と Pass のトランザクションを実行する。どちらも、市場のルールを
// 強制するのがアプリケーションではなくデータベースになるように書かれている。
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

	if err := lockPair(ctx, q, actor.ID, targetID); err != nil {
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
	if sent >= DailyLikeBudget {
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

	result := LikeResult{
		RemainingLikes: RemainingLikes(sent + 1),
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

	if err := lockPair(ctx, q, actor.ID, targetID); err != nil {
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

// lockPair は2つの Persona 行ロックを正規化した順で取得する。行が無い場合は
// 対象がそもそも存在しないということ。
func lockPair(ctx context.Context, q *sqlc.Queries, a, b uuid.UUID) error {
	low, high := NormalizePair(a, b)
	for _, id := range [2]uuid.UUID{low, high} {
		if _, err := q.LockPersona(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.TargetPersonaUnavailable
			}
			return fmt.Errorf("lock persona: %w", err)
		}
	}
	return nil
}
