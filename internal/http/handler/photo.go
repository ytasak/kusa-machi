package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kusamachi/internal/apperr"
	"kusamachi/internal/db/sqlc"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/http/response"
	"kusamachi/internal/photo"
)

// UploadPhoto は POST /api/persona/photo を実装する。
//
// ボディは生の画像。クライアントが先に縮小し、サーバは届いたものを
// 必ず再エンコードする。
func (h *Handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	self, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	body := http.MaxBytesReader(w, r.Body, photo.MaxUploadBytes)
	if err := h.photos.Save(s.GameDate, self.ID, body); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.Fail(w, apperr.CodeInvalidProfileInput, "画像が大きすぎます")
			return
		}
		response.Error(w, err)
		return
	}

	if _, err := h.q.SetPersonaPhoto(ctx, self.ID); err != nil {
		response.Error(w, err)
		return
	}

	updated, err := h.q.GetPersonaByID(ctx, self.ID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, newPersonaCard(updated))
}

// DeletePhoto は DELETE /api/persona/photo を実装し、写真を取り消せるようにする。
func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	self, err := h.ownPersona(ctx, s.Participant.ID)
	if err != nil {
		response.Error(w, err)
		return
	}

	if err := h.photos.Delete(s.GameDate, self.ID); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.q.ClearPersonaPhoto(ctx, self.ID); err != nil {
		response.Error(w, err)
		return
	}

	updated, err := h.q.GetPersonaByID(ctx, self.ID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, newPersonaCard(updated))
}

// GetPhoto は GET /api/personas/{personaID}/photo を実装する。
func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := middleware.SessionFrom(ctx)

	personaID, err := uuid.Parse(chi.URLParam(r, "personaID"))
	if err != nil {
		response.Fail(w, apperr.CodeInvalidRequest, "persona_id must be a uuid")
		return
	}

	// 見えるのは当日の Persona だけ。他の読み取りと同じ扱い。
	target, err := h.q.GetActivePersona(ctx, sqlc.GetActivePersonaParams{
		PersonaID: personaID,
		GameDate:  s.GameDate,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, apperr.TargetPersonaUnavailable)
		return
	}
	if err != nil {
		response.Error(w, err)
		return
	}
	if target.PhotoUpdatedAt == nil {
		response.Error(w, apperr.TargetPersonaUnavailable)
		return
	}

	file, err := h.photos.Open(s.GameDate, target.ID)
	if err != nil {
		response.Error(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", photo.ContentType)
	// URL にバージョンが入っているため、その URL の中身が変わることはない。
	w.Header().Set("Cache-Control", "private, max-age=86400")
	// 念のため、ブラウザが画像以外として解釈することを絶対に許さない。
	w.Header().Set("X-Content-Type-Options", "nosniff")

	http.ServeContent(w, r, strconv.FormatInt(target.PhotoUpdatedAt.Unix(), 10)+".jpg", *target.PhotoUpdatedAt, file)
}
