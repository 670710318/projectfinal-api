# DormHub นครปฐม: Backend (Go)

API ของเว็บแนะนำหอพักในนครปฐม ทำด้วย Go `net/http` + `pgx/v5` + JWT + bcrypt (stack เดียวกับ lab MovieHub)
ฐานข้อมูลอยู่บน Supabase (PostgreSQL) และ deploy บน Render แบบเชื่อม GitHub repo

## โครงสร้างโปรเจกต์

```
dormhub-api/
├── main.go            เปิดเซิร์ฟเวอร์ + ผูกเส้นทาง API ทั้งหมด (แบ่งเป็น Guest / Member / Admin ชัดเจน)
├── config/            ค่าคงที่ทั้งระบบ (กติการหัสผ่าน รีวิว สถานะ) + อ่าน Environment Variable
├── auth/              ออก/ตรวจตั๋ว JWT และ hash รหัสผ่านด้วย bcrypt (+ เทสต์)
├── mw/                middleware: CORS, Logging, Recovery, RequireAuth, RequireAdmin (+ เทสต์)
├── handler/           รับคำขอ HTTP -> ตรวจข้อมูล -> เรียก store -> ตอบ JSON (+ เทสต์ validate)
├── store/             คุยกับฐานข้อมูลอย่างเดียว (SQL ทั้งหมดอยู่ที่นี่)
└── db/
    ├── schema.sql     สร้างตารางทั้งหมด (รันใน Supabase SQL Editor)
    └── seed.sql       ข้อมูลตัวอย่าง 12 หอ (สมมติขึ้น)
```

เส้นทางของข้อมูล 1 คำขอ: `main.go (route)` -> `mw (ตรวจตั๋ว/สิทธิ์)` -> `handler (ตรวจข้อมูล)` -> `store (SQL)` -> Supabase

## API

รูปแบบ error เดียวทั้งระบบ: `{"error": {"code": "EMAIL_TAKEN", "message": "อีเมลนี้มีบัญชีอยู่แล้ว"}}`

| สิทธิ์ | Method | Path | ทำอะไร |
|---|---|---|---|
| Guest | GET | `/api/health` | เช็กว่าเซิร์ฟเวอร์และ DB ทำงาน |
| Guest | POST | `/api/auth/register` | สมัครสมาชิก (ตอบตั๋วมาเลย) |
| Guest | POST | `/api/auth/login` | เข้าสู่ระบบ ได้ `{ token, member }` |
| Guest | GET | `/api/zones` | ย่านทั้งหมด + จำนวนหอ |
| Guest | GET | `/api/dorms?q=&zone=&gender=&priceMin=&priceMax=&available=1&sort=&limit=` | ค้นหาหอ (เฉพาะที่เผยแพร่) |
| Guest | GET | `/api/dorms/{id}` | รายละเอียดหอ + ประเภทห้อง |
| Guest | GET | `/api/dorms/{id}/reviews` | รีวิวของหอ |
| Member | GET / PUT | `/api/me` | ดู / แก้โปรไฟล์ (ชื่อ เบอร์โทร) |
| Member | PUT | `/api/me/password` | เปลี่ยนรหัสผ่าน (ต้องยืนยันรหัสเดิม) |
| Member | GET | `/api/me/favorites` | รายการโปรด |
| Member | PUT / DELETE | `/api/me/favorites/{dormId}` | เพิ่ม / ลบรายการโปรด |
| Member | POST | `/api/dorms/{id}/reviews` | เขียนรีวิว (1 คน 1 รีวิวต่อหอ) |
| Member | DELETE | `/api/reviews/{id}` | ลบรีวิวของตัวเอง (Admin ลบได้ทุกอัน) |
| Member | POST | `/api/dorms/{id}/bookings` | ส่งคำขอนัดดูห้อง |
| Member | GET | `/api/me/bookings` | คำขอของฉัน + สถานะ |
| Member | POST | `/api/me/bookings/{id}/cancel` | ยกเลิกคำขอที่ยังรอ |
| Admin | GET | `/api/admin/stats` | ตัวเลขสรุป |
| Admin | GET / POST | `/api/admin/dorms` | หอทั้งหมด (รวมที่ซ่อน) / เพิ่มหอ |
| Admin | GET / PUT / DELETE | `/api/admin/dorms/{id}` | ดู / แก้ / ลบหอ (รวมประเภทห้อง) |
| Admin | GET | `/api/admin/bookings?status=pending` | คำขอนัดดูห้องทั้งหมด |
| Admin | PUT | `/api/admin/bookings/{id}` | อนุมัติ / ปฏิเสธ `{status, adminNote}` |
| Admin | GET | `/api/admin/members` | สมาชิกทั้งหมด |
| Admin | PUT | `/api/admin/members/{id}/role` | ตั้ง / ถอด Admin |

