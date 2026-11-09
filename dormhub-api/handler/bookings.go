package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"dormhub/config"
	"dormhub/store"
)

// checkVisitDate วันนัดต้องเป็นวันนี้ถึงอีก 90 วันข้างหน้า (นับตามเวลาไทย)
// รับ now เข้ามาแทนการเรียก time.Now() ข้างใน เพื่อให้เทสต์กำหนดวันได้
func checkVisitDate(s string, now time.Time) string {
	d, err := time.ParseInLocation(config.DateLayout, s, config.Bangkok)
	if err != nil {
		return "วันที่ต้องอยู่ในรูปแบบ YYYY-MM-DD"
	}
	today := now.In(config.Bangkok)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, config.Bangkok)
	if d.Before(today) {
		return "เลือกวันที่ผ่านมาแล้วไม่ได้"
	}
	if d.After(today.AddDate(0, 0, config.MaxBookingDays)) {
		return "นัดล่วงหน้าได้ไม่เกิน 90 วัน"
	}
	return ""
}

// CreateBooking POST /api/dorms/{id}/bookings  (Member ขอนัดดูห้อง)
func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	dormID, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	var in store.BookingInput
	if !readJSON(w, r, &in) {
		return
	}
	in.ContactPhone = strings.TrimSpace(in.ContactPhone)
	in.Note = strings.TrimSpace(in.Note)
	for _, msg := range []string{checkVisitDate(in.VisitDate, time.Now()), checkPhone(in.ContactPhone, true)} {
		if msg != "" {
			writeError(w, http.StatusBadRequest, "VALIDATION", msg)
			return
		}
	}
	if runeLen(in.Note) > config.MaxNoteLen {
		writeError(w, http.StatusBadRequest, "VALIDATION", "ข้อความถึงหอพักยาวเกิน 300 ตัวอักษร")
		return
	}
	// หอต้องเผยแพร่อยู่ และถ้าเลือกประเภทห้อง ห้องนั้นต้องเป็นของหอนี้
	if _, err := h.Store.GetDorm(r.Context(), dormID, false); err != nil {
		if isNotFound(err) {
			notFound(w, "หอพักนี้")
			return
		}
		internal(w, err)
		return
	}
	if in.RoomID != nil {
		belongs, err := h.Store.RoomBelongsToDorm(r.Context(), *in.RoomID, dormID)
		if err != nil {
			internal(w, err)
			return
		}
		if !belongs {
			writeError(w, http.StatusBadRequest, "BAD_ROOM", "ประเภทห้องที่เลือกไม่ใช่ของหอนี้")
			return
		}
	}
	b, err := h.Store.CreateBooking(r.Context(), who(r).ID, dormID, in)
	switch {
	case errors.Is(err, store.ErrDuplicate):
		writeError(w, http.StatusConflict, "PENDING_EXISTS", "คุณมีคำขอนัดดูห้องของหอนี้ที่รอการยืนยันอยู่แล้ว")
		return
	case errors.Is(err, store.ErrBadReference):
		notFound(w, "หอพักหรือห้องนี้")
		return
	case err != nil:
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// ListMyBookings GET /api/me/bookings
func (h *Handler) ListMyBookings(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListMyBookings(r.Context(), who(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// CancelMyBooking POST /api/me/bookings/{id}/cancel
func (h *Handler) CancelMyBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	b, err := h.Store.CancelBooking(r.Context(), who(r).ID, id)
	h.respondStatusChange(w, b, err)
}

// ---------- Admin ----------

var adminBookingFilters = map[string]bool{
	"": true, config.BookingPending: true, config.BookingApproved: true,
	config.BookingRejected: true, config.BookingCancelled: true,
}

// AdminListBookings GET /api/admin/bookings?status=pending
func (h *Handler) AdminListBookings(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if !adminBookingFilters[status] {
		writeError(w, http.StatusBadRequest, "BAD_STATUS", "สถานะไม่ถูกต้อง")
		return
	}
	items, err := h.Store.ListBookings(r.Context(), status)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type decideReq struct {
	Status    string `json:"status"`
	AdminNote string `json:"adminNote"`
}

// AdminDecideBooking PUT /api/admin/bookings/{id}  body: {"status":"approved"|"rejected","adminNote":"..."}
func (h *Handler) AdminDecideBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	var req decideReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.Status != config.BookingApproved && req.Status != config.BookingRejected {
		writeError(w, http.StatusBadRequest, "BAD_STATUS", "เลือกได้แค่ อนุมัติ หรือ ปฏิเสธ")
		return
	}
	note := strings.TrimSpace(req.AdminNote)
	if runeLen(note) > config.MaxNoteLen {
		writeError(w, http.StatusBadRequest, "VALIDATION", "หมายเหตุยาวเกิน 300 ตัวอักษร")
		return
	}
	b, err := h.Store.DecideBooking(r.Context(), id, req.Status, note)
	h.respondStatusChange(w, b, err)
}

func (h *Handler) respondStatusChange(w http.ResponseWriter, b store.Booking, err error) {
	switch {
	case isNotFound(err):
		notFound(w, "คำขอนี้")
	case errors.Is(err, store.ErrInvalidStatus):
		writeError(w, http.StatusConflict, "NOT_PENDING", "คำขอนี้ไม่ได้อยู่ในสถานะรอยืนยันแล้ว")
	case err != nil:
		internal(w, err)
	default:
		writeJSON(w, http.StatusOK, b)
	}
}
