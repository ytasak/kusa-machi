package middleware

import (
	"crypto/subtle"
	"net/http"

	"kusamachi/internal/apperr"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/response"
)

// CSRFHeader carries the daily token on mutating requests.
const CSRFHeader = "X-CSRF-Token"

// CSRF rejects any mutating request that does not carry today's participant
// token. The token is per participant and therefore per game day, so a token
// from a previous day is reported as DayExpired: a tab left open across
// midnight must be told to start a new life rather than shown a security error.
func CSRF(q *sqlc.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			s := SessionFrom(r.Context())
			presented := r.Header.Get(CSRFHeader)

			if subtle.ConstantTimeCompare([]byte(presented), []byte(s.Participant.CsrfToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}

			if presented != "" {
				_, err := q.GetParticipantByCookieAndCSRF(r.Context(), sqlc.GetParticipantByCookieAndCSRFParams{
					CookieToken: s.Participant.CookieToken,
					CsrfToken:   presented,
				})
				if err == nil {
					response.Error(w, apperr.DayExpired)
					return
				}
			}

			response.Error(w, apperr.InvalidCSRF)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
