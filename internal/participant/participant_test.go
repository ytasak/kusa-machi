package participant

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSetTokenUsesTheIframeSafeCookieAttributes(t *testing.T) {
	rec := httptest.NewRecorder()
	token := uuid.New()

	// 本番の設定。kusa の iframe に埋め込むため SameSite=None が必要で、
	// それに伴い Secure も必要になる。
	SetToken(rec, CookieConfig{Secure: true, SameSite: http.SameSiteNoneMode}, token)

	header := rec.Header().Get("Set-Cookie")
	if header == "" {
		t.Fatal("no Set-Cookie header")
	}

	for _, want := range []string{
		CookieName + "=" + token.String(),
		"Path=/",
		"HttpOnly",
		"Secure",
		"SameSite=None",
		"Max-Age=2592000", // 30日
		// サードパーティ Cookie を遮断するブラウザでも埋め込み元ごとに
		// 分離して保存させるため。これが無いと iOS Safari で詰む。
		"Partitioned",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("Set-Cookie %q is missing %q", header, want)
		}
	}

	if strings.Contains(header, "Domain=") {
		t.Errorf("Set-Cookie %q must not pin an explicit domain", header)
	}
}

// ローカルの平文 http では Partitioned は付けられない（ブラウザが Secure を
// 要求する）。開発用の設定でうっかり付かないことを確かめる。
func TestSetTokenDoesNotPartitionInTheLocalConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	SetToken(rec, CookieConfig{Secure: false, SameSite: http.SameSiteLaxMode}, uuid.New())

	header := rec.Header().Get("Set-Cookie")
	if strings.Contains(header, "Partitioned") {
		t.Errorf("Set-Cookie %q must not be Partitioned without Secure", header)
	}
}

func TestReadToken(t *testing.T) {
	token := uuid.New()

	tests := []struct {
		name   string
		cookie *http.Cookie
		wantOK bool
	}{
		{"Cookieなし", nil, false},
		{"正しいトークン", &http.Cookie{Name: CookieName, Value: token.String()}, true},
		{"形式が不正なトークン", &http.Cookie{Name: CookieName, Value: "not-a-uuid"}, false},
		{"別名のCookie", &http.Cookie{Name: "unrelated", Value: token.String()}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/home", nil)
			if tc.cookie != nil {
				r.AddCookie(tc.cookie)
			}

			got, ok := ReadToken(r)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != token {
				t.Fatalf("token = %s, want %s", got, token)
			}
		})
	}
}

func TestCSRFTokensAreOpaqueAndUnique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for range 100 {
		token, err := NewCSRFToken()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		// 32バイトの乱数をパディングなし base64url にしたもの。
		if len(token) != 43 {
			t.Fatalf("token %q has length %d, want 43", token, len(token))
		}
		if seen[token] {
			t.Fatalf("duplicate csrf token %q", token)
		}
		seen[token] = true
	}
}
