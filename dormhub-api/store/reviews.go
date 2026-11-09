package store

import (
	"context"
	"time"
)

type ReviewAuthor struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
}

type Review struct {
	ID        int64        `json:"id"`
	DormID    int64        `json:"dormId"`
	Rating    int          `json:"rating"`
	Text      string       `json:"text"`
	CreatedAt time.Time    `json:"createdAt"`
	Member    ReviewAuthor `json:"member"`
}

func (s *Store) ListReviews(ctx context.Context, dormID int64) ([]Review, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT r.id, r.dorm_id, r.rating, r.text, r.created_at, m.id, m.display_name
		 FROM reviews r
		 JOIN members m ON m.id = r.member_id
		 WHERE r.dorm_id = $1
		 ORDER BY r.created_at DESC`, dormID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.DormID, &r.Rating, &r.Text, &r.CreatedAt,
			&r.Member.ID, &r.Member.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateReview รีวิวซ้ำ -> ErrDuplicate (409), หอไม่มีจริง -> ErrBadReference (404)
func (s *Store) CreateReview(ctx context.Context, memberID, dormID int64, rating int, text string) (Review, error) {
	var r Review
	err := s.Pool.QueryRow(ctx,
		`WITH ins AS (
		   INSERT INTO reviews (member_id, dorm_id, rating, text) VALUES ($1, $2, $3, $4)
		   RETURNING id, dorm_id, rating, text, created_at, member_id
		 )
		 SELECT ins.id, ins.dorm_id, ins.rating, ins.text, ins.created_at, m.id, m.display_name
		 FROM ins JOIN members m ON m.id = ins.member_id`,
		memberID, dormID, rating, text).
		Scan(&r.ID, &r.DormID, &r.Rating, &r.Text, &r.CreatedAt, &r.Member.ID, &r.Member.DisplayName)
	if err != nil {
		return Review{}, translate(err)
	}
	return r, nil
}

// DeleteReview ownerID = 0 แปลว่าลบได้ทุกอัน (Admin), ไม่ใช่ 0 = ลบได้เฉพาะของตัวเอง
// เงื่อนไขสิทธิ์อยู่ใน WHERE เลย คนอื่นส่ง id รีวิวที่ไม่ใช่ของตัวเองมาจะได้ ErrNotFound
func (s *Store) DeleteReview(ctx context.Context, reviewID, ownerID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM reviews WHERE id = $1 AND ($2::bigint = 0 OR member_id = $2)`, reviewID, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- รายการโปรด ----------

func (s *Store) ListFavorites(ctx context.Context, memberID int64) ([]DormCard, error) {
	rows, err := s.Pool.Query(ctx, dormCardSelect+`
		JOIN favorites f ON f.dorm_id = d.id
		WHERE f.member_id = $1 AND d.is_published
		ORDER BY f.added_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DormCard{}
	for rows.Next() {
		var c DormCard
		if err := scanCard(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddFavorite กดซ้ำได้ไม่ error (ON CONFLICT DO NOTHING) หอไม่มีจริง -> ErrBadReference
func (s *Store) AddFavorite(ctx context.Context, memberID, dormID int64) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO favorites (member_id, dorm_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		memberID, dormID)
	return translate(err)
}

func (s *Store) RemoveFavorite(ctx context.Context, memberID, dormID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM favorites WHERE member_id = $1 AND dorm_id = $2`, memberID, dormID)
	return err
}
