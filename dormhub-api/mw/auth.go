package mw

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"dormhub/config"
)

// TokenParser คืออะไรก็ได้ที่ตรวจตั๋วได้ (auth.Issuer ทำได้)
// ใช้ interface เพื่อให้เทสต์ส่งตัวปลอมเข้ามาได้ โดยไม่ต้องสร้างตั๋วจริง
type TokenParser interface {
	Parse(token string) (memberID int64, role string, err error)
}

// Who คือ "คนที่ยิงคำขอนี้" ได้มาจากตั๋วที่ตรวจผ่านแล้ว
type Who struct {
	ID   int64
	Role string
}

type ctxKey struct{}

// WhoFrom ให้ handler หยิบคนที่ login อยู่ออกมาจาก context
func WhoFrom(ctx context.Context) (Who, bool) {
	w, ok := ctx.Value(ctxKey{}).(Who)
	return w, ok
}

// WithWho ใส่ Who ลงใน context (ใช้ในเทสต์ของ handler)
func WithWho(ctx context.Context, w Who) context.Context {
	return context.WithValue(ctx, ctxKey{}, w)
}

// RequireAuth ด่านที่ 1: ต้องมีตั๋วที่ถูกต้อง ไม่งั้นตอบ 401
// ใช้ครอบ endpoint ของ Member (และ Admin ก็ผ่านด่านนี้ได้เพราะเป็นสมาชิกเหมือนกัน)
func RequireAuth(tp TokenParser, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(header, "Bearer ")
		if !found || strings.TrimSpace(token) == "" {
			writeErr(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
			return
		}
		id, role, err := tp.Parse(strings.TrimSpace(token))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "BAD_TOKEN", "เซสชันหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		next(w, r.WithContext(WithWho(r.Context(), Who{ID: id, Role: role})))
	})
}

// RequireAdmin ด่านที่ 2: ผ่านด่านที่ 1 แล้ว role ต้องเป็น admin ไม่งั้นตอบ 403
// 401 = ไม่รู้ว่าเป็นใคร  /  403 = รู้ว่าเป็นใคร แต่ไม่มีสิทธิ์
// การซ่อนเมนูในหน้าเว็บกันได้แค่คนทั่วไป คนที่ยิง API ตรง ๆ จะมาติดที่ด่านนี้
func RequireAdmin(tp TokenParser, next http.HandlerFunc) http.Handler {
	return RequireAuth(tp, func(w http.ResponseWriter, r *http.Request) {
		who, _ := WhoFrom(r.Context())
		if who.Role != config.RoleAdmin {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "ส่วนนี้สำหรับผู้ดูแลระบบเท่านั้น")
			return
		}
		next(w, r)
	})
}

// writeErr รูปแบบเดียวกับ handler.writeError: {"error":{"code","message"}}
// เขียนซ้ำไว้ที่นี่เพราะ handler import mw อยู่แล้ว ถ้า mw import handler จะวนกัน
func writeErr(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func logPanic(err any) {
	log.Printf("panic: %v", err)
}
