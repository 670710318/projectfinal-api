package handler

import (
	"net/http"
	"strings"

	"dormhub/auth"
)

// Me GET /api/me  อ่านข้อมูลล่าสุดจากฐานข้อมูล (ไม่เชื่อข้อมูลเก่าในตั๋ว)
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	m, err := h.Store.MemberByID(r.Context(), who(r).ID)
	if isNotFound(err) {
		// ตั๋วถูกต้องแต่บัญชีถูกลบไปแล้ว
		writeError(w, http.StatusUnauthorized, "NO_MEMBER", "ไม่พบบัญชีนี้แล้ว กรุณาเข้าสู่ระบบใหม่")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type profileReq struct {
	DisplayName string `json:"displayName"`
	Phone       string `json:"phone"`
}

// UpdateMe PUT /api/me  แก้ชื่อที่แสดงและเบอร์โทร
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req profileReq
	if !readJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	phone := strings.TrimSpace(req.Phone)
	for _, msg := range []string{checkName(name), checkPhone(phone, false)} {
		if msg != "" {
			writeError(w, http.StatusBadRequest, "VALIDATION", msg)
			return
		}
	}
	m, err := h.Store.UpdateProfile(r.Context(), who(r).ID, name, phone)
	if isNotFound(err) {
		notFound(w, "บัญชีนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type passwordReq struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePassword PUT /api/me/password  ต้องยืนยันรหัสเดิมก่อน กันคนแอบใช้เครื่องที่ login ค้างไว้
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req passwordReq
	if !readJSON(w, r, &req) {
		return
	}
	if msg := checkPassword(req.NewPassword); msg != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "รหัสผ่านใหม่: "+msg)
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "SAME_PASSWORD", "รหัสผ่านใหม่ต้องไม่ซ้ำกับรหัสเดิม")
		return
	}
	m, err := h.Store.MemberByID(r.Context(), who(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	// ตอบ 400 ไม่ใช่ 401: หน้าเว็บถือว่า 401 = ตั๋วหมดอายุ แล้วจะ logout ให้อัตโนมัติ
	if !auth.CheckPassword(m.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "WRONG_PASSWORD", "รหัสผ่านเดิมไม่ถูกต้อง")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		internal(w, err)
		return
	}
	if err := h.Store.UpdatePassword(r.Context(), m.ID, hash); err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
