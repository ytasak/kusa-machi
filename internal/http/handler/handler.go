// Package handler implements the /api endpoints.
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

// maxBodyBytes caps request bodies; every payload in this API is tiny.
const maxBodyBytes = 8 << 10

// Handler carries the dependencies every endpoint needs.
type Handler struct {
	pool     *pgxpool.Pool
	q        *sqlc.Queries
	clock    clock.Clock
	gen      *persona.Generator
	matching *matching.Service
	photos   *photo.Store
}

// New builds the handler set.
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

// personaCard is the public shape of a persona. It deliberately excludes
// exposure count, like/match counts and any participant or cookie identity.
// Unset B attributes are omitted so the card can simply skip those rows.
// Field order mirrors the card display order in the spec.
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

	// The version in the URL changes whenever the picture does, so the response
	// can be cached hard without ever going stale.
	if p.PhotoUpdatedAt != nil {
		url := fmt.Sprintf("/api/personas/%s/photo?v=%d", p.ID, p.PhotoUpdatedAt.UnixMicro())
		card.PhotoURL = &url
	}
	return card
}

// ownPersona loads today's persona for the participant, translating "missing"
// into the domain error the API contract defines.
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

// decodeJSON reads a request body strictly: unknown fields are rejected so a
// client cannot quietly submit system-generated attributes.
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.New(apperr.CodeInvalidRequest, "invalid request body")
	}
	return nil
}
