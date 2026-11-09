// Package handler แปลงคำขอ HTTP เป็นการเรียก store แล้วตอบกลับเป็น JSON
// หน้าที่ของชั้นนี้: อ่าน/ตรวจข้อมูลที่ส่งมา -> เรียก store -> เลือก status code
package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"dormhub/auth"
	"dormhub/config"
	"dormhub/mw"
	"dormhub/store"
)

type Handler struct {
	Store   *store.Store
	Tickets *auth.Issuer
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError รูปแบบ error เดียวทั้งระบบ: {"error":{"code":"...","message":"..."}}
// หน้าเว็บแสดง message ให้ผู้ใช้อ่าน ส่วน code ไว้ให้โปรแกรมเช็ก
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง (ไม่เผยโครงสร้าง DB)
func internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

// readJSON อ่านกล่อง JSON จำกัดขนาด และไม่ยอมรับ field ที่ไม่รู้จัก (พิมพ์ชื่อผิดจะรู้ทันที)
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, config.MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "ข้อมูลที่ส่งมาไม่ถูกรูปแบบ")
		return false
	}
	return true
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// badID ใช้คู่กับ pathID
func badID(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "BAD_ID", "รหัสในที่อยู่ต้องเป็นตัวเลข")
}

// who ดึงคนที่ login อยู่ (มีเสมอ เพราะ endpoint ที่เรียกฟังก์ชันนี้ครอบด้วย RequireAuth แล้ว)
func who(r *http.Request) mw.Who {
	w, _ := mw.WhoFrom(r.Context())
	return w
}

func notFound(w http.ResponseWriter, what string) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "ไม่พบ"+what)
}

// Health ให้ Render ใช้เช็กว่าเซิร์ฟเวอร์ยังอยู่ และใช้ปลุกเซิร์ฟเวอร์ก่อนสาธิต
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.Pool.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "DB_DOWN", "ต่อฐานข้อมูลไม่ได้")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// isNotFound ช่วยให้โค้ดสั้นลงตอนเช็ก error จาก store
func isNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
