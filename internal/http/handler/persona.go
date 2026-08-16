package handler

import (
	"net/http"

	"github.com/google/uuid"

	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
	"kusamachi/internal/persona"
)

// GeneratePersona implements POST /api/persona.
//
// Idempotent by construction: an existing persona is returned untouched, and
// the unique index on personas.participant_id turns a lost race into the same
// answer rather than a second roll. A persona is never rerolled within a day.
func (h *Handler) GeneratePersona(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	if existing, err := h.q.GetPersonaByParticipant(ctx, s.Participant.ID); err == nil {
		response.JSON(w, http.StatusOK, newPersonaCard(existing))
		return
	}

	attrs := h.gen.Generate()
	p, err := h.q.InsertPersona(ctx, sqlc.InsertPersonaParams{
		ID:            uuid.New(),
		ParticipantID: s.Participant.ID,
		Age:           int16(attrs.Age),
		Gender:        attrs.Gender,
		HeightCm:      int16(attrs.HeightCm),
		Education:     attrs.Education,
		Occupation:    attrs.Occupation,
		AnnualIncome:  int32(attrs.AnnualIncome),
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, newPersonaCard(p))
}

// MyPersona implements GET /api/persona/me.
func (h *Handler) MyPersona(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	p, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, newPersonaCard(p))
}

// UpdateProfile implements PATCH /api/persona/profile.
//
// Only the B attributes are accepted; a payload carrying a system-generated
// attribute is rejected outright rather than silently ignored.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	var input persona.ProfileInput
	if err := decodeJSON(r, &input); err != nil {
		response.Error(w, err)
		return
	}

	profile, err := input.Validate()
	if err != nil {
		response.Error(w, err)
		return
	}

	p, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	updated, err := h.q.UpdatePersonaProfile(ctx, sqlc.UpdatePersonaProfileParams{
		ID:    p.ID,
		Name:  profile.Name,
		Hobby: profile.Hobby,
		Bio:   profile.Bio,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, newPersonaCard(updated))
}
