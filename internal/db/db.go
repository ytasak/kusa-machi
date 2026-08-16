// Package db は PostgreSQL の接続プールとスキーマのマイグレーションを担う。
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // golang-migrate が使う database/sql ドライバ

	"kusamachi/migrations"
)

// Connect は pgx のプールを開き、DB に到達できることを確認する。
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate はバイナリに埋め込まれた未適用のマイグレーションをすべて適用する。
func Migrate(databaseURL string) error {
	m, closeFn, err := migrator(databaseURL)
	if err != nil {
		return err
	}
	defer closeFn()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDown はすべてのマイグレーションを巻き戻す。ローカル開発専用。
func MigrateDown(databaseURL string) error {
	m, closeFn, err := migrator(databaseURL)
	if err != nil {
		return err
	}
	defer closeFn()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func migrator(databaseURL string) (*migrate.Migrate, func(), error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("migration source: %w", err)
	}

	sqlDB, err := sql.Open("pgx/v5", databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open migration connection: %w", err)
	}

	driver, err := migratepg.WithInstance(sqlDB, &migratepg.Config{})
	if err != nil {
		sqlDB.Close()
		return nil, nil, fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		sqlDB.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	return m, func() { sqlDB.Close() }, nil
}
