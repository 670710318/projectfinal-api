package handler

import (
	"net/url"
	"testing"
	"time"

	"dormhub/config"
	"dormhub/store"
)

func TestCheckEmail(t *testing.T) {
	for _, ok := range []string{"a@b.co", "nong.dorm@gmail.com"} {
		if msg := checkEmail(ok); msg != "" {
			t.Errorf("%q ควรผ่าน แต่ได้ %q", ok, msg)
		}
	}
	for _, bad := range []string{"", "abc", "@b.co", "a@", "a b@c.d"} {
		if checkEmail(bad) == "" {
			t.Errorf("%q ไม่ควรผ่าน", bad)
		}
	}
}

func TestCheckPasswordCountsThaiAsCharacters(t *testing.T) {
	if checkPassword("1234567") == "" {
		t.Error("7 ตัวต้องไม่ผ่าน")
	}
	if msg := checkPassword("12345678"); msg != "" {
		t.Errorf("8 ตัวต้องผ่าน แต่ได้ %q", msg)
	}
	if msg := checkPassword("รหัสผ่านไทย"); msg != "" { // 11 ตัวอักษร แต่ 33 ไบต์
		t.Errorf("ภาษาไทย 11 ตัวต้องผ่าน แต่ได้ %q", msg)
	}
}

func TestCheckPhone(t *testing.T) {
	if checkPhone("", false) != "" {
		t.Error("ไม่บังคับ + ว่าง ต้องผ่าน")
	}
	if checkPhone("", true) == "" {
		t.Error("บังคับ + ว่าง ต้องไม่ผ่าน")
	}
	if msg := checkPhone("081-234-5678", true); msg != "" {
		t.Errorf("เบอร์ปกติต้องผ่าน แต่ได้ %q", msg)
	}
	if checkPhone("08x1234567", true) == "" {
		t.Error("มีตัวอักษรต้องไม่ผ่าน")
	}
}

func TestCheckReview(t *testing.T) {
	if checkReview(0, "หอสะอาดมาก เจ้าของใจดี") == "" {
		t.Error("0 ดาวต้องไม่ผ่าน")
	}
	if checkReview(6, "หอสะอาดมาก เจ้าของใจดี") == "" {
		t.Error("6 ดาวต้องไม่ผ่าน")
	}
	if checkReview(5, "สั้นไป") == "" {
		t.Error("ข้อความสั้นกว่า 10 ตัวต้องไม่ผ่าน")
	}
	if msg := checkReview(4, "หอสะอาดมาก เจ้าของใจดี"); msg != "" {
		t.Errorf("รีวิวปกติต้องผ่าน แต่ได้ %q", msg)
	}
}

func TestCheckVisitDate(t *testing.T) {
	// 8 พ.ย. 2026 เวลา 23:30 ที่ไทย (= 16:30 UTC) วันนี้ของไทยคือวันที่ 8
	now := time.Date(2026, 11, 8, 16, 30, 0, 0, time.UTC)
	cases := map[string]bool{
		"2026-11-08": true,  // วันนี้
		"2026-11-07": false, // เมื่อวาน
		"2027-02-06": true,  // +90 วัน
		"2027-02-07": false, // +91 วัน
		"8/11/2026":  false, // รูปแบบผิด
	}
	for in, want := range cases {
		got := checkVisitDate(in, now) == ""
		if got != want {
			t.Errorf("%s: ผ่าน=%v ต้องเป็น %v", in, got, want)
		}
	}
}

func TestParseFilter(t *testing.T) {
	f, msg := parseFilter(url.Values{"zone": {"salaya"}, "priceMax": {"5000"}, "available": {"1"}, "limit": {"500"}})
	if msg != "" {
		t.Fatal(msg)
	}
	if f.Zone != "salaya" || f.PriceMax != 5000 || !f.AvailableOnly || f.Limit != config.MaxPageSize {
		t.Fatalf("อ่านตัวกรองผิด: %+v", f)
	}
	if _, msg := parseFilter(url.Values{"sort": {"id; DROP TABLE dorms"}}); msg == "" {
		t.Error("sort แปลก ๆ ต้องถูกปฏิเสธ")
	}
	if _, msg := parseFilter(url.Values{"priceMin": {"abc"}}); msg == "" {
		t.Error("ราคาที่ไม่ใช่ตัวเลขต้องถูกปฏิเสธ")
	}
}

func TestCleanDorm(t *testing.T) {
	in := store.DormInput{
		Name: "  หอทดสอบ  ", ZoneID: 1, Gender: "mixed", Address: "นครปฐม",
		Facilities: []string{"แอร์", " แอร์ ", "", "Wi-Fi"},
		Rooms:      []store.RoomInput{{Name: "ห้องแอร์", PriceMonth: 3500, Available: 2}},
	}
	if msg := cleanDorm(&in); msg != "" {
		t.Fatal(msg)
	}
	if in.Name != "หอทดสอบ" {
		t.Errorf("ต้องตัดช่องว่าง ได้ %q", in.Name)
	}
	if len(in.Facilities) != 2 {
		t.Errorf("ต้องตัดตัวซ้ำและค่าว่าง ได้ %v", in.Facilities)
	}

	bad := in
	bad.Rooms = []store.RoomInput{{Name: "ห้อง", PriceMonth: 0}}
	if cleanDorm(&bad) == "" {
		t.Error("ราคา 0 ต้องไม่ผ่าน")
	}
	bad = in
	bad.ImageURL = "javascript:alert(1)"
	if cleanDorm(&bad) == "" {
		t.Error("ลิงก์รูปที่ไม่ใช่ https ต้องไม่ผ่าน")
	}
}
