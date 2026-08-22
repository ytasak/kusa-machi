package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kusamachi/internal/apperr"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
	"kusamachi/internal/persona"
)

// childCard は子ガチャの結果の公開表現。
//
// 年齢・名前・趣味・一言は持たない。レア度も含めない。レア度の判定は §7.7 の
// とおり画面側に1か所だけ置くと決めているためで、ここに並ぶ5つの属性だけで
// その判定は完結する。属性はその日のうち変わらないので、開き直しても同じ
// レア度になる。
type childCard struct {
	Gender       string `json:"gender"`
	HeightCm     int16  `json:"height_cm"`
	Education    string `json:"education"`
	Occupation   string `json:"occupation"`
	AnnualIncome int32  `json:"annual_income"`
}

func newChildCard(c sqlc.MatchChild) childCard {
	return childCard{
		Gender:       c.Gender,
		HeightCm:     c.HeightCm,
		Education:    c.Education,
		Occupation:   c.Occupation,
		AnnualIncome: c.AnnualIncome,
	}
}

// matchDetailResponse は Match 1件の詳細。Match 成立演出を閉じた後でも、
// 一覧からここへ戻ってきて子ガチャを回収できるようにするための画面。
type matchDetailResponse struct {
	MatchID       uuid.UUID   `json:"match_id"`
	OwnPersona    personaCard `json:"own_persona"`
	TargetPersona personaCard `json:"target_persona"`
	// ChildGenerated は子ガチャを引いたか。画面はこれで「この2人の子を引く」を
	// 出すか、生成済みの子を出すかを決める。
	ChildGenerated bool       `json:"child_generated"`
	Child          *childCard `json:"child,omitempty"`
}

// MatchDetail は GET /api/matches/{matchID} を実装する。
func (h *Handler) MatchDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	own, match, err := h.ownMatch(r)
	if err != nil {
		response.Error(w, err)
		return
	}

	resp := matchDetailResponse{
		MatchID:       match.MatchID,
		OwnPersona:    newPersonaCard(own),
		TargetPersona: newPersonaCard(match.Persona),
	}

	child, err := h.q.GetMatchChild(ctx, match.MatchID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// まだ引いていない。画面は CTA を出す。
	case err != nil:
		response.Error(w, err)
		return
	default:
		card := newChildCard(child)
		resp.ChildGenerated = true
		resp.Child = &card
	}

	response.JSON(w, http.StatusOK, resp)
}

// CreateMatchChild は POST /api/matches/{matchID}/child を実装する。
//
// 1 Match につき子は1人だけで、引き直しはできない。この処理は冪等であり、
// すでに子がいる Match に対しては再抽選せず、保存済みの子をそのまま返す。
// 多重クリックや再送で2人目が生まれることはない。
//
// 保証しているのは match_children.match_id の UNIQUE 制約であって、この関数の
// 事前確認ではない。同時に2つ届いた場合、片方の INSERT が制約に当たり、
// ON CONFLICT により先に入った子が返る。
func (h *Handler) CreateMatchChild(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	own, match, err := h.ownMatch(r)
	if err != nil {
		response.Error(w, err)
		return
	}

	// すでにいるなら、そこで終わり。無駄に抽選しない。
	if existing, err := h.q.GetMatchChild(ctx, match.MatchID); err == nil {
		response.JSON(w, http.StatusOK, newChildCard(existing))
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, err)
		return
	}

	// 親から渡すのは身長と学歴だけ。職業・年収・性別・レア度は親と無関係に
	// 引き直す（internal/persona/child.go を参照）。
	attrs := h.gen.GenerateChild(parentOf(own), parentOf(match.Persona))

	child, err := h.q.InsertMatchChild(ctx, sqlc.InsertMatchChildParams{
		ID:           uuid.New(),
		MatchID:      match.MatchID,
		Gender:       attrs.Gender,
		HeightCm:     int16(attrs.HeightCm),
		Education:    attrs.Education,
		Occupation:   attrs.Occupation,
		AnnualIncome: int32(attrs.AnnualIncome),
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, newChildCard(child))
}

func parentOf(p sqlc.Persona) persona.Parent {
	return persona.Parent{HeightCm: int(p.HeightCm), Education: p.Education}
}

// ownMatch は URL の matchID を読み、それが「自分が含まれる当日の Match」で
// あることを確かめて、自分の Persona と Match（相手 Persona 付き）を返す。
//
// 他人の Match を指定した場合も、存在しない Match を指定した場合も、前日の
// Match を指定した場合も、区別せず MatchUnavailable になる。どれなのかを
// 教えると、自分が含まれない Match の存在を外から数えられてしまう。
func (h *Handler) ownMatch(r *http.Request) (sqlc.Persona, sqlc.GetMatchForPersonaRow, error) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	matchID, parseErr := uuid.Parse(chi.URLParam(r, "matchID"))
	if parseErr != nil {
		return sqlc.Persona{}, sqlc.GetMatchForPersonaRow{},
			apperr.New(apperr.CodeInvalidRequest, "match_id must be a uuid")
	}

	own, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		return sqlc.Persona{}, sqlc.GetMatchForPersonaRow{}, err
	}

	match, err := h.q.GetMatchForPersona(ctx, sqlc.GetMatchForPersonaParams{
		PersonaID: own.ID,
		MatchID:   matchID,
		GameDate:  s.GameDate,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Persona{}, sqlc.GetMatchForPersonaRow{}, apperr.MatchUnavailable
	}
	if err != nil {
		return sqlc.Persona{}, sqlc.GetMatchForPersonaRow{}, err
	}
	return own, match, nil
}
