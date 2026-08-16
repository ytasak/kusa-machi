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

// Service performs the like and pass transactions. Both are written so that the
// database, not the application, is the thing enforcing the market rules.
type Service struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewService builds the service on a connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: sqlc.New(pool)}
}

// LikeResult is what the client needs after spending a like.
type LikeResult struct {
	RemainingLikes int
	Matched        bool
	MatchID        *uuid.UUID
	Target         sqlc.Persona
}

// Like spends one like on the target and creates a match if the like is mutual.
//
// The transaction locks both personas of the pair in normalised order. That
// single ordering rule gives three properties at once:
//   - the actor's like budget cannot be exceeded by parallel tabs, because every
//     like by this actor must first take the actor's row lock;
//   - two people liking each other at the same moment cannot both miss the
//     other's like and end up unmatched;
//   - locks are always taken low id first, so the two cases above can never
//     deadlock against each other.
func (s *Service) Like(ctx context.Context, actor sqlc.Persona, targetID uuid.UUID, gameDate time.Time) (LikeResult, error) {
	if actor.ID == targetID {
		return LikeResult{}, apperr.SelfActionNotAllowed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LikeResult{}, fmt.Errorf("begin like transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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

	// Duplicate is checked before the budget so a retry of an already-counted
	// like reports AlreadyLiked instead of eating another like.
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

// PassResult is what the client needs after a pass.
type PassResult struct {
	PassCount        int
	ExcludedForToday bool
}

// Pass records a pass against the target, capping at three per day.
func (s *Service) Pass(ctx context.Context, actor sqlc.Persona, targetID uuid.UUID, gameDate time.Time) (PassResult, error) {
	if actor.ID == targetID {
		return PassResult{}, apperr.SelfActionNotAllowed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PassResult{}, fmt.Errorf("begin pass transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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

	// A liked or matched persona is out of the pass flow for the rest of the day.
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
		// The conditional upsert refused to go past three.
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

// lockPair takes the two persona row locks in normalised order. A missing row
// means the target does not exist at all.
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
