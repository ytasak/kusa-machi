package config

import (
	"net/http"
	"testing"
)

func TestLoadDefaultsAreProductionShaped(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// iframe 埋め込みに必要な既定値。デプロイ時に設定し忘れても本番として
	// 正しい側に倒れるようにしてある。
	if !cfg.CookieSecure {
		t.Error("COOKIE_SECURE の既定値は true であること")
	}
	if cfg.CookieSameSite != http.SameSiteNoneMode {
		t.Error("COOKIE_SAMESITE の既定値は None であること")
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
}

func TestPortOverridesAddr(t *testing.T) {
	// PaaS はリッスンポートを PORT で注入する。ここが効いていないと
	// プラットフォームのヘルスチェックが永久に届かない。
	t.Setenv("ADDR", ":9999")
	t.Setenv("PORT", "10000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Addr != ":10000" {
		t.Fatalf("Addr = %q, want :10000（PORT が ADDR より優先されること）", cfg.Addr)
	}
}

func TestInvalidPortIsRejected(t *testing.T) {
	for _, v := range []string{"0", "-1", "70000", "http", "8080abc"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("PORT", v)
			if _, err := Load(); err == nil {
				t.Fatalf("PORT=%q が受理された", v)
			}
		})
	}
}

func TestCookieOverridesForLocalDevelopment(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("COOKIE_SAMESITE", "lax")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CookieSecure {
		t.Error("COOKIE_SECURE=false が反映されていない")
	}
	if cfg.CookieSameSite != http.SameSiteLaxMode {
		t.Error("COOKIE_SAMESITE=lax が反映されていない")
	}
}

func TestUnknownSameSiteIsRejected(t *testing.T) {
	t.Setenv("COOKIE_SAMESITE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("未知の COOKIE_SAMESITE が受理された")
	}
}
