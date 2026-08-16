// Package handler は /api のエンドポイントを実装する。
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/matching"
	"kusamachi/internal/persona"
	"kusamachi/internal/photo"
)

// maxBodyBytes はリクエストボディの上限。この API のペイロードはどれも小さい。
const maxBodyBytes = 8 << 10

// Handler は各エンドポイントが必要とする依存を持つ。
type Handler struct {
	pool     *pgxpool.Pool
	q        *sqlc.Queries
	clock    clock.Clock
	gen      *persona.Generator
	matching *matching.Service
	photos   *photo.Store
}

// New はハンドラ一式を組み立てる。
func New(pool *pgxpool.Pool, clk clock.Clock, gen *persona.Generator, photos *photo.Store) *Handler {
	return &Handler{
		pool:     pool,
		q:        sqlc.New(pool),
		clock:    clk,
		gen:      gen,
		matching: matching.NewService(pool),
		photos:   photos,
	}
}

// personaCard は Persona の公開表現。exposure 数・Like/Match 数・participant や
// Cookie の identity は意図的に含めない。未設定の B属性は省略するので、
// カード側はその行を出さないだけでよい。
// フィールドの並びは仕様のカード表示順に合わせている。
type personaCard struct {
	ID           uuid.UUID `json:"id"`
	Name         *string   `json:"name,omitempty"`
	Age          int16     `json:"age"`
	Gender       string    `json:"gender"`
	HeightCm     int16     `json:"height_cm"`
	Occupation   string    `json:"occupation"`
	AnnualIncome int32     `json:"annual_income"`
	Education    string    `json:"education"`
	Hobby        *string   `json:"hobby,omitempty"`
	Bio          *string   `json:"bio,omitempty"`
	PhotoURL     *string   `json:"photo_url,omitempty"`
}

func newPersonaCard(p sqlc.Persona) personaCard {
	card := personaCard{
		ID:           p.ID,
		Name:         p.Name,
		Age:          p.Age,
		Gender:       p.Gender,
		HeightCm:     p.HeightCm,
		Occupation:   p.Occupation,
		AnnualIncome: p.AnnualIncome,
		Education:    p.Education,
		Hobby:        p.Hobby,
		Bio:          p.Bio,
	}

	// URL のバージョンは写真が変わるたびに変わるため、強くキャッシュしても
	// 古い画像が残ることはない。
	if p.PhotoUpdatedAt != nil {
		url := fmt.Sprintf("/api/personas/%s/photo?v=%d", p.ID, p.PhotoUpdatedAt.UnixMicro())
		card.PhotoURL = &url
	}
	return card
}

// ownPersona は participant の当日 Persona を読み込み、「存在しない」を
// API 契約が定めるドメインエラーに変換する。
func (h *Handler) ownPersona(ctx context.Context, participantID uuid.UUID) (sqlc.Persona, error) {
	p, err := h.q.GetPersonaByParticipant(ctx, participantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Persona{}, apperr.PersonaNotGenerated
	}
	if err != nil {
		return sqlc.Persona{}, err
	}
	return p, nil
}

// decodeJSON はリクエストボディを厳格に読む。未知のフィールドを拒否することで、
// クライアントがシステム生成の属性をこっそり送ることを防ぐ。
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.New(apperr.CodeInvalidRequest, "invalid request body")
	}
	return nil
}
