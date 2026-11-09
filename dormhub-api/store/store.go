// Package store คุยกับฐานข้อมูลอย่างเดียว ไม่รู้เรื่อง HTTP
// ค่าจากผู้ใช้ส่งผ่าน $1 $2 ... แยกจากคำสั่ง SQL เสมอ (กัน SQL Injection)
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrEmailTaken    = errors.New("email taken")
	ErrDuplicate     = errors.New("duplicate")
	ErrBadReference  = errors.New("bad reference")
	ErrInvalidStatus = errors.New("invalid status")
)

type Store struct {
	Pool *pgxpool.Pool
}

// รหัส error ของ PostgreSQL ที่เราแปลงเป็นข้อความเข้าใจง่าย
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// pgCode ดึงรหัส error ของ PostgreSQL ออกมา (ถ้าไม่ใช่ error จาก DB คืน "")
func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// translate แปลง error ของ DB ให้เป็น error ของเรา handler จะได้เลือก status code ถูก
func translate(err error) error {
	switch pgCode(err) {
	case pgUniqueViolation:
		return ErrDuplicate
	case pgForeignKeyViolation:
		return ErrBadReference
	}
	return err
}
