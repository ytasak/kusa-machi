// Package cleanup physically deletes expired game data.
//
// Correctness never depends on this job: every query is scoped by game_date, so
// stale rows are already invisible. The job only reclaims storage, which makes
// it safe to run late, twice, or not at all.
package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kusamachi/internal/clock"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/photo"
)

// Job deletes participants from previous game days. ON DELETE CASCADE removes
// their personas, likes, passes and matches.
type Job struct {
	q      *sqlc.Queries
	clock  clock.Clock
	photos *photo.Store
}

// NewJob builds the deletion job.
func NewJob(pool *pgxpool.Pool, clk clock.Clock, photos *photo.Store) *Job {
	return &Job{q: sqlc.New(pool), clock: clk, photos: photos}
}

// RunOnce deletes everything older than the current JST game day and reports
// how many participants were removed. Idempotent and safe to retry.
//
// Photo files are not covered by ON DELETE CASCADE, so they are swept too.
func (j *Job) RunOnce(ctx context.Context) (int64, error) {
	today := clock.Today(j.clock)

	deleted, err := j.q.DeleteParticipantsBefore(ctx, today)
	if err != nil {
		return 0, err
	}

	if j.photos != nil {
		if _, err := j.photos.DeleteBefore(today); err != nil {
			return deleted, err
		}
	}
	return deleted, nil
}

// Run executes the job immediately and then on every tick until ctx is done.
func (j *Job) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		deleted, err := j.RunOnce(ctx)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			slog.Error("cleanup failed", "error", err)
		case deleted > 0:
			slog.Info("cleanup removed expired participants", "count", deleted)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
