// Package config loads server configuration from the environment.
package config

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every knob the server needs. Defaults are production-shaped;
// local development overrides them through the environment (see README).
type Config struct {
	Addr        string
	DatabaseURL string
	WebDistDir  string
	PhotoDir    string

	// CookieSecure / CookieSameSite exist only so the app can be exercised over
	// plain http on localhost. The spec's required production values are the
	// defaults: Secure, SameSite=None (needed for the kusa iframe).
	CookieSecure   bool
	CookieSameSite http.SameSite

	CleanupInterval time.Duration
}

// Load reads the environment, applying defaults.
func Load() (Config, error) {
	cfg := Config{
		Addr:            env("ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://kusa:kusa@localhost:5433/kusamachi?sslmode=disable"),
		WebDistDir:      env("WEB_DIST_DIR", "web/dist"),
		PhotoDir:        env("PHOTO_DIR", "data/photos"),
		CookieSecure:    true,
		CookieSameSite:  http.SameSiteNoneMode,
		CleanupInterval: time.Hour,
	}

	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
		}
		cfg.CookieSecure = b
	}

	if v := os.Getenv("COOKIE_SAMESITE"); v != "" {
		switch strings.ToLower(v) {
		case "none":
			cfg.CookieSameSite = http.SameSiteNoneMode
		case "lax":
			cfg.CookieSameSite = http.SameSiteLaxMode
		case "strict":
			cfg.CookieSameSite = http.SameSiteStrictMode
		default:
			return Config{}, fmt.Errorf("COOKIE_SAMESITE: unknown value %q", v)
		}
	}

	if v := os.Getenv("CLEANUP_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("CLEANUP_INTERVAL: %w", err)
		}
		cfg.CleanupInterval = d
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
