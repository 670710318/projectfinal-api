package mw

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeParser ตั๋วปลอม: "member-token" = สมาชิกเลข 7, "admin-token" = admin เลข 1 นอกนั้นผิดหมด
type fakeParser struct{}

func (fakeParser) Parse(t string) (int64, string, error) {
	switch t {
	case "member-token":
		return 7, "member", nil
	case "admin-token":
		return 1, "admin", nil
	}
	return 0, "", errors.New("bad token")
}

// ok คือ handler ปลายทาง ถ้าถูกเรียกจะตอบ 200 พร้อมเลขสมาชิก
func ok(w http.ResponseWriter, r *http.Request) {
	who, _ := WhoFrom(r.Context())
	w.Header().Set("X-Member", string(rune('0'+who.ID)))
	w.WriteHeader(http.StatusOK)
}

func call(h http.Handler, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequireAuth(t *testing.T) {
	h := RequireAuth(fakeParser{}, ok)
	cases := []struct {
		name   string
		auth   string
		status int
	}{
		{"ไม่มีตั๋ว", "", http.StatusUnauthorized},
		{"ไม่ใช่ Bearer", "Basic abc", http.StatusUnauthorized},
		{"ตั๋วผิด", "Bearer nope", http.StatusUnauthorized},
		{"สมาชิก", "Bearer member-token", http.StatusOK},
		{"admin ก็ผ่านด่านสมาชิก", "Bearer admin-token", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := call(h, c.auth).Code; got != c.status {
				t.Fatalf("ได้ %d ต้องได้ %d", got, c.status)
			}
		})
	}
}

func TestRequireAuthPutsWhoInContext(t *testing.T) {
	rec := call(RequireAuth(fakeParser{}, ok), "Bearer member-token")
	if rec.Header().Get("X-Member") != "7" {
		t.Fatalf("handler ต้องเห็นสมาชิกเลข 7 แต่เห็น %q", rec.Header().Get("X-Member"))
	}
}

func TestRequireAdmin(t *testing.T) {
	h := RequireAdmin(fakeParser{}, ok)
	if got := call(h, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("ไม่มีตั๋วต้องได้ 401 แต่ได้ %d", got)
	}
	if got := call(h, "Bearer member-token").Code; got != http.StatusForbidden {
		t.Fatalf("สมาชิกธรรมดาต้องได้ 403 แต่ได้ %d", got)
	}
	if got := call(h, "Bearer admin-token").Code; got != http.StatusOK {
		t.Fatalf("admin ต้องได้ 200 แต่ได้ %d", got)
	}
}
