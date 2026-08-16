// Package participant owns the anonymous browser identity: the cookie token
// and the one participant row that exists per cookie per game day.
package participant

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"kusamachi/internal/db/sqlc"
)

// CookieName is the anonymous browser identifier. It carries no game state.
const CookieName = "kusa_machi_token"

// cookieMaxAge is the ~30 day expiry required by the spec.
const cookieMaxAge = 30 * 24 * 60 * 60

// csrfTokenBytes is the entropy of the daily CSRF token.
const csrfTokenBytes = 32

// CookieConfig carries the attributes the cookie is written with.
// Secure and SameSite are configurable only so the app can be exercised over
// plain http locally; production uses Secure + SameSite=None for the iframe.
type CookieConfig struct {
	Secure   bool
	SameSite http.SameSite
}

// ReadToken returns the cookie's token when it is present and well-formed.
func ReadToken(r *http.Request) (uuid.UUID, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return uuid.Nil, false
	}
	token, err := uuid.Parse(c.Value)
	if err != nil {
		return uuid.Nil, false
	}
	return token, true
}

// SetToken writes the cookie, refreshing its expiry on every request.
func SetToken(w http.ResponseWriter, cfg CookieConfig, token uuid.UUID) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token.String(),
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// NewToken mints a fresh anonymous browser identity.
func NewToken() uuid.UUID { return uuid.New() }

// Ensure returns today's participant for the cookie token, creating it on
// first access. The unique (cookie_token, game_date) index makes this
// race-safe: concurrent first requests converge on the same row.
func Ensure(ctx context.Context, q *sqlc.Queries, token uuid.UUID, gameDate time.Time) (sqlc.Participant, error) {
	csrfToken, err := NewCSRFToken()
	if err != nil {
		return sqlc.Participant{}, err
	}

	p, err := q.UpsertParticipant(ctx, sqlc.UpsertParticipantParams{
		ID:          uuid.New(),
		CookieToken: token,
		GameDate:    gameDate,
		CsrfToken:   csrfToken,
	})
	if err != nil {
		return sqlc.Participant{}, fmt.Errorf("ensure participant: %w", err)
	}
	return p, nil
}

// NewCSRFToken generates an opaque Base64URL token from 32 random bytes.
func NewCSRFToken() (string, error) {
	buf := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
