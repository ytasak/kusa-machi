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

	// Production configuration: the app is embedded in the kusa iframe, which
	// requires SameSite=None and therefore Secure.
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
		"Max-Age=2592000", // 30 days
	} {
		if !strings.Contains(header, want) {
			t.Errorf("Set-Cookie %q is missing %q", header, want)
		}
	}

	if strings.Contains(header, "Domain=") {
		t.Errorf("Set-Cookie %q must not pin an explicit domain", header)
	}
}

func TestReadToken(t *testing.T) {
	token := uuid.New()

	tests := []struct {
		name   string
		cookie *http.Cookie
		wantOK bool
	}{
		{"no cookie", nil, false},
		{"valid token", &http.Cookie{Name: CookieName, Value: token.String()}, true},
		{"malformed token", &http.Cookie{Name: CookieName, Value: "not-a-uuid"}, false},
		{"other cookie", &http.Cookie{Name: "unrelated", Value: token.String()}, false},
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
		// 32 random bytes as unpadded base64url.
		if len(token) != 43 {
			t.Fatalf("token %q has length %d, want 43", token, len(token))
		}
		if seen[token] {
			t.Fatalf("duplicate csrf token %q", token)
		}
		seen[token] = true
	}
}
