package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"kusamachi/internal/apperr"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
)

// discoverBatchSize は Discover が1回に返すカード数。
const discoverBatchSize = 5

// maxExcludeIDs はクライアントが送れるクールダウン一覧の上限。
const maxExcludeIDs = 50

type discoverResponse struct {
	Personas []personaCard `json:"personas"`
	// 残数まわりも返す。探索画面は長く開かれたままになるので、カードを
	// 継ぎ足すこの往復が、時間回復をヘッダーに反映する機会にもなる。
	RemainingLikes int     `json:"remaining_likes"`
	LikeCapacity   int     `json:"like_capacity"`
	NextRecoveryAt *string `json:"next_recovery_at"`
	LikesRecovered int     `json:"likes_recovered"`
}

// Discover は GET /api/discover を実装する。
//
// バッチを返すだけでは exposure_count に触れない。exposure が数えるのは
// Like か Pass で実際に評価されたプロフィールだけ。
func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	self, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	// ホームと同じ lazy 評価。探索を開いたままの人にも回復が届く。
	self, likes, err := h.matching.SyncTimeRecovery(ctx, self)
	if err != nil {
		response.Error(w, err)
		return
	}

	exclude, err := parseExcludeIDs(r.URL.Query().Get("exclude"))
	if err != nil {
		response.Error(w, err)
		return
	}

	rows, err := h.q.DiscoverCandidates(ctx, sqlc.DiscoverCandidatesParams{
		SelfID:     self.ID,
		GameDate:   s.GameDate,
		ExcludeIds: exclude,
		LimitCount: discoverBatchSize,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	cards := make([]personaCard, 0, len(rows))
	for _, p := range rows {
		cards = append(cards, newPersonaCard(p))
	}

	response.JSON(w, http.StatusOK, discoverResponse{
		Personas:       cards,
		RemainingLikes: likes.Remaining,
		LikeCapacity:   likes.Capacity,
		NextRecoveryAt: jstTime(likes.NextRecoveryAt),
		LikesRecovered: likes.Recovered,
	})
}

// parseExcludeIDs はフロントエンドが持つローカルな Pass クールダウン一覧を読む。
func parseExcludeIDs(raw string) ([]uuid.UUID, error) {
	if raw == "" {
		return []uuid.UUID{}, nil
	}

	parts := strings.Split(raw, ",")
	if len(parts) > maxExcludeIDs {
		return nil, apperr.New(apperr.CodeInvalidRequest, "too many exclude ids")
	}

	ids := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			return nil, apperr.New(apperr.CodeInvalidRequest, "exclude must be a comma separated list of persona ids")
		}
		ids = append(ids, id)
	}
	return ids, nil
}
