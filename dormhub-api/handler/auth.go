package handler

import (
	"errors"
	"net/http"
	"strings"

	"dormhub/auth"
	"dormhub/store"
)

type registerReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register POST /api/auth/register  สมัครสำเร็จตอบ 201 พร้อมตั๋ว (หน้าเว็บจะได้ login ต่อเลย)
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !readJSON(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.DisplayName)
	for _, msg := range []string{checkEmail(email), checkPassword(req.Password), checkName(name)} {
		if msg != "" {
			writeError(w, http.StatusBadRequest, "VALIDATION", msg)
			return
		}
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internal(w, err)
		return
	}
	m, err := h.Store.CreateMember(r.Context(), email, name, hash) // ลำดับ: email, ชื่อ, hash
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	h.sendTicket(w, http.StatusCreated, m)
}

// Login POST /api/auth/login
// ไม่มีอีเมลนี้ กับ รหัสผิด ตอบข้อความเดียวกัน เพื่อไม่ให้คนนอกเดาได้ว่าอีเมลไหนมีบัญชี
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !readJSON(w, r, &req) {
		return
	}
	m, err := h.Store.MemberByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if isNotFound(err) || (err == nil && !auth.CheckPassword(m.PasswordHash, req.Password)) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	h.sendTicket(w, http.StatusOK, m)
}

// Logout ไม่มี endpoint: ตั๋ว JWT ไม่ได้เก็บไว้ฝั่งเซิร์ฟเวอร์
// หน้าเว็บ "ออกจากระบบ" ด้วยการลบตั๋วออกจาก localStorage แล้วตั๋วก็จะหมดอายุเองตาม TokenTTL

func (h *Handler) sendTicket(w http.ResponseWriter, status int, m store.Member) {
	token, err := h.Tickets.Issue(m.ID, m.Role)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, status, map[string]any{"token": token, "member": m})
}
