-- DormHub นครปฐม: database schema (PostgreSQL 14+ / Supabase)
-- วิธีใช้: Supabase > SQL Editor > วางทั้งไฟล์นี้แล้วกด Run จากนั้นรัน seed.sql ต่อ
-- รันซ้ำได้: บรรทัด DROP ด้านล่างจะลบของเก่าทิ้งก่อน (ข้อมูลหายหมด ระวังตอนใช้งานจริง)

BEGIN;

DROP VIEW  IF EXISTS dorm_room_stats, dorm_review_stats;
DROP TABLE IF EXISTS dorm_images, bookings, favorites, reviews, rooms, dorms, members, zones CASCADE;

-- ---------------------------------------------------------------------------
-- zones: ย่าน/ทำเลในนครปฐม ใช้เป็นตัวกรองหลักของหน้าค้นหา
-- ---------------------------------------------------------------------------
CREATE TABLE zones (
    id          SERIAL        PRIMARY KEY,
    slug        VARCHAR(40)   NOT NULL UNIQUE,      -- ใช้ใน URL เช่น /dorms?zone=salaya
    name_th     VARCHAR(100)  NOT NULL,
    landmark    VARCHAR(150),                       -- สถานที่สำคัญใกล้ ๆ เช่น มหาวิทยาลัย
    position    SMALLINT      NOT NULL DEFAULT 0    -- ลำดับที่แสดงบนหน้าแรก
);

