-- ใช้เฉพาะกรณีที่รัน schema.sql รุ่นแรก (ที่ dorms ยังมีคอลัมน์ image_url) ไปแล้ว และไม่อยากล้างข้อมูล
-- ถ้ายังไม่มีข้อมูลสำคัญ ให้รัน schema.sql + seed.sql ใหม่แทนได้เลย
BEGIN;

CREATE TABLE IF NOT EXISTS dorm_images (
    id        BIGSERIAL     PRIMARY KEY,
    dorm_id   BIGINT        NOT NULL REFERENCES dorms (id) ON DELETE CASCADE,
    url       VARCHAR(500)  NOT NULL CHECK (url LIKE 'https://%' OR url LIKE '/photos/%'),
    caption   VARCHAR(100),
    position  SMALLINT      NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dorm_images_dorm ON dorm_images (dorm_id, position);

-- ย้ายรูปเดิม (ถ้ามี) มาเป็นรูปแรกของหอ แล้วลบคอลัมน์เก่า
INSERT INTO dorm_images (dorm_id, url, position)
SELECT id, image_url, 0 FROM dorms WHERE image_url LIKE 'https://%';
ALTER TABLE dorms DROP COLUMN IF EXISTS image_url;

-- ถ้าเคยสร้าง dorm_images รุ่นที่รับแค่ https:// ไว้แล้ว ให้ยอมรับ /photos/... ด้วย
ALTER TABLE dorm_images DROP CONSTRAINT IF EXISTS dorm_images_url_check;
ALTER TABLE dorm_images ADD CONSTRAINT dorm_images_url_check CHECK (url LIKE 'https://%' OR url LIKE '/photos/%');

COMMIT;