ไม่มีตั๋ว = **401**, มีตั๋วแต่ไม่ใช่ Admin = **403** (ตรวจที่ `mw/auth.go` ก่อนถึง handler)

## รันในเครื่อง

```bash
cp .env.example .env          # ใส่ DATABASE_URL, JWT_SECRET, ALLOWED_ORIGINS
set -a; source .env; set +a   # โหลดตัวแปรเข้า shell
go run .                      # ฟังที่ :8080
go test ./...                 # รันเทสต์
```

ทดสอบเร็ว ๆ: `curl http://localhost:8080/api/dorms?zone=salaya`

## Deploy

### 1) Supabase (Database)
1. สร้างโปรเจกต์ใหม่ เลือก region **Southeast Asia (Singapore)**
2. **SQL Editor** > วางไฟล์ `db/schema.sql` > Run แล้ววาง `db/seed.sql` > Run
3. **Connect** (ปุ่มบนสุด) > เลือก **Session pooler** > คัดลอก URI แล้วแทน `[YOUR-PASSWORD]` ด้วยรหัสฐานข้อมูล
   - ห้ามใช้ Direct connection: เป็น IPv6 ซึ่ง Render ต่อไม่ได้
   - ห้ามใช้ Transaction pooler (พอร์ต 6543): pgx ใช้ prepared statement แล้วจะ error

### 2) Render (Backend)
1. push โค้ดขึ้น GitHub
2. Render > **New > Web Service** > เชื่อม repo นี้
3. ตั้งค่า
   - Language: **Go**
   - Build Command: `go build -o app .`
   - Start Command: `./app`
   - Health Check Path: `/api/health`
4. **Environment**: `DATABASE_URL`, `JWT_SECRET` (สุ่ม 32 ตัวขึ้นไป เช่น `openssl rand -hex 32`), `ALLOWED_ORIGINS=https://<ชื่อเว็บ>.vercel.app`
5. ได้ URL เช่น `https://dormhub-api.onrender.com` เอาไปใส่ `REACT_APP_API_URL` บน Vercel

Render ฟรีจะหลับถ้าไม่มีคนใช้ 15 นาที คำขอแรกอาจช้า 30-60 วินาที **ก่อนอัดคลิปให้เปิด `/api/health` ปลุกไว้ก่อน**

### 3) ตั้ง Admin คนแรก
สมัครสมาชิกผ่านหน้าเว็บ แล้วรันใน Supabase SQL Editor:
```sql
UPDATE members SET role = 'admin' WHERE lower(email) = lower('อีเมลที่สมัคร');
```
แล้ว logout / login ใหม่ (ตั๋วใบใหม่ถึงจะมี role admin)

## ปัญหาที่อาจเจอ

| อาการ | สาเหตุ / วิธีแก้ |
|---|---|
| หน้าเว็บขึ้น "เชื่อมต่อเซิร์ฟเวอร์ไม่ได้" | Render กำลังตื่น รอสักครู่ หรือ `ALLOWED_ORIGINS` ไม่ตรงกับโดเมน Vercel (ห้ามมี `/` ท้าย) |
| Render log: `ต่อฐานข้อมูลไม่ติด` | ใช้ Direct connection อยู่ ให้เปลี่ยนเป็น Session pooler |
| `prepared statement already exists` | ใช้ Transaction pooler (6543) ให้เปลี่ยนเป็น Session pooler (5432) |
| build ฟ้อง `missing go.sum entry` | รัน `go mod tidy` แล้ว commit `go.sum` |
| เปลี่ยน role แล้วสิทธิ์ยังไม่เปลี่ยน | role อยู่ในตั๋ว ต้อง login ใหม่ (ตั๋วเดิมใช้ได้ถึงหมดอายุ 24 ชม. ตาม `config.TokenTTL`) |

## สิ่งที่แก้จาก lab เดิม
- `Register` เรียก `CreateMember` สลับลำดับ hash กับชื่อ ทำให้สมัครแล้ว login ไม่ได้: แก้แล้ว
- `mw/auth.go` และ `handler/admin.go` เดิมว่าง: เขียน `RequireAuth` / `RequireAdmin` และส่วน Admin ครบ
- พอร์ตเดิมฝังไว้ที่ `:8080`: เปลี่ยนเป็นอ่าน `PORT` จาก Render
- รูปแบบ error ฝั่ง Go กับฝั่ง React ไม่ตรงกัน: ฝั่ง React อ่าน `error.message` แล้ว
