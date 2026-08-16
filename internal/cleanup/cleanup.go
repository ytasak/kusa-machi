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
)

// Job deletes participants from previous game days. ON DELETE CASCADE removes
// their personas, likes, passes and matches.
type Job struct {
	q     *sqlc.Queries
	clock clock.Clock
}

// NewJob builds the deletion job.
func NewJob(pool *pgxpool.Pool, clk clock.Clock) *Job {
	return &Job{q: sqlc.New(pool), clock: clk}
}

// RunOnce deletes everything older than the current JST game day and reports
// how many participants were removed. Idempotent and safe to retry.
func (j *Job) RunOnce(ctx context.Context) (int64, error) {
	return j.q.DeleteParticipantsBefore(ctx, clock.Today(j.clock))
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
