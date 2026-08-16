// Package config はサーバ設定を環境変数から読み込む。
package config

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config はサーバが必要とする設定をすべて持つ。既定値は本番向けで、
// ローカル開発では環境変数で上書きする（README 参照）。
type Config struct {
	// Addr は ADDR、または PaaS が注入する PORT から決まる。
	Addr        string
	DatabaseURL string
	WebDistDir  string
	PhotoDir    string

	// CookieSecure / CookieSameSite は localhost の平文 http で動作確認するために
	// だけ存在する。仕様が本番に要求する値が既定値になっている:
	// Secure かつ SameSite=None（kusa の iframe 埋め込みに必要）。
	CookieSecure   bool
	CookieSameSite http.SameSite

	CleanupInterval time.Duration
}

// Load は環境変数を読み、既定値を適用する。
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

	// PaaS はリッスンポートを PORT で注入するため、ADDR より優先する。
	// これが無いとプラットフォームのヘルスチェックが届かない。
	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("PORT: %q は有効なポート番号ではありません", v)
		}
		cfg.Addr = ":" + v
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
