// Package httpx wires the chi router: /api/* endpoints plus the Svelte build
// served from the same Origin, so the app works in an iframe and standalone.
package httpx

import (
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"kusamachi/internal/apperr"
	"kusamachi/internal/http/response"
)

// Deps are the collaborators the router needs.
type Deps struct {
	WebDistDir string
}

// NewRouter builds the whole HTTP surface.
func NewRouter(deps Deps) http.Handler {
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
	})

	r.NotFound(staticHandler(deps.WebDistDir))
	return r
}

// staticHandler serves the Vite build with SPA fallback to index.html.
func staticHandler(dir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// path.Clean on a rooted path neutralises "..", so the join stays inside dir.
		clean := path.Clean("/" + r.URL.Path)
		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); err == nil && !st.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		if _, err := os.Stat(index); err != nil {
			http.Error(w, "frontend build not found: run `npm --prefix web run build`", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, index)
	}
}
