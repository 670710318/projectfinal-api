package handler

import (
	"net/http"

	"dormhub/config"
)

// endpoint ทุกตัวในไฟล์นี้ครอบด้วย mw.RequireAdmin ใน main.go แล้ว
// Member ที่ยิงตรงมาจะได้ 403 ก่อนถึงโค้ดในไฟล์นี้

// AdminStats GET /api/admin/stats  ตัวเลขสรุปบนหน้า Dashboard
func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.Stats(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// AdminListMembers GET /api/admin/members
func (h *Handler) AdminListMembers(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListMembers(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type roleReq struct {
	Role string `json:"role"`
}

// AdminSetRole PUT /api/admin/members/{id}/role  body: {"role":"admin"|"member"}
func (h *Handler) AdminSetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	var req roleReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.Role != config.RoleAdmin && req.Role != config.RoleMember {
		writeError(w, http.StatusBadRequest, "BAD_ROLE", "บทบาทต้องเป็น admin หรือ member")
		return
	}
	// กันลดสิทธิ์ตัวเอง ไม่อย่างนั้นระบบอาจไม่เหลือ Admin สักคน
	if id == who(r).ID {
		writeError(w, http.StatusBadRequest, "SELF_ROLE", "เปลี่ยนบทบาทของตัวเองไม่ได้")
		return
	}
	m, err := h.Store.SetRole(r.Context(), id, req.Role)
	if isNotFound(err) {
		notFound(w, "สมาชิกนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
