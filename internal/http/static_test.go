package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// distDir は Vite の出力ディレクトリを模したものを作る。
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
		{"ルートでアプリが配信される", "/", "<!doctype html>"},
		{"ビルド済みアセットはそのまま配信される", "/assets/app.js", "export default 1;"},
		// 深いパスを直接開いても SPA が起動する必要がある。
		{"深いパスは index にフォールバックする", "/matches", "<!doctype html>"},
		{"未知のネストしたパスも index にフォールバックする", "/a/b/c", "<!doctype html>"},
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
	// kusa の iframe に埋め込める必要があるため、X-Frame-Options や
	// frame-ancestors ポリシーを付けてはならない。
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

func TestIndexIsRevalidatedSoDeploysReachReturningVisitors(t *testing.T) {
	// index.html はハッシュ付きアセットの入口。ここが古いまま使い回されると、
	// デプロイ済みの新しいバンドルに永久に切り替わらない。
	h := staticHandler(distDir(t))

	for _, path := range []string{"/", "/matches"} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, path, nil))

		got := rec.Header().Get("Cache-Control")
		if !strings.Contains(got, "no-cache") {
			t.Errorf("%s: Cache-Control = %q, want it to force revalidation", path, got)
		}
	}
}

func TestHashedAssetsAreCachedHard(t *testing.T) {
	// ファイル名にハッシュが入っているので、中身が変わればパスも変わる。
	// 長期キャッシュしても古いものを掴み続けることはない。
	rec := httptest.NewRecorder()
	staticHandler(distDir(t))(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	got := rec.Header().Get("Cache-Control")
	if !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable for a hashed asset", got)
	}
}

func TestMissingAssetIs404RatherThanIndexHTML(t *testing.T) {
	// 古い index.html を握ったままのブラウザは、消えたバンドルを取りに来る。
	// ここで index.html を 200 で返すと、HTML が JS として実行されて
	// 画面が黙って壊れる。404 なら読み込み失敗として扱われる。
	rec := httptest.NewRecorder()
	staticHandler(distDir(t))(rec, httptest.NewRequest(http.MethodGet, "/assets/index-OLDHASH.js", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}
