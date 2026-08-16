// Package middleware resolves the anonymous session and enforces CSRF.
package middleware

import (
	"context"
	"net/http"
	"time"

	"kusamachi/internal/clock"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/response"
	"kusamachi/internal/participant"

	"github.com/google/uuid"
)

type contextKey struct{}

var sessionKey contextKey

// Session is today's resolved identity for the current request.
type Session struct {
	Participant sqlc.Participant
	GameDate    time.Time
	Now         time.Time
}

// WithSession reads or issues the anonymous cookie and guarantees that today's
// participant row exists before the handler runs.
func WithSession(q *sqlc.Queries, clk clock.Clock, cfg participant.CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := participant.ReadToken(r)
			if !ok {
				token = participant.NewToken()
			}
			// Rewrite on every request so the ~30 day expiry slides forward.
			participant.SetToken(w, cfg, token)

			now := clk.Now()
			gameDate := clock.GameDate(now)

			p, err := participant.Ensure(r.Context(), q, token, gameDate)
			if err != nil {
				response.Error(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, Session{
				Participant: p,
				GameDate:    gameDate,
				Now:         now,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionFrom returns the session WithSession stored. It panics when the route
// was mounted without the middleware, which is a wiring bug, not a user error.
func SessionFrom(ctx context.Context) Session {
	s, ok := ctx.Value(sessionKey).(Session)
	if !ok {
		panic("session middleware is not mounted on this route")
	}
	return s
}

// CookieTokenFrom is a convenience accessor used by handlers that need the
// browser identity itself rather than the participant.
func CookieTokenFrom(ctx context.Context) uuid.UUID {
	return SessionFrom(ctx).Participant.CookieToken
}
