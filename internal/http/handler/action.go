package handler

import (
	"net/http"

	"github.com/google/uuid"

	"kusamachi/internal/apperr"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
)

// targetRequest は Like と Pass のエンドポイントで共通のリクエストボディ。
type targetRequest struct {
	PersonaID string `json:"persona_id"`
}

type likeResponse struct {
	RemainingLikes int        `json:"remaining_likes"`
	Matched        bool       `json:"matched"`
	MatchID        *uuid.UUID `json:"match_id,omitempty"`
	// NextRecoveryAt はこの Like を消費した後の、次の時間回復の時刻。その日の
	// 1つ目の Like がタイマーを開始させるので、多くの場合ここで初めて値が入る。
	NextRecoveryAt *string `json:"next_recovery_at"`
	// LikesRecovered はこのリクエストの中で時間回復した Like の数。残数0の
	// まま開いていた画面から送ったときに 0 より大きくなることがある。
	LikesRecovered int `json:"likes_recovered"`
	// LikesGained は Match 報酬で実際に増えた Like の数。0 のときは画面が
	// 回復の表示を省くので、所持上限で何も増えなかった Match は静かに終わる。
	LikesGained   int          `json:"likes_gained"`
	TargetPersona *personaCard `json:"target_persona,omitempty"`
}

type passResponse struct {
	PassCount        int  `json:"pass_count"`
	ExcludedForToday bool `json:"excluded_for_today"`
}

// CreateLike は POST /api/likes を実装する。
func (h *Handler) CreateLike(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	targetID, actor, err := h.readAction(r)
	if err != nil {
		response.Error(w, err)
		return
	}

	result, err := h.matching.Like(ctx, actor, targetID, s.GameDate)
	if err != nil {
		response.Error(w, err)
		return
	}

	resp := likeResponse{
		RemainingLikes: result.Likes.Remaining,
		NextRecoveryAt: jstTime(result.Likes.NextRecoveryAt),
		LikesRecovered: result.Likes.Recovered,
		Matched:        result.Matched,
		LikesGained:    result.LikesGained,
	}
	if result.Matched {
		// Match アニメーションは相手のカードを即座に必要とする。
		card := newPersonaCard(result.Target)
		resp.MatchID = result.MatchID
		resp.TargetPersona = &card
	}

	response.JSON(w, http.StatusOK, resp)
}

// CreatePass は POST /api/passes を実装する。
func (h *Handler) CreatePass(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	targetID, actor, err := h.readAction(r)
	if err != nil {
		response.Error(w, err)
		return
	}

	result, err := h.matching.Pass(ctx, actor, targetID, s.GameDate)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, passResponse{
		PassCount:        result.PassCount,
		ExcludedForToday: result.ExcludedForToday,
	})
}

// readAction は対象 Persona の id をデコードし、操作する側の Persona を読み込む。
func (h *Handler) readAction(r *http.Request) (uuid.UUID, sqlc.Persona, error) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	var body targetRequest
	if err := decodeJSON(r, &body); err != nil {
		return uuid.Nil, sqlc.Persona{}, err
	}

	targetID, parseErr := uuid.Parse(body.PersonaID)
	if parseErr != nil {
		return uuid.Nil, sqlc.Persona{}, apperr.New(apperr.CodeInvalidRequest, "persona_id must be a uuid")
	}

	actor, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		return uuid.Nil, sqlc.Persona{}, err
	}
	return targetID, actor, nil
}
