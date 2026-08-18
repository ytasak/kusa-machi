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
	ServerTime       string       `json:"server_time"`
	GameDate         string       `json:"game_date"`
	PersonaGenerated bool         `json:"persona_generated"`
	Persona          *personaCard `json:"persona"`
	RemainingLikes   int          `json:"remaining_likes"`
	// LikeCapacity は所持上限。残数の分母として画面に出す。値は定数だが、
	// 上限を決めるのはサーバであってフロントではないのでここから配る。
	LikeCapacity      int   `json:"like_capacity"`
	ReceivedLikeCount int64 `json:"received_like_count"`
	MatchCount        int64 `json:"match_count"`
	HasUnseenLikes    bool  `json:"has_unseen_likes"`
	HasUnseenMatches  bool  `json:"has_unseen_matches"`
	// ProfileRewardAvailable は「プロフィールを完成させれば Like が回復する」
	// 状態か。画面はこれを見て事前の訴求を出す。受け取り済みなら false になり、
	// 取れない報酬を誘導しないで済む。
	ProfileRewardAvailable bool `json:"profile_reward_available"`
	// NextRecoveryAt は次に時間回復が起きる時刻。タイマーを出す状態でなければ
	// null。画面はこの null をそのまま「タイマーを表示しない」条件に使える。
	NextRecoveryAt *string `json:"next_recovery_at"`
	// LikesRecovered はこのリクエストの中で時間回復した Like の数。0 より
	// 大きいときだけ、画面が軽い通知を出す。
	LikesRecovered int    `json:"likes_recovered"`
	CSRFToken      string `json:"csrf_token"`
	// CookieReceived が2回続けて false なら、ブラウザが Cookie を保存して
	// いない。iframe 埋め込み時にサードパーティ Cookie を遮断されると起きる。
	CookieReceived bool `json:"cookie_received"`
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
		LikeCapacity:   matching.LikeCap,
		CSRFToken:      s.Participant.CsrfToken,
		CookieReceived: s.CookieReceived,
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

	// 時間回復はここで lazy に評価する。ホームは画面を開くたびに呼ばれるため、
	// 3時間ごとのジョブを持たずにこれだけで回復が行き渡る。
	p, likes, err := h.matching.SyncTimeRecovery(ctx, state.Persona)
	if err != nil {
		response.Error(w, err)
		return
	}

	card := newPersonaCard(p)
	resp.PersonaGenerated = true
	resp.Persona = &card
	resp.RemainingLikes = likes.Remaining
	resp.LikeCapacity = likes.Capacity
	resp.NextRecoveryAt = jstTime(likes.NextRecoveryAt)
	resp.LikesRecovered = likes.Recovered
	resp.ReceivedLikeCount = state.LikesReceived
	resp.MatchCount = state.MatchCount
	resp.HasUnseenLikes = state.HasUnseenLikes
	resp.HasUnseenMatches = state.HasUnseenMatches
	resp.ProfileRewardAvailable = !p.ProfileRewardClaimed

	response.JSON(w, http.StatusOK, resp)
}
