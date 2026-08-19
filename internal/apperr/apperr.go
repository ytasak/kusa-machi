// Package apperr は API 契約が公開するドメインエラーコードを定義する。
// ドメイン層はこれを返し、HTTP 層がステータスコードへ対応付ける。
package apperr

import "net/http"

// Code はフロントエンドが分岐に使う機械可読な値。
type Code string

const (
	CodePersonaNotGenerated      Code = "PersonaNotGenerated"
	CodeLikeLimitExceeded        Code = "LikeLimitExceeded"
	CodeAlreadyLiked             Code = "AlreadyLiked"
	CodeTargetPersonaUnavailable Code = "TargetPersonaUnavailable"
	CodeSelfActionNotAllowed     Code = "SelfActionNotAllowed"
	CodeDayExpired               Code = "DayExpired"
	CodeInvalidProfileInput      Code = "InvalidProfileInput"

	// ドメインの語彙ではないが、フロントエンドが分岐できるよう
	// 安定したコードが必要な転送層のエラー。
	CodeInvalidRequest Code = "InvalidRequest"
	CodeInvalidCSRF    Code = "InvalidCSRFToken"
	CodeInternal       Code = "InternalError"
)

// Error は API エラーコードを持つドメインエラー。
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// New はドメインエラーを組み立てる。
func New(code Code, message string) *Error { return &Error{Code: code, Message: message} }

// 個別のメッセージが不要なケース向けの定義済みエラー。
var (
	PersonaNotGenerated      = New(CodePersonaNotGenerated, "今日のPersonaがまだ生成されていません")
	LikeLimitExceeded        = New(CodeLikeLimitExceeded, "今日のLikeを使い切りました")
	AlreadyLiked             = New(CodeAlreadyLiked, "この相手にはすでにLikeを送っています")
	TargetPersonaUnavailable = New(CodeTargetPersonaUnavailable, "対象のPersonaは利用できません")
	SelfActionNotAllowed     = New(CodeSelfActionNotAllowed, "自分のPersonaへの操作はできません")
	DayExpired               = New(CodeDayExpired, "ゲーム日が終了しています")
	InvalidCSRF              = New(CodeInvalidCSRF, "CSRFトークンが不正です")
)

// HTTPStatus はコードを仕様書で定義された HTTP ステータスへ対応付ける。
func HTTPStatus(code Code) int {
	switch code {
	case CodeInvalidProfileInput, CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeInvalidCSRF:
		return http.StatusForbidden
	case CodePersonaNotGenerated, CodeTargetPersonaUnavailable:
		return http.StatusNotFound
	case CodeAlreadyLiked, CodeDayExpired:
		return http.StatusConflict
	case CodeLikeLimitExceeded, CodeSelfActionNotAllowed:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
