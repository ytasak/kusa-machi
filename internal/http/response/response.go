// Package response は API の成功／エラーの JSON ペイロードを書き出す。
package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"kusamachi/internal/apperr"
)

type errorBody struct {
	Code    apperr.Code `json:"code"`
	Message string      `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// JSON は指定したステータスで v を書き出す。
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "error", err)
	}
}

// NoContent は空の 204 を書き出す。
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Error は err を仕様のエラーエンベロープとして書き出す。未知のエラーは 500 に
// なりログに残る。クライアントへ内容が漏れることはない。
func Error(w http.ResponseWriter, err error) {
	var domainErr *apperr.Error
	if !errors.As(err, &domainErr) {
		slog.Error("unhandled error", "error", err)
		domainErr = apperr.New(apperr.CodeInternal, "internal server error")
	}
	Fail(w, domainErr.Code, domainErr.Message)
}

// Fail は指定したコードでエラーエンベロープを書き出す。
func Fail(w http.ResponseWriter, code apperr.Code, message string) {
	JSON(w, apperr.HTTPStatus(code), errorEnvelope{Error: errorBody{Code: code, Message: message}})
}
