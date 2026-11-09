package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Member struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	Phone        *string   `json:"phone"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	PasswordHash string    `json:"-"` // json:"-" = ไม่มีวันหลุดออกไปในคำตอบ API
}

const memberColumns = `id, email, display_name, phone, role, created_at, password_hash`

func scanMember(row pgx.Row) (Member, error) {
	var m Member
	err := row.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Phone, &m.Role, &m.CreatedAt, &m.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

func (s *Store) MemberByEmail(ctx context.Context, email string) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx,
		`SELECT `+memberColumns+` FROM members WHERE lower(email) = lower($1)`, email))
}

func (s *Store) MemberByID(ctx context.Context, id int64) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx,
		`SELECT `+memberColumns+` FROM members WHERE id = $1`, id))
}

// CreateMember ลำดับพารามิเตอร์: email, displayName, passwordHash
// (lab เดิมเรียกสลับ hash กับชื่อ ทำให้ login ไม่ผ่าน ระวังจุดนี้)
func (s *Store) CreateMember(ctx context.Context, email, displayName, passwordHash string) (Member, error) {
	m, err := scanMember(s.Pool.QueryRow(ctx,
		`INSERT INTO members (email, display_name, password_hash)
		 VALUES (lower($1), $2, $3)
		 RETURNING `+memberColumns,
		email, displayName, passwordHash))
	if pgCode(err) == pgUniqueViolation {
		return Member{}, ErrEmailTaken
	}
	return m, err
}

// UpdateProfile แก้ชื่อที่แสดงและเบอร์โทร (phone ว่าง = ลบเบอร์)
func (s *Store) UpdateProfile(ctx context.Context, id int64, displayName, phone string) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx,
		`UPDATE members SET display_name = $2, phone = NULLIF($3, ''), updated_at = now()
		 WHERE id = $1
		 RETURNING `+memberColumns,
		id, displayName, phone))
}

func (s *Store) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE members SET password_hash = $2, updated_at = now() WHERE id = $1`, id, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- ส่วนของ Admin ----------

type MemberRow struct {
	Member
	ReviewCount  int `json:"reviewCount"`
	BookingCount int `json:"bookingCount"`
}

func (s *Store) ListMembers(ctx context.Context) ([]MemberRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT m.id, m.email, m.display_name, m.phone, m.role, m.created_at,
		        (SELECT count(*) FROM reviews r WHERE r.member_id = m.id),
		        (SELECT count(*) FROM bookings b WHERE b.member_id = m.id)
		 FROM members m
		 ORDER BY m.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemberRow{}
	for rows.Next() {
		var m MemberRow
		if err := rows.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Phone, &m.Role, &m.CreatedAt,
			&m.ReviewCount, &m.BookingCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SetRole(ctx context.Context, id int64, role string) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx,
		`UPDATE members SET role = $2, updated_at = now() WHERE id = $1 RETURNING `+memberColumns,
		id, role))
}
