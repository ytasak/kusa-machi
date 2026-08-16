// Package middleware は匿名セッションの解決と CSRF の強制を担う。
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

// Session は現在のリクエストについて解決された当日の identity。
type Session struct {
	Participant sqlc.Participant
	GameDate    time.Time
	Now         time.Time
}

// WithSession は匿名 Cookie を読むか発行し、ハンドラが動く前に当日の
// participant 行が存在することを保証する。
func WithSession(q *sqlc.Queries, clk clock.Clock, cfg participant.CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := participant.ReadToken(r)
			if !ok {
				token = participant.NewToken()
			}
			// 毎リクエストで書き直し、約30日の有効期限をスライドさせる。
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

// SessionFrom は WithSession が格納したセッションを返す。ミドルウェアを付けずに
// ルートを登録した場合は panic する。これは配線のバグであってユーザーの誤りではない。
func SessionFrom(ctx context.Context) Session {
	s, ok := ctx.Value(sessionKey).(Session)
	if !ok {
		panic("session middleware is not mounted on this route")
	}
	return s
}

// CookieTokenFrom は participant ではなくブラウザ identity 自体が必要な
// ハンドラ向けの補助アクセサ。
func CookieTokenFrom(ctx context.Context) uuid.UUID {
	return SessionFrom(ctx).Participant.CookieToken
}
