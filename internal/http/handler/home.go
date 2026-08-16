package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

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
//
// Persona とカウンタは1本のクエリでまとめて取る。画面を開くたびに呼ばれる
// エンドポイントなので、往復回数がそのまま体感速度になる。
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	resp := homeResponse{
		ServerTime:     s.Now.In(clock.JST).Format(time.RFC3339),
		GameDate:       clock.FormatGameDate(s.GameDate),
		RemainingLikes: matching.DailyLikeBudget,
		CSRFToken:      s.Participant.CsrfToken,
	}

	state, err := h.q.GetHomeState(ctx, sqlc.GetHomeStateParams{
		ParticipantID:     s.Participant.ID,
		LikesLastSeenAt:   s.Participant.LikesLastSeenAt,
		MatchesLastSeenAt: s.Participant.MatchesLastSeenAt,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Persona がまだ無い日は正常な状態であって、エラーではない。
		response.JSON(w, http.StatusOK, resp)
		return
	}
	if err != nil {
		response.Error(w, err)
		return
	}

	card := newPersonaCard(state.Persona)
	resp.PersonaGenerated = true
	resp.Persona = &card
	resp.RemainingLikes = matching.RemainingLikes(state.LikesSent)
	resp.ReceivedLikeCount = state.LikesReceived
	resp.MatchCount = state.MatchCount
	resp.HasUnseenLikes = state.HasUnseenLikes
	resp.HasUnseenMatches = state.HasUnseenMatches

	response.JSON(w, http.StatusOK, resp)
}
