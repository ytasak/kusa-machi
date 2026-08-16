// Command server runs the anonymous matching MVP: API + static frontend.
//
// Usage:
//
//	server              start the HTTP server (migrations run on boot)
//	server migrate up   apply migrations and exit
//	server migrate down roll migrations back and exit (local development only)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kusamachi/internal/cleanup"
	"kusamachi/internal/clock"
	"kusamachi/internal/config"
	"kusamachi/internal/db"
	httpx "kusamachi/internal/http"
	"kusamachi/internal/participant"
	"kusamachi/internal/persona"
	"kusamachi/internal/photo"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		return runCommand(cfg, args)
	}
	return serve(cfg)
}

func runCommand(cfg config.Config, args []string) error {
	switch {
	case args[0] == "migrate" && len(args) > 1 && args[1] == "up":
		return db.Migrate(cfg.DatabaseURL)
	case args[0] == "migrate" && len(args) > 1 && args[1] == "down":
		return db.MigrateDown(cfg.DatabaseURL)
	default:
		return fmt.Errorf("unknown command: %v", args)
	}
}

func serve(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	gameClock := clock.Real{}

	photos, err := photo.NewStore(cfg.PhotoDir)
	if err != nil {
		return err
	}

	// Physical deletion of previous days; correctness does not depend on it.
	go cleanup.NewJob(pool, gameClock, photos).Run(ctx, cfg.CleanupInterval)

	router := httpx.NewRouter(httpx.Deps{
		Pool:      pool,
		Clock:     gameClock,
		Generator: persona.NewGenerator(),
		Photos:    photos,
		Cookie: participant.CookieConfig{
			Secure:   cfg.CookieSecure,
			SameSite: cfg.CookieSameSite,
		},
		WebDistDir: cfg.WebDistDir,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "web_dist", cfg.WebDistDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
