package handler

import (
	"errors"
	"net/http"
	"strings"

	"dormhub/config"
	"dormhub/store"
)

// ListReviews GET /api/dorms/{id}/reviews  (Guest ดูได้)
func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	dormID, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	items, err := h.Store.ListReviews(r.Context(), dormID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type reviewReq struct {
	Rating int    `json:"rating"`
	Text   string `json:"text"`
}

// CreateReview POST /api/dorms/{id}/reviews  (Member)
func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	dormID, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	var req reviewReq
	if !readJSON(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if msg := checkReview(req.Rating, text); msg != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", msg)
		return
	}
	// หอที่ยังไม่เผยแพร่ รีวิวไม่ได้
	if _, err := h.Store.GetDorm(r.Context(), dormID, false); err != nil {
		if isNotFound(err) {
			notFound(w, "หอพักนี้")
			return
		}
		internal(w, err)
		return
	}
	rv, err := h.Store.CreateReview(r.Context(), who(r).ID, dormID, req.Rating, text)
	switch {
	case errors.Is(err, store.ErrDuplicate):
		writeError(w, http.StatusConflict, "ALREADY_REVIEWED", "คุณรีวิวหอนี้ไปแล้ว")
		return
	case errors.Is(err, store.ErrBadReference):
		notFound(w, "หอพักนี้")
		return
	case err != nil:
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rv)
}

// DeleteReview DELETE /api/reviews/{id}
// Member ลบได้เฉพาะรีวิวของตัวเอง ส่วน Admin ลบได้ทุกรีวิว (ตรวจที่ Backend ไม่ใช่แค่ซ่อนปุ่ม)
func (h *Handler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	me := who(r)
	owner := me.ID
	if me.Role == config.RoleAdmin {
		owner = 0 // 0 = ไม่จำกัดเจ้าของ
	}
	err := h.Store.DeleteReview(r.Context(), id, owner)
	if isNotFound(err) {
		notFound(w, "รีวิวนี้ (หรือไม่ใช่รีวิวของคุณ)")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- รายการโปรด (Member) ----------

// ListFavorites GET /api/me/favorites
func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListFavorites(r.Context(), who(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AddFavorite PUT /api/me/favorites/{dormId}  (PUT เพราะกดซ้ำกี่ครั้งผลก็เหมือนเดิม)
func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	dormID, ok := pathID(r, "dormId")
	if !ok {
		badID(w)
		return
	}
	err := h.Store.AddFavorite(r.Context(), who(r).ID, dormID)
	if errors.Is(err, store.ErrBadReference) {
		notFound(w, "หอพักนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveFavorite DELETE /api/me/favorites/{dormId}
func (h *Handler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	dormID, ok := pathID(r, "dormId")
	if !ok {
		badID(w)
		return
	}
	if err := h.Store.RemoveFavorite(r.Context(), who(r).ID, dormID); err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
