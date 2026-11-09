package handler

import (
	"strings"
	"unicode/utf8"

	"dormhub/config"
)

// ฟังก์ชันตรวจข้อมูล คืน "" ถ้าผ่าน หรือคืนข้อความ error ภาษาไทย
// แยกออกมาเป็นฟังก์ชันเล็ก ๆ เพื่อเทสต์ได้โดยไม่ต้องมีฐานข้อมูล (ดู validate_test.go)

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func checkEmail(email string) string {
	at := strings.Index(email, "@")
	if at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t") || len(email) > 255 {
		return "รูปแบบอีเมลไม่ถูกต้อง"
	}
	return ""
}

func checkPassword(pw string) string {
	if runeLen(pw) < config.MinPasswordLen || len(pw) > config.MaxPasswordBytes {
		return "รหัสผ่านต้องยาว 8 ตัวอักษรขึ้นไป และไม่เกิน 72 ไบต์"
	}
	return ""
}

func checkName(name string) string {
	if name == "" || runeLen(name) > config.MaxNameLen {
		return "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร"
	}
	return ""
}

// checkPhone อนุญาตตัวเลข ช่องว่าง - และ + เท่านั้น ("" = ไม่กรอก ถือว่าผ่าน)
func checkPhone(phone string, required bool) string {
	if phone == "" {
		if required {
			return "กรุณากรอกเบอร์โทรศัพท์"
		}
		return ""
	}
	digits := 0
	for _, c := range phone {
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == ' ' || c == '-' || c == '+':
		default:
			return "เบอร์โทรศัพท์ใส่ได้เฉพาะตัวเลข"
		}
	}
	if digits < 9 || len(phone) > config.MaxPhoneLen {
		return "เบอร์โทรศัพท์ต้องมีตัวเลข 9-10 หลัก"
	}
	return ""
}

func checkReview(rating int, text string) string {
	if rating < config.MinRating || rating > config.MaxRating {
		return "ให้คะแนนได้ 1 ถึง 5 ดาว"
	}
	n := runeLen(strings.TrimSpace(text))
	if n < config.MinReviewLen {
		return "รีวิวต้องยาวอย่างน้อย 10 ตัวอักษร"
	}
	if n > config.MaxReviewLen {
		return "รีวิวยาวเกิน 1,000 ตัวอักษร"
	}
	return ""
}
