package handler

import (
	"net/http"

	"github.com/google/uuid"

	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
	"kusamachi/internal/persona"
)

// GeneratePersona は POST /api/persona を実装する。
//
// 構造的に冪等。既存の Persona はそのまま返し、personas.participant_id の
// 一意インデックスにより、競合に負けた場合も2回目の抽選ではなく同じ答えになる。
// 同じ日のうちに Persona が振り直されることはない。
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

// MyPersona は GET /api/persona/me を実装する。
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

// UpdateProfile は PATCH /api/persona/profile を実装する。
//
// 受け付けるのは B属性のみ。システム生成の属性を含むペイロードは黙って
// 無視するのではなく、はっきり拒否する。
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
