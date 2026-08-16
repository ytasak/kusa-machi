package middleware

import (
	"crypto/subtle"
	"net/http"

	"kusamachi/internal/apperr"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/response"
)

// CSRFHeader は更新系リクエストで日次トークンを運ぶヘッダ。
const CSRFHeader = "X-CSRF-Token"

// CSRF は当日の participant のトークンを持たない更新系リクエストをすべて拒否する。
// トークンは participant 単位、すなわちゲーム日単位なので、前日のトークンは
// DayExpired として返す。日付をまたいで開きっぱなしのタブには、セキュリティ
// エラーではなく「新しい人生を始める」よう促すべきだから。
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
