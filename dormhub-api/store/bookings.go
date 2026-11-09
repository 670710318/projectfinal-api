package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"dormhub/config"
)

// Booking คำขอนัดดูห้อง 1 รายการ
type Booking struct {
	ID           int64         `json:"id"`
	DormID       int64         `json:"dormId"`
	DormName     string        `json:"dormName"`
	RoomID       *int64        `json:"roomId"`
	RoomName     *string       `json:"roomName"`
	VisitDate    string        `json:"visitDate"` // YYYY-MM-DD
	ContactPhone string        `json:"contactPhone"`
	Note         *string       `json:"note"`
	Status       string        `json:"status"`
	AdminNote    *string       `json:"adminNote"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	Member       *ReviewAuthor `json:"member,omitempty"` // ส่งเฉพาะหน้า Admin
	MemberEmail  *string       `json:"memberEmail,omitempty"`
}

type BookingInput struct {
	RoomID       *int64 `json:"roomId"`
	VisitDate    string `json:"visitDate"`
	ContactPhone string `json:"contactPhone"`
	Note         string `json:"note"`
}

const bookingSelect = `
SELECT b.id, b.dorm_id, d.name, b.room_id, r.name, to_char(b.visit_date, 'YYYY-MM-DD'),
       b.contact_phone, b.note, b.status, b.admin_note, b.created_at, b.updated_at,
       m.id, m.display_name, m.email
FROM bookings b
JOIN dorms d   ON d.id = b.dorm_id
JOIN members m ON m.id = b.member_id
LEFT JOIN rooms r ON r.id = b.room_id`

func scanBooking(row pgx.Row, withMember bool) (Booking, error) {
	var b Booking
	var author ReviewAuthor
	var email string
	err := row.Scan(&b.ID, &b.DormID, &b.DormName, &b.RoomID, &b.RoomName, &b.VisitDate,
		&b.ContactPhone, &b.Note, &b.Status, &b.AdminNote, &b.CreatedAt, &b.UpdatedAt,
		&author.ID, &author.DisplayName, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if withMember {
		b.Member = &author
		b.MemberEmail = &email
	}
	return b, nil
}

func collectBookings(rows pgx.Rows, withMember bool) ([]Booking, error) {
	defer rows.Close()
	out := []Booking{}
	for rows.Next() {
		b, err := scanBooking(rows, withMember)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CreateBooking มีคำขอรออยู่แล้ว -> ErrDuplicate, หอ/ห้องไม่มีจริง -> ErrBadReference
func (s *Store) CreateBooking(ctx context.Context, memberID, dormID int64, in BookingInput) (Booking, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO bookings (member_id, dorm_id, room_id, visit_date, contact_phone, note)
		 VALUES ($1, $2, $3, $4::date, $5, NULLIF($6, ''))
		 RETURNING id`,
		memberID, dormID, in.RoomID, in.VisitDate, in.ContactPhone, in.Note).Scan(&id)
	if err != nil {
		return Booking{}, translate(err)
	}
	return scanBooking(s.Pool.QueryRow(ctx, bookingSelect+` WHERE b.id = $1`, id), false)
}

func (s *Store) ListMyBookings(ctx context.Context, memberID int64) ([]Booking, error) {
	rows, err := s.Pool.Query(ctx, bookingSelect+`
		WHERE b.member_id = $1 ORDER BY b.created_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	return collectBookings(rows, false)
}

// CancelBooking ยกเลิกได้เฉพาะคำขอของตัวเองที่ยัง pending
func (s *Store) CancelBooking(ctx context.Context, memberID, bookingID int64) (Booking, error) {
	return s.changeStatus(ctx, bookingID, memberID, config.BookingCancelled, "")
}

// ---------- ส่วนของ Admin ----------

func (s *Store) ListBookings(ctx context.Context, status string) ([]Booking, error) {
	rows, err := s.Pool.Query(ctx, bookingSelect+`
		WHERE ($1::text = '' OR b.status = $1)
		ORDER BY (b.status = 'pending') DESC, b.visit_date, b.created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	return collectBookings(rows, true)
}

// DecideBooking Admin อนุมัติหรือปฏิเสธ (เปลี่ยนได้เฉพาะคำขอที่ยัง pending)
func (s *Store) DecideBooking(ctx context.Context, bookingID int64, status, adminNote string) (Booking, error) {
	return s.changeStatus(ctx, bookingID, 0, status, adminNote)
}

// changeStatus ownerID = 0 คือ Admin (เปลี่ยนของใครก็ได้)
// คืน ErrNotFound ถ้าไม่มีคำขอนี้ (หรือไม่ใช่ของเรา), ErrInvalidStatus ถ้าไม่ได้อยู่ในสถานะ pending แล้ว
func (s *Store) changeStatus(ctx context.Context, bookingID, ownerID int64, status, adminNote string) (Booking, error) {
	var current string
	err := s.Pool.QueryRow(ctx,
		`SELECT status FROM bookings WHERE id = $1 AND ($2::bigint = 0 OR member_id = $2)`,
		bookingID, ownerID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	// AND status = 'pending' ในคำสั่ง UPDATE กันกรณีสองคนกดพร้อมกัน
	tag, err := s.Pool.Exec(ctx,
		`UPDATE bookings SET status = $2, admin_note = coalesce(NULLIF($3, ''), admin_note), updated_at = now()
		 WHERE id = $1 AND status = 'pending'`, bookingID, status, adminNote)
	if err != nil {
		return Booking{}, err
	}
	if tag.RowsAffected() == 0 {
		return Booking{}, ErrInvalidStatus
	}
	return scanBooking(s.Pool.QueryRow(ctx, bookingSelect+` WHERE b.id = $1`, bookingID), ownerID == 0)
}
