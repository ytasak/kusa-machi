// Package apperr defines the domain error codes the API contract exposes.
// Domain packages return these; the HTTP layer maps them to status codes.
package apperr

import "net/http"

// Code is the machine-readable value the frontend branches on.
type Code string

const (
	CodePersonaNotGenerated      Code = "PersonaNotGenerated"
	CodeLikeLimitExceeded        Code = "LikeLimitExceeded"
	CodeAlreadyLiked             Code = "AlreadyLiked"
	CodeTargetPersonaUnavailable Code = "TargetPersonaUnavailable"
	CodePassLimitReached         Code = "PassLimitReached"
	CodeSelfActionNotAllowed     Code = "SelfActionNotAllowed"
	CodeDayExpired               Code = "DayExpired"
	CodeInvalidProfileInput      Code = "InvalidProfileInput"

	// Transport-level codes that are not part of the domain vocabulary but
	// still need a stable code for the frontend.
	CodeInvalidRequest Code = "InvalidRequest"
	CodeInvalidCSRF    Code = "InvalidCSRFToken"
	CodeInternal       Code = "InternalError"
)

// Error is a domain error carrying an API error code.
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// New builds a domain error.
func New(code Code, message string) *Error { return &Error{Code: code, Message: message} }

// Predefined errors for the cases that never need a custom message.
var (
	PersonaNotGenerated      = New(CodePersonaNotGenerated, "persona is not generated for today")
	LikeLimitExceeded        = New(CodeLikeLimitExceeded, "like limit exceeded")
	AlreadyLiked             = New(CodeAlreadyLiked, "target persona is already liked")
	TargetPersonaUnavailable = New(CodeTargetPersonaUnavailable, "target persona is unavailable")
	PassLimitReached         = New(CodePassLimitReached, "pass limit reached")
	SelfActionNotAllowed     = New(CodeSelfActionNotAllowed, "action against own persona is not allowed")
	DayExpired               = New(CodeDayExpired, "the game day has expired")
	InvalidCSRF              = New(CodeInvalidCSRF, "invalid csrf token")
)

// HTTPStatus maps a code to the status defined in the API spec.
func HTTPStatus(code Code) int {
	switch code {
	case CodeInvalidProfileInput, CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeInvalidCSRF:
		return http.StatusForbidden
	case CodePersonaNotGenerated, CodeTargetPersonaUnavailable:
		return http.StatusNotFound
	case CodeAlreadyLiked, CodePassLimitReached, CodeDayExpired:
		return http.StatusConflict
	case CodeLikeLimitExceeded, CodeSelfActionNotAllowed:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
