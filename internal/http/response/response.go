// Package response writes the API's JSON success and error payloads.
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

// JSON writes v with the given status.
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

// NoContent writes an empty 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Error writes err as the spec's error envelope. Unknown errors become 500 and
// are logged, never leaked to the client.
func Error(w http.ResponseWriter, err error) {
	var domainErr *apperr.Error
	if !errors.As(err, &domainErr) {
		slog.Error("unhandled error", "error", err)
		domainErr = apperr.New(apperr.CodeInternal, "internal server error")
	}
	Fail(w, domainErr.Code, domainErr.Message)
}

// Fail writes an error envelope for an explicit code.
func Fail(w http.ResponseWriter, code apperr.Code, message string) {
	JSON(w, apperr.HTTPStatus(code), errorEnvelope{Error: errorBody{Code: code, Message: message}})
}
