// Package config รวมค่าคงที่ทั้งหมดของระบบไว้ที่เดียว และอ่านค่าจาก Environment Variable
// เวลาจะปรับกติกา (เช่น รหัสผ่านยาวขั้นต่ำ) แก้ที่ไฟล์นี้ไฟล์เดียว ไม่ต้องไล่หาในโค้ด
package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

// ---------- กติกาของระบบ ----------
const (
	TokenTTL         = 24 * time.Hour // ตั๋ว JWT ใช้ได้ 1 วัน แล้วต้อง login ใหม่
	MinSecretLen     = 32             // JWT_SECRET สั้นกว่านี้เดาง่าย ไม่ยอมเปิดเซิร์ฟเวอร์
	MinPasswordLen   = 8              // นับเป็นตัวอักษร (rune)
	MaxPasswordBytes = 72             // เพดานของ bcrypt
	MaxNameLen       = 50
	MaxPhoneLen      = 20
	MinReviewLen     = 10
	MaxReviewLen     = 1000
	MinRating        = 1
	MaxRating        = 5
	MaxNoteLen       = 300
	MaxBookingDays   = 90      // นัดดูห้องล่วงหน้าได้ไม่เกิน 90 วัน
	MaxBodyBytes     = 1 << 20 // กล่อง JSON ใหญ่สุด 1 MB
	DefaultPageSize  = 60
	MaxPageSize      = 100
	DateLayout       = "2006-01-02"
)

// Bangkok ใช้ตัดสินว่า "วันนี้" คือวันไหน (ไม่พึ่ง tzdata บนเครื่อง Render)
var Bangkok = time.FixedZone("ICT", 7*60*60)

// บทบาทของสมาชิก ต้องตรงกับ CHECK ในตาราง members
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// สถานะคำขอนัดดูห้อง ต้องตรงกับ CHECK ในตาราง bookings
const (
	BookingPending   = "pending"
	BookingApproved  = "approved"
	BookingRejected  = "rejected"
	BookingCancelled = "cancelled"
)

// ประเภทหอ ต้องตรงกับ CHECK ในตาราง dorms
var Genders = map[string]bool{"mixed": true, "female": true, "male": true}

// Config คือค่าที่อ่านจาก Environment Variable ตอนเปิดเซิร์ฟเวอร์
type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AllowedOrigins []string
}

// Load อ่านค่าจาก env ถ้าขาดตัวที่จำเป็นจะคืน error ให้ main หยุดทันที
func Load() (Config, error) {
	c := Config{
		Port:        os.Getenv("PORT"), // Render ส่งพอร์ตมาให้ทางตัวแปรนี้
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if c.Port == "" {
		c.Port = "8080" // ตอนรันในเครื่อง
	}
	if c.DatabaseURL == "" {
		return c, errors.New("ต้องตั้ง DATABASE_URL (connection string ของ Supabase)")
	}
	if len(c.JWTSecret) < MinSecretLen {
		return c, errors.New("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร")
	}
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.TrimSuffix(o, "/"))
		}
	}
	return c, nil
}
