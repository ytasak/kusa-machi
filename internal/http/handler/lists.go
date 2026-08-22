package handler

import (
	"net/http"

	"github.com/google/uuid"

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

// matchCard は Match 一覧の1件。相手の公開カードに、詳細を開くための match_id と
// 子ガチャを引いたかどうかの目印を添える。子の中身は一覧には出さない。
type matchCard struct {
	personaCard
	MatchID        uuid.UUID `json:"match_id"`
	ChildGenerated bool      `json:"child_generated"`
}

type matchListResponse struct {
	Personas []matchCard `json:"personas"`
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
			personaCard: newPersonaCard(row.Persona),
			Matched:     row.Matched,
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

	cards := make([]matchCard, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, matchCard{
			personaCard:    newPersonaCard(row.Persona),
			MatchID:        row.MatchID,
			ChildGenerated: row.ChildGenerated,
		})
	}

	response.JSON(w, http.StatusOK, matchListResponse{Personas: cards})
}

func toCards(rows []sqlc.Persona) []personaCard {
	cards := make([]personaCard, 0, len(rows))
	for _, p := range rows {
		cards = append(cards, newPersonaCard(p))
	}
	return cards
}
