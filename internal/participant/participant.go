// Package participant は匿名のブラウザ identity を担う。Cookie のトークンと、
// Cookie × ゲーム日ごとに1行だけ存在する participant を扱う。
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

// CookieName は匿名のブラウザ識別子。ゲームの状態は一切持たない。
const CookieName = "kusa_machi_token"

// cookieMaxAge は仕様が要求する約30日の有効期限。
const cookieMaxAge = 30 * 24 * 60 * 60

// csrfTokenBytes は日次 CSRF トークンのエントロピー。
const csrfTokenBytes = 32

// CookieConfig は Cookie に付与する属性を持つ。
// Secure と SameSite を可変にしているのはローカルの平文 http で動作確認する
// ためだけで、本番は iframe 埋め込みのため Secure + SameSite=None を使う。
type CookieConfig struct {
	Secure   bool
	SameSite http.SameSite
}

// ReadToken は Cookie のトークンが存在し形式が正しい場合にそれを返す。
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

// SetToken は Cookie を書き込む。リクエストのたびに有効期限が延びる。
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

// NewToken は新しい匿名ブラウザ identity を発行する。
func NewToken() uuid.UUID { return uuid.New() }

// Ensure は Cookie トークンに対応する当日の participant を返し、初回アクセス時は
// 作成する。(cookie_token, game_date) の一意インデックスにより競合に強く、
// 同時に来た初回リクエストは同じ行に収束する。
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

// NewCSRFToken は32バイトの乱数から Base64URL の不透明トークンを生成する。
func NewCSRFToken() (string, error) {
	buf := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
