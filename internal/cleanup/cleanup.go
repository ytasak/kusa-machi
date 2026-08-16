// Package cleanup は期限切れのゲームデータを物理削除する。
//
// 正しさがこのジョブに依存することはない。すべてのクエリが game_date で
// スコープされているため、古い行はすでに見えない。このジョブは容量を
// 回収するだけなので、遅れて動いても、二重に動いても、動かなくても安全。
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

// Job は過去のゲーム日の participant を削除する。ON DELETE CASCADE により
// 紐づく persona / like / pass / match も一緒に消える。
type Job struct {
	q      *sqlc.Queries
	clock  clock.Clock
	photos *photo.Store
}

// NewJob は削除ジョブを組み立てる。
func NewJob(pool *pgxpool.Pool, clk clock.Clock, photos *photo.Store) *Job {
	return &Job{q: sqlc.New(pool), clock: clk, photos: photos}
}

// RunOnce は現在の JST ゲーム日より古いものをすべて削除し、消した participant の
// 件数を返す。冪等でリトライしても安全。
//
// 写真ファイルは ON DELETE CASCADE の対象外なので、あわせて掃除する。
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

// Run はジョブを即座に実行し、その後 ctx が終わるまで一定間隔で実行し続ける。
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
