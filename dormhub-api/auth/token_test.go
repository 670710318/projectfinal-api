package auth

import (
	"testing"
	"time"
)

func newIssuer(ttl time.Duration) *Issuer {
	return &Issuer{Secret: []byte("test-secret-ที่ยาวพอสำหรับเทสต์-1234567890"), TTL: ttl}
}

func TestIssueThenParse(t *testing.T) {
	i := newIssuer(time.Hour)
	tok, err := i.Issue(42, "admin")
	if err != nil {
		t.Fatal(err)
	}
	id, role, err := i.Parse(tok)
	if err != nil {
		t.Fatalf("ตั๋วที่เพิ่งออกต้องผ่าน แต่ได้ %v", err)
	}
	if id != 42 || role != "admin" {
		t.Fatalf("ได้ id=%d role=%q ต้องได้ 42 admin", id, role)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	tok, _ := newIssuer(time.Hour).Issue(1, "member")
	other := &Issuer{Secret: []byte("another-secret-another-secret-123456"), TTL: time.Hour}
	if _, _, err := other.Parse(tok); err == nil {
		t.Fatal("ตั๋วที่เซ็นด้วยกุญแจอื่นต้องไม่ผ่าน")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	i := newIssuer(-time.Minute) // หมดอายุตั้งแต่ออก
	tok, _ := i.Issue(1, "member")
	if _, _, err := i.Parse(tok); err == nil {
		t.Fatal("ตั๋วหมดอายุต้องไม่ผ่าน")
	}
}

func TestParseRejectsTampered(t *testing.T) {
	i := newIssuer(time.Hour)
	tok, _ := i.Issue(1, "member")
	if _, _, err := i.Parse(tok + "x"); err == nil {
		t.Fatal("ตั๋วที่ถูกแก้ต้องไม่ผ่าน")
	}
}

func TestPassword(t *testing.T) {
	hash, err := HashPassword("dorm12345")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "dorm12345") {
		t.Fatal("รหัสถูกต้องต้องผ่าน")
	}
	if CheckPassword(hash, "dorm12346") {
		t.Fatal("รหัสผิดต้องไม่ผ่าน")
	}
}
