package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// distDir builds a fake Vite output directory.
func distDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>app</title>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o700); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("export default 1;"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	return dir
}

func TestStaticHandlerServesAssetsAndFallsBackToIndex(t *testing.T) {
	h := staticHandler(distDir(t))

	tests := []struct {
		name     string
		path     string
		wantBody string
	}{
		{"root serves the app", "/", "<!doctype html>"},
		{"a built asset is served as itself", "/assets/app.js", "export default 1;"},
		// Opening the app directly on a deep link must still boot the SPA.
		{"deep link falls back to index", "/matches", "<!doctype html>"},
		{"unknown nested path falls back to index", "/a/b/c", "<!doctype html>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); !strings.Contains(got, tc.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", got, tc.wantBody)
			}
		})
	}
}

func TestStaticHandlerDoesNotBlockIframeEmbedding(t *testing.T) {
	// The app must be embeddable in the kusa iframe, so nothing may add
	// X-Frame-Options or a frame-ancestors policy.
	rec := httptest.NewRecorder()
	staticHandler(distDir(t))(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want none", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("Content-Security-Policy = %q, want none", got)
	}
}

func TestStaticHandlerRejectsNonReadMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	staticHandler(distDir(t))(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