-- ---------------------------------------------------------------------------
-- members: สมาชิกของเว็บ (Guest คือคนที่ยังไม่มีแถวในตารางนี้หรือยังไม่ login)
-- ---------------------------------------------------------------------------
CREATE TABLE members (
    id             BIGSERIAL     PRIMARY KEY,
    email          VARCHAR(255)  NOT NULL,
    password_hash  VARCHAR(255)  NOT NULL,          -- bcrypt ห้ามเก็บรหัสผ่านตรง ๆ
    display_name   VARCHAR(50)   NOT NULL CHECK (char_length(btrim(display_name)) >= 1),
    phone          VARCHAR(20),
    role           VARCHAR(10)   NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- อีเมลห้ามซ้ำโดยไม่สนตัวพิมพ์เล็กใหญ่
CREATE UNIQUE INDEX uq_members_email ON members (lower(email));

-- ---------------------------------------------------------------------------
-- dorms: หอพัก 1 แถวต่อ 1 หอ
-- ---------------------------------------------------------------------------
CREATE TABLE dorms (
    id             BIGSERIAL     PRIMARY KEY,
    name           VARCHAR(150)  NOT NULL CHECK (char_length(btrim(name)) >= 2),
    zone_id        INTEGER       NOT NULL REFERENCES zones (id),
    gender         VARCHAR(10)   NOT NULL DEFAULT 'mixed' CHECK (gender IN ('mixed', 'female', 'male')),
    address        VARCHAR(300)  NOT NULL,
    nearby         VARCHAR(200),                    -- ข้อความสั้น เช่น "เดิน 5 นาทีถึง ม.มหิดล"
    description    TEXT          NOT NULL DEFAULT '',
    facilities     TEXT[]        NOT NULL DEFAULT '{}',   -- เช่น {แอร์,Wi-Fi,ที่จอดรถ}
    phone          VARCHAR(20),
    line_id        VARCHAR(50),
    water_rate     VARCHAR(60),                     -- เช่น "100 บาท/คน/เดือน"
    electric_rate  VARCHAR(60),                     -- เช่น "7 บาท/หน่วย"
    deposit        VARCHAR(100),                    -- เช่น "ล่วงหน้า 1 เดือน + ประกัน 3,000"
    is_published   BOOLEAN       NOT NULL DEFAULT TRUE,   -- FALSE = ซ่อนจาก Guest/Member แต่ Admin ยังเห็น
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_dorms_zone ON dorms (zone_id);
CREATE INDEX idx_dorms_name_lower ON dorms (lower(name));

-- ---------------------------------------------------------------------------
-- rooms: ประเภทห้องของแต่ละหอ (หอ 1 หอมีได้หลายแบบ ราคาต่างกัน)
-- ---------------------------------------------------------------------------
CREATE TABLE rooms (
    id           BIGSERIAL     PRIMARY KEY,
    dorm_id      BIGINT        NOT NULL REFERENCES dorms (id) ON DELETE CASCADE,
    name         VARCHAR(100)  NOT NULL CHECK (char_length(btrim(name)) >= 1),
    size_sqm     NUMERIC(5,1)  CHECK (size_sqm > 0),
    price_month  INTEGER       NOT NULL CHECK (price_month > 0),   -- บาท/เดือน
    available    SMALLINT      NOT NULL DEFAULT 0 CHECK (available >= 0),   -- จำนวนห้องว่าง
    position     SMALLINT      NOT NULL DEFAULT 0
);

CREATE INDEX idx_rooms_dorm ON rooms (dorm_id, position);

-- ---------------------------------------------------------------------------
-- dorm_images: รูปของหอ หอละหลายรูป รูปแรก (position น้อยสุด) คือรูปปก
-- เก็บแค่ลิงก์ ไม่ได้เก็บไฟล์รูปในฐานข้อมูล: https://... หรือ /photos/ชื่อไฟล์ (ไฟล์ใน public/photos ของหน้าเว็บ)
-- ---------------------------------------------------------------------------
CREATE TABLE dorm_images (
    id        BIGSERIAL     PRIMARY KEY,
    dorm_id   BIGINT        NOT NULL REFERENCES dorms (id) ON DELETE CASCADE,
    url       VARCHAR(500)  NOT NULL CHECK (url LIKE 'https://%' OR url LIKE '/photos/%'),
    caption   VARCHAR(100),                         -- เช่น "ห้องแอร์มาตรฐาน", "ล็อบบี้"
    position  SMALLINT      NOT NULL DEFAULT 0
);

CREATE INDEX idx_dorm_images_dorm ON dorm_images (dorm_id, position);

-- ---------------------------------------------------------------------------
-- reviews: รีวิวจากสมาชิก 1 คนรีวิวหอเดียวกันได้ 1 ครั้ง
-- ---------------------------------------------------------------------------
CREATE TABLE reviews (
    id          BIGSERIAL    PRIMARY KEY,
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    dorm_id     BIGINT       NOT NULL REFERENCES dorms (id) ON DELETE CASCADE,
    rating      SMALLINT     NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text        TEXT         NOT NULL CHECK (char_length(btrim(text)) >= 10),   -- กติกาเดียวกับหน้าเว็บและ Go
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (member_id, dorm_id)
);

CREATE INDEX idx_reviews_dorm_created ON reviews (dorm_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- favorites: หอที่สมาชิกกดบันทึกไว้ (รายการโปรด)
-- ---------------------------------------------------------------------------
CREATE TABLE favorites (
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    dorm_id     BIGINT       NOT NULL REFERENCES dorms (id) ON DELETE CASCADE,
    added_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, dorm_id)
);

CREATE INDEX idx_favorites_member ON favorites (member_id, added_at DESC);

-- ---------------------------------------------------------------------------
-- bookings: คำขอนัดดูห้อง  Member ส่งคำขอ -> Admin อนุมัติ/ปฏิเสธ -> Member เห็นผล
-- ---------------------------------------------------------------------------
CREATE TABLE bookings (
    id             BIGSERIAL     PRIMARY KEY,
    member_id      BIGINT        NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    dorm_id        BIGINT        NOT NULL REFERENCES dorms (id) ON DELETE CASCADE,
    room_id        BIGINT        REFERENCES rooms (id) ON DELETE SET NULL,   -- ไม่เลือกประเภทห้องก็ได้
    visit_date     DATE          NOT NULL,
    contact_phone  VARCHAR(20)   NOT NULL,
    note           VARCHAR(300),
    status         VARCHAR(12)   NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    admin_note     VARCHAR(300),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- 1 คนมีคำขอที่ "รอดำเนินการ" กับหอเดียวกันได้ทีละ 1 คำขอ
CREATE UNIQUE INDEX uq_bookings_one_pending ON bookings (member_id, dorm_id) WHERE status = 'pending';
CREATE INDEX idx_bookings_member ON bookings (member_id, created_at DESC);
CREATE INDEX idx_bookings_status ON bookings (status, created_at DESC);

-- ---------------------------------------------------------------------------
-- view สรุปตัวเลขต่อหอ คำนวณสดจากตารางจริง ไม่ต้องเก็บซ้ำ
-- แยกเป็น 2 view เพราะถ้า JOIN rooms กับ reviews พร้อมกัน แถวจะคูณกันจนตัวเลขผิด
-- ---------------------------------------------------------------------------
CREATE VIEW dorm_room_stats AS
SELECT dorm_id,
       min(price_month)  AS price_min,
       max(price_month)  AS price_max,
       sum(available)    AS available
FROM rooms
GROUP BY dorm_id;

CREATE VIEW dorm_review_stats AS
SELECT dorm_id,
       count(*)               AS review_count,
       round(avg(rating), 1)  AS rating
FROM reviews
GROUP BY dorm_id;

-- ---------------------------------------------------------------------------
-- ย่านในนครปฐม
-- ---------------------------------------------------------------------------
INSERT INTO zones (slug, name_th, landmark, position) VALUES
    ('salaya',        'ศาลายา',              'ม.มหิดล ศาลายา',                    1),
    ('sanamchan',     'สนามจันทร์',          'ม.ศิลปากร พระราชวังสนามจันทร์',    2),
    ('kamphaengsaen', 'กำแพงแสน',            'ม.เกษตรศาสตร์ วิทยาเขตกำแพงแสน',  3),
    ('mueang',        'ในเมืองนครปฐม',       'ม.ราชภัฏนครปฐม / องค์พระปฐมเจดีย์', 4),
    ('nakhonchaisi',  'นครชัยศรี',           'ถนนเพชรเกษม',                      5);

COMMIT;
