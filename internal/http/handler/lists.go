package handler

import (
	"net/http"

	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
)

type personaListResponse struct {
	Personas []personaCard `json:"personas"`
}

// sentLikeCard is the public card plus the MATCH badge flag. The embedded
// struct is inlined by encoding/json, so the shape stays a plain persona card.
type sentLikeCard struct {
	personaCard
	Matched bool `json:"matched"`
}

type sentLikeListResponse struct {
	Personas []sentLikeCard `json:"personas"`
}

// ReceivedLikes implements GET /api/likes/received.
//
// Opening the screen clears the "新しいLikeがあります" badge. This is the one
// state change the spec allows a GET to make.
func (h *Handler) ReceivedLikes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	self, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	rows, err := h.q.ListReceivedLikes(ctx, self.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	if err := h.q.MarkLikesSeen(ctx, s.Participant.ID); err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, personaListResponse{Personas: toCards(rows)})
}

// SentLikes implements GET /api/likes/sent.
func (h *Handler) SentLikes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	self, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	rows, err := h.q.ListSentLikes(ctx, self.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	cards := make([]sentLikeCard, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, sentLikeCard{
			personaCard: newPersonaCard(sqlc.Persona{
				ID:           row.ID,
				Age:          row.Age,
				Gender:       row.Gender,
				HeightCm:     row.HeightCm,
				Education:    row.Education,
				Occupation:   row.Occupation,
				AnnualIncome: row.AnnualIncome,
				Name:         row.Name,
				Hobby:        row.Hobby,
				Bio:          row.Bio,
			}),
			Matched: row.Matched,
		})
	}

	response.JSON(w, http.StatusOK, sentLikeListResponse{Personas: cards})
}

// Matches implements GET /api/matches.
//
// Opening the screen clears the "新しいMatchがあります！" badge.
func (h *Handler) Matches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	self, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	rows, err := h.q.ListMatches(ctx, self.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	if err := h.q.MarkMatchesSeen(ctx, s.Participant.ID); err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, personaListResponse{Personas: toCards(rows)})
}

func toCards(rows []sqlc.Persona) []personaCard {
	cards := make([]personaCard, 0, len(rows))
	for _, p := range rows {
		cards = append(cards, newPersonaCard(p))
	}
	return cards
}
