package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------- รูปร่างข้อมูลที่ส่งออกไปเป็น JSON ----------

type Zone struct {
	ID        int     `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Landmark  *string `json:"landmark,omitempty"`
	DormCount int     `json:"dormCount"`
}

type ZoneRef struct {
	ID   int    `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// DormCard ข้อมูลพอสำหรับการ์ด 1 ใบในหน้ารายการ
type DormCard struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Zone        ZoneRef  `json:"zone"`
	Gender      string   `json:"gender"`
	Nearby      *string  `json:"nearby"`
	ImageURL    *string  `json:"imageUrl"`
	Facilities  []string `json:"facilities"`
	PriceMin    *int     `json:"priceMin"` // nil = ยังไม่มีประเภทห้อง
	PriceMax    *int     `json:"priceMax"`
	Available   int      `json:"available"`
	ReviewCount int      `json:"reviewCount"`
	Rating      *float64 `json:"rating"` // nil = ยังไม่มีรีวิว
	IsPublished bool     `json:"isPublished"`
}

type Room struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	SizeSqm    *float64 `json:"sizeSqm"`
	PriceMonth int      `json:"priceMonth"`
	Available  int      `json:"available"`
}

// DormDetail ข้อมูลเต็มสำหรับหน้ารายละเอียด (ฝัง DormCard ไว้ JSON จะออกมาแบน ๆ ชั้นเดียว)
type DormDetail struct {
	DormCard
	Address      string    `json:"address"`
	Description  string    `json:"description"`
	Phone        *string   `json:"phone"`
	LineID       *string   `json:"lineId"`
	WaterRate    *string   `json:"waterRate"`
	ElectricRate *string   `json:"electricRate"`
	Deposit      *string   `json:"deposit"`
	Rooms        []Room    `json:"rooms"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ---------- ตัวกรองของหน้าค้นหา ----------

type DormFilter struct {
	Q             string // คำค้น: ชื่อ / ที่อยู่ / สถานที่ใกล้เคียง
	Zone          string // slug ของย่าน
	Gender        string
	PriceMin      int // งบขั้นต่ำ (0 = ไม่กำหนด)
	PriceMax      int // งบสูงสุด (0 = ไม่กำหนด)
	AvailableOnly bool
	Sort          string
	Limit         int
	IncludeHidden bool // true เฉพาะหน้า Admin
}

// การเรียงที่อนุญาต: ผู้ใช้ส่งได้แค่ "ชื่อ" ของการเรียง ส่วนข้อความ SQL เราเป็นคนเลือกเอง
// (ห้ามเอาค่าจาก URL ไปต่อเป็น ORDER BY ตรง ๆ เพราะส่งผ่าน $1 ไม่ได้)
var dormSorts = map[string]string{
	"recommended": `coalesce(vs.rating, 0) DESC, coalesce(vs.review_count, 0) DESC, d.id`,
	"newest":      `d.created_at DESC, d.id DESC`,
	"price_asc":   `rs.price_min ASC NULLS LAST, d.id`,
	"price_desc":  `rs.price_min DESC NULLS LAST, d.id`,
	"rating":      `vs.rating DESC NULLS LAST, coalesce(vs.review_count, 0) DESC, d.id`,
}

func ValidSort(s string) bool { _, ok := dormSorts[s]; return ok }

const dormCardSelect = `
SELECT d.id, d.name, z.id, z.slug, z.name_th, d.gender, d.nearby, d.image_url, d.facilities,
       rs.price_min, rs.price_max, coalesce(rs.available, 0)::int,
       coalesce(vs.review_count, 0)::int, vs.rating::float8, d.is_published
FROM dorms d
JOIN zones z ON z.id = d.zone_id
LEFT JOIN dorm_room_stats   rs ON rs.dorm_id = d.id
LEFT JOIN dorm_review_stats vs ON vs.dorm_id = d.id`

func scanCard(row pgx.Row, c *DormCard) error {
	return row.Scan(&c.ID, &c.Name, &c.Zone.ID, &c.Zone.Slug, &c.Zone.Name, &c.Gender, &c.Nearby,
		&c.ImageURL, &c.Facilities, &c.PriceMin, &c.PriceMax, &c.Available,
		&c.ReviewCount, &c.Rating, &c.IsPublished)
}

func (s *Store) ListDorms(ctx context.Context, f DormFilter) ([]DormCard, error) {
	order, ok := dormSorts[f.Sort]
	if !ok {
		order = dormSorts["recommended"]
	}
	q := strings.ToLower(strings.TrimSpace(f.Q))
	rows, err := s.Pool.Query(ctx, dormCardSelect+`
		WHERE ($1::bool OR d.is_published)
		  AND ($2::text = '' OR lower(d.name) LIKE '%' || $2 || '%'
		                     OR lower(d.address) LIKE '%' || $2 || '%'
		                     OR lower(coalesce(d.nearby, '')) LIKE '%' || $2 || '%')
		  AND ($3::text = '' OR z.slug = $3)
		  AND ($4::text = '' OR d.gender = $4)
		  AND ($5::int = 0 OR rs.price_max >= $5)
		  AND ($6::int = 0 OR rs.price_min <= $6)
		  AND (NOT $7::bool OR rs.available > 0)
		ORDER BY `+order+`
		LIMIT $8`,
		f.IncludeHidden, q, f.Zone, f.Gender, f.PriceMin, f.PriceMax, f.AvailableOnly, f.Limit)
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

// GetDorm includeHidden=false สำหรับ Guest/Member (หอที่ยังไม่เผยแพร่จะตอบว่าไม่พบ)
func (s *Store) GetDorm(ctx context.Context, id int64, includeHidden bool) (DormDetail, error) {
	var d DormDetail
	row := s.Pool.QueryRow(ctx, `
		SELECT d.id, d.name, z.id, z.slug, z.name_th, d.gender, d.nearby, d.image_url, d.facilities,
		       rs.price_min, rs.price_max, coalesce(rs.available, 0)::int,
		       coalesce(vs.review_count, 0)::int, vs.rating::float8, d.is_published,
		       d.address, d.description, d.phone, d.line_id, d.water_rate, d.electric_rate, d.deposit,
		       d.updated_at
		FROM dorms d
		JOIN zones z ON z.id = d.zone_id
		LEFT JOIN dorm_room_stats   rs ON rs.dorm_id = d.id
		LEFT JOIN dorm_review_stats vs ON vs.dorm_id = d.id
		WHERE d.id = $1 AND ($2::bool OR d.is_published)`, id, includeHidden)
	c := &d.DormCard
	err := row.Scan(&c.ID, &c.Name, &c.Zone.ID, &c.Zone.Slug, &c.Zone.Name, &c.Gender, &c.Nearby,
		&c.ImageURL, &c.Facilities, &c.PriceMin, &c.PriceMax, &c.Available,
		&c.ReviewCount, &c.Rating, &c.IsPublished,
		&d.Address, &d.Description, &d.Phone, &d.LineID, &d.WaterRate, &d.ElectricRate, &d.Deposit,
		&d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DormDetail{}, ErrNotFound
	}
	if err != nil {
		return DormDetail{}, err
	}
	d.Rooms, err = s.listRooms(ctx, id)
	return d, err
}

func (s *Store) listRooms(ctx context.Context, dormID int64) ([]Room, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, size_sqm::float8, price_month, available
		 FROM rooms WHERE dorm_id = $1 ORDER BY position, id`, dormID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Room{}
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Name, &r.SizeSqm, &r.PriceMonth, &r.Available); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RoomBelongsToDorm ใช้ตอนจองว่า room ที่ส่งมาเป็นของหอนี้จริงไหม
func (s *Store) RoomBelongsToDorm(ctx context.Context, roomID, dormID int64) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM rooms WHERE id = $1 AND dorm_id = $2)`, roomID, dormID).Scan(&ok)
	return ok, err
}

func (s *Store) ListZones(ctx context.Context) ([]Zone, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT z.id, z.slug, z.name_th, z.landmark,
		        (SELECT count(*) FROM dorms d WHERE d.zone_id = z.id AND d.is_published)::int
		 FROM zones z ORDER BY z.position, z.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Zone{}
	for rows.Next() {
		var z Zone
		if err := rows.Scan(&z.ID, &z.Slug, &z.Name, &z.Landmark, &z.DormCount); err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

// ---------- เขียนข้อมูล (Admin) ----------

type RoomInput struct {
	ID         int64    `json:"id"` // 0 = ห้องใหม่
	Name       string   `json:"name"`
	SizeSqm    *float64 `json:"sizeSqm"`
	PriceMonth int      `json:"priceMonth"`
	Available  int      `json:"available"`
}

type DormInput struct {
	Name         string      `json:"name"`
	ZoneID       int         `json:"zoneId"`
	Gender       string      `json:"gender"`
	Address      string      `json:"address"`
	Nearby       string      `json:"nearby"`
	Description  string      `json:"description"`
	Facilities   []string    `json:"facilities"`
	ImageURL     string      `json:"imageUrl"`
	Phone        string      `json:"phone"`
	LineID       string      `json:"lineId"`
	WaterRate    string      `json:"waterRate"`
	ElectricRate string      `json:"electricRate"`
	Deposit      string      `json:"deposit"`
	IsPublished  bool        `json:"isPublished"`
	Rooms        []RoomInput `json:"rooms"`
}

// CreateDorm เพิ่มหอ + ประเภทห้องใน transaction เดียว พังกลางทางจะไม่มีอะไรค้างในตาราง
func (s *Store) CreateDorm(ctx context.Context, in DormInput) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) // ถ้า Commit แล้ว Rollback จะไม่ทำอะไร

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO dorms (name, zone_id, gender, address, nearby, description, facilities, image_url,
		                   phone, line_id, water_rate, electric_rate, deposit, is_published)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, ''),
		        NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''), $14)
		RETURNING id`,
		in.Name, in.ZoneID, in.Gender, in.Address, in.Nearby, in.Description, in.Facilities, in.ImageURL,
		in.Phone, in.LineID, in.WaterRate, in.ElectricRate, in.Deposit, in.IsPublished).Scan(&id)
	if err != nil {
		return 0, translate(err)
	}
	if err := saveRooms(ctx, tx, id, in.Rooms); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) UpdateDorm(ctx context.Context, id int64, in DormInput) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE dorms SET name = $2, zone_id = $3, gender = $4, address = $5, nearby = NULLIF($6, ''),
		       description = $7, facilities = $8, image_url = NULLIF($9, ''), phone = NULLIF($10, ''),
		       line_id = NULLIF($11, ''), water_rate = NULLIF($12, ''), electric_rate = NULLIF($13, ''),
		       deposit = NULLIF($14, ''), is_published = $15, updated_at = now()
		WHERE id = $1`,
		id, in.Name, in.ZoneID, in.Gender, in.Address, in.Nearby, in.Description, in.Facilities,
		in.ImageURL, in.Phone, in.LineID, in.WaterRate, in.ElectricRate, in.Deposit, in.IsPublished)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := saveRooms(ctx, tx, id, in.Rooms); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// saveRooms ทำให้ห้องในฐานข้อมูลตรงกับรายการที่ส่งมา:
// ห้องที่มี id = แก้ไข, ไม่มี id = เพิ่มใหม่, ห้องเดิมที่ไม่อยู่ในรายการ = ลบ
// (ใช้วิธีนี้แทนการลบทิ้งทั้งหมดแล้วใส่ใหม่ เพราะคำขอนัดดูห้องอ้างถึง room id อยู่)
func saveRooms(ctx context.Context, tx pgx.Tx, dormID int64, rooms []RoomInput) error {
	keep := []int64{} // ต้องเป็น slice ว่าง ไม่ใช่ nil เพราะ nil จะกลายเป็น NULL แล้ว ANY(NULL) ไม่ลบอะไรเลย
	for _, r := range rooms {
		if r.ID > 0 {
			keep = append(keep, r.ID)
		}
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM rooms WHERE dorm_id = $1 AND NOT (id = ANY($2))`, dormID, keep); err != nil {
		return err
	}
	for i, r := range rooms {
		if r.ID > 0 {
			tag, err := tx.Exec(ctx,
				`UPDATE rooms SET name = $3, size_sqm = $4, price_month = $5, available = $6, position = $7
				 WHERE id = $1 AND dorm_id = $2`,
				r.ID, dormID, r.Name, r.SizeSqm, r.PriceMonth, r.Available, i)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrBadReference // ส่ง id ห้องของหออื่นมา
			}
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO rooms (dorm_id, name, size_sqm, price_month, available, position)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			dormID, r.Name, r.SizeSqm, r.PriceMonth, r.Available, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteDorm(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM dorms WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- ภาพรวมสำหรับหน้า Admin ----------

type Stats struct {
	Dorms           int `json:"dorms"`
	PublishedDorms  int `json:"publishedDorms"`
	Members         int `json:"members"`
	Reviews         int `json:"reviews"`
	PendingBookings int `json:"pendingBookings"`
	TotalBookings   int `json:"totalBookings"`
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM dorms)::int,
		       (SELECT count(*) FROM dorms WHERE is_published)::int,
		       (SELECT count(*) FROM members)::int,
		       (SELECT count(*) FROM reviews)::int,
		       (SELECT count(*) FROM bookings WHERE status = 'pending')::int,
		       (SELECT count(*) FROM bookings)::int`).
		Scan(&st.Dorms, &st.PublishedDorms, &st.Members, &st.Reviews, &st.PendingBookings, &st.TotalBookings)
	return st, err
}
