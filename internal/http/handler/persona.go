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

// profileUpdateResponse はプロフィール保存の結果。
//
// カードを直接返さず封筒に入れるのは、プロフィール完成報酬で Like が回復した
// ことを同じ応答で伝えるため。画面はこれ1つで、カードの再描画と残数の更新と
// 回復の表示をまとめて行える。
type profileUpdateResponse struct {
	Persona        personaCard `json:"persona"`
	RemainingLikes int         `json:"remaining_likes"`
	LikeCapacity   int         `json:"like_capacity"`
	// NextRecoveryAt は保存後の次の時間回復の時刻。報酬で所持上限に達すると
	// 待つ意味が無くなるため、ここで null に変わることがある。
	NextRecoveryAt *string `json:"next_recovery_at"`
	// LikesRecovered はこのリクエストの中で時間回復した Like の数。
	LikesRecovered int `json:"likes_recovered"`
	// LikesGained は今回の保存で実際に増えた Like の数。報酬の条件を
	// 満たさないときも、所持上限で溢れたときも 0 になる。
	LikesGained int `json:"likes_gained"`
	// ProfileRewardAvailable はまだ報酬を受け取れるか。保存の直後に
	// 訴求を引っ込めるために返す。
	ProfileRewardAvailable bool `json:"profile_reward_available"`
}

// UpdateProfile は PATCH /api/persona/profile を実装する。
//
// 受け付けるのは B属性のみ。システム生成の属性を含むペイロードは黙って
// 無視するのではなく、はっきり拒否する。
//
// 保存とプロフィール完成報酬の判定はサービス側の同一トランザクションで
// 行う。したがって、同じ PATCH を何度送っても報酬は1日1回で止まる。
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

	result, err := h.matching.UpdateProfile(ctx, p.ID, profile.Name, profile.Hobby, profile.Bio)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, profileUpdateResponse{
		Persona:        newPersonaCard(result.Persona),
		RemainingLikes: result.Likes.Remaining,
		LikeCapacity:   result.Likes.Capacity,
		NextRecoveryAt: jstTime(result.Likes.NextRecoveryAt),
		LikesRecovered: result.Likes.Recovered,
		LikesGained:    result.LikesGained,
		// 報酬は同じトランザクションで判定済み。返ってきた行がそのまま答えになる。
		ProfileRewardAvailable: !result.Persona.ProfileRewardClaimed,
	})
}
