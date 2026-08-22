// Package httpx は chi のルータを組み立てる。/api/* のエンドポイントに加えて
// Svelte のビルドを同一 Origin で配信するため、iframe 埋め込みでも
// 直接 URL を開いた場合でも動作する。
package httpx

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/handler"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
	"kusamachi/internal/participant"
	"kusamachi/internal/persona"
	"kusamachi/internal/photo"
)

// Deps はルータが必要とする依存。
type Deps struct {
	Pool       *pgxpool.Pool
	Clock      clock.Clock
	Generator  *persona.Generator
	Photos     *photo.Store
	Cookie     participant.CookieConfig
	WebDistDir string
}

// NewRouter は HTTP の全面を組み立てる。
func NewRouter(deps Deps) http.Handler {
	q := sqlc.New(deps.Pool)
	h := handler.New(deps.Pool, deps.Clock, deps.Generator, deps.Photos)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			response.Fail(w, apperr.CodeInvalidRequest, "no such endpoint")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			response.Fail(w, apperr.CodeInvalidRequest, "method not allowed")
		})

		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})

		// 以下はすべて、ブラウザを識別し、当日の participant の存在を保証し、
		// 更新系には CSRF トークンを要求する。
		r.Group(func(r chi.Router) {
			r.Use(middleware.WithSession(q, deps.Clock, deps.Cookie))
			r.Use(middleware.CSRF(q))

			r.Get("/home", h.Home)
			r.Post("/persona", h.GeneratePersona)
			r.Get("/persona/me", h.MyPersona)
			r.Patch("/persona/profile", h.UpdateProfile)
			r.Post("/persona/photo", h.UploadPhoto)
			r.Delete("/persona/photo", h.DeletePhoto)
			r.Get("/personas/{personaID}/photo", h.GetPhoto)
			r.Get("/discover", h.Discover)
			r.Post("/likes", h.CreateLike)
			r.Post("/passes", h.CreatePass)
			r.Get("/likes/received", h.ReceivedLikes)
			r.Get("/likes/sent", h.SentLikes)
			r.Get("/matches", h.Matches)
			r.Get("/matches/{matchID}", h.MatchDetail)
			r.Post("/matches/{matchID}/child", h.CreateMatchChild)
		})
	})

	r.NotFound(staticHandler(deps.WebDistDir))
	return r
}

// assetsPrefix は Vite がハッシュ付きのファイルを置く場所。
const assetsPrefix = "/assets/"

// staticHandler は Vite のビルドを配信し、SPA として index.html にフォールバックする。
//
// キャッシュの扱いは2つに分ける。
//
// /assets/ 配下はファイル名にハッシュが入っているので、中身が変われば URL も
// 変わる。よって恒久的にキャッシュさせてよい。
//
// index.html はそのハッシュを指す入口なので、必ず再検証させる。ここを黙って
// 使い回されると、配信済みの新しいバンドルに永久に切り替わらない。ブラウザは
// Cache-Control が無いと Last-Modified から勝手に鮮度を推測して使い回すため、
// 「何も指定しない」は「毎回取り直す」ではなく「いつまで古いままか分からない」を
// 意味する。
func staticHandler(dir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 先頭スラッシュ付きパスへの path.Clean は ".." を無効化するため、結合結果は dir の外に出ない。
		clean := path.Clean("/" + r.URL.Path)
		isAsset := strings.HasPrefix(clean, assetsPrefix)

		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); err == nil && !st.IsDir() {
			if isAsset {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				// index.html を直接叩かれた場合もここに来る。入口は再検証させる。
				w.Header().Set("Cache-Control", "no-cache")
			}
			files.ServeHTTP(w, r)
			return
		}

		// 古い index.html を握ったままのブラウザは、削除済みのバンドルを取りに来る。
		// そこへ index.html を 200 で返すと、HTML が JS として実行されて画面が
		// 黙って壊れる。素直に 404 を返して読み込み失敗として扱わせる。
		if isAsset {
			http.NotFound(w, r)
			return
		}

		if _, err := os.Stat(index); err != nil {
			http.Error(w, "frontend build not found: run `npm --prefix web run build`", http.StatusNotFound)
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}
}
