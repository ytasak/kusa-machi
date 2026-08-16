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

// sentLikeCard は公開カードに MATCH バッジ用のフラグを足したもの。埋め込み構造体は
// encoding/json によって展開されるので、形はただの Persona カードのまま。
type sentLikeCard struct {
	personaCard
	Matched bool `json:"matched"`
}

type sentLikeListResponse struct {
	Personas []sentLikeCard `json:"personas"`
}

// ReceivedLikes は GET /api/likes/received を実装する。
//
// 画面を開くと「新しいLikeがあります」バッジが消える。仕様が GET に許している
// 唯一の状態変更。
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

// SentLikes は GET /api/likes/sent を実装する。
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

// Matches は GET /api/matches を実装する。
//
// 画面を開くと「新しいMatchがあります！」バッジが消える。
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
