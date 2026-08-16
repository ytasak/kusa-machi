package handler

import (
	"errors"
	"net/http"
	"time"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
	"kusamachi/internal/matching"
)

type homeResponse struct {
	ServerTime        string       `json:"server_time"`
	GameDate          string       `json:"game_date"`
	PersonaGenerated  bool         `json:"persona_generated"`
	Persona           *personaCard `json:"persona"`
	RemainingLikes    int          `json:"remaining_likes"`
	ReceivedLikeCount int64        `json:"received_like_count"`
	MatchCount        int64        `json:"match_count"`
	HasUnseenLikes    bool         `json:"has_unseen_likes"`
	HasUnseenMatches  bool         `json:"has_unseen_matches"`
	CSRFToken         string       `json:"csrf_token"`
}

// Home はホーム画面の状態を丸ごと返す。この処理が動く時点で、セッション
// ミドルウェアが当日の participant の存在をすでに保証している。
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	resp := homeResponse{
		ServerTime:     s.Now.In(clock.JST).Format(time.RFC3339),
		GameDate:       clock.FormatGameDate(s.GameDate),
		RemainingLikes: matching.DailyLikeBudget,
		CSRFToken:      s.Participant.CsrfToken,
	}

	p, err := h.ownPersona(ctx, s.Participant.ID)
	if errors.Is(err, apperr.PersonaNotGenerated) {
		// Persona がまだ無い日は正常な状態であって、エラーではない。
		response.JSON(w, http.StatusOK, resp)
		return
	}
	if err != nil {
		response.Error(w, err)
		return
	}

	card := newPersonaCard(p)
	resp.PersonaGenerated = true
	resp.Persona = &card

	sent, err := h.q.CountLikesSent(ctx, p.ID)
	if err != nil {
		response.Error(w, err)
		return
	}
	resp.RemainingLikes = matching.RemainingLikes(sent)

	if resp.ReceivedLikeCount, err = h.q.CountLikesReceived(ctx, p.ID); err != nil {
		response.Error(w, err)
		return
	}
	if resp.MatchCount, err = h.q.CountMatches(ctx, p.ID); err != nil {
		response.Error(w, err)
		return
	}
	if resp.HasUnseenLikes, err = h.q.HasLikesReceivedSince(ctx, sqlc.HasLikesReceivedSinceParams{
		PersonaID: p.ID,
		Since:     s.Participant.LikesLastSeenAt,
	}); err != nil {
		response.Error(w, err)
		return
	}
	if resp.HasUnseenMatches, err = h.q.HasMatchesSince(ctx, sqlc.HasMatchesSinceParams{
		PersonaID: p.ID,
		Since:     s.Participant.MatchesLastSeenAt,
	}); err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}
