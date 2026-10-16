# ใช้ฐานข้อมูลร่วมกับระบบ check-in

อัปเดต 14 ต.ค. 2569

SDMS ใช้ PostgreSQL บน Supabase ตัวเดียวกับระบบ check-in แต่เก็บตารางไว้ใน schema `sdms` และเชื่อมต่อด้วย role `sdms_app` ของตัวเอง ระบบ check-in ไม่ต้องแก้อะไรเลย

| | check-in | SDMS |
| --- | --- | --- |
| schema | `public` | `sdms` |
| role ที่แอปใช้ต่อ | `postgres` | `sdms_app` |
| การสร้างตาราง | ไฟล์ SQL + `schema_migrations` | GORM AutoMigrate (`go run ./cmd/migrate`) |
| สิทธิ์ข้ามระบบ | ไม่มี | อ่าน view `sdms.checkin_teacher_accounts` ได้อย่างเดียว (เฉพาะเมื่อเปิดใช้การนำเข้าครู) |

**กติกาที่ระบบคุมให้**

- `sdms_app` ไม่มีสิทธิ์อ่านหรือเขียนตารางของ check-in ใน `public`
- SDMS ตรวจตอนเริ่มทุกครั้งว่า `current_schema()` ตรงกับ `DB_SCHEMA` ถ้าไม่ตรงจะไม่เริ่ม จึงไม่มีทางสร้างหรืออ่านตารางผิด schema
- ทุกตารางของ SDMS เปิด RLS แบบเดียวกับ check-in ถ้ามีคนเผลอเปิด schema นี้ให้ Data API เห็น role `anon` และ `authenticated` ก็ยังอ่านแถวไม่ได้
- บัญชีผู้ใช้ของสองระบบยังแยกกัน แต่นำเข้าครูจาก check-in มาเป็นบัญชี SDMS ได้ด้วยคำสั่งเดียว (หัวข้อ 4)

## 1. ตั้งค่าบน Supabase (ทำครั้งเดียว)

1. เปิด [`deploy/supabase/sdms-schema.sql`](deploy/supabase/sdms-schema.sql) แก้ `app_password` เป็นรหัสผ่านสุ่มยาวอย่างน้อย 16 ตัวอักษร
2. วางทั้งไฟล์ใน Supabase Dashboard → SQL Editor แล้วกด Run ผลลัพธ์ท้ายสคริปต์ต้องเห็น `rolsuper = false` และ `rolconfig = {search_path=sdms}`
3. ลบรหัสผ่านออกจาก SQL Editor และอย่าบันทึกเป็น snippet
4. ถ้าจะนำเข้าครูจาก check-in ให้รัน [`deploy/supabase/checkin-teacher-view.sql`](deploy/supabase/checkin-teacher-view.sql) ต่อ ผลลัพธ์จะบอกจำนวนครูและจำนวนที่มีบัญชีใน check-in

> ไม่ต้องเพิ่ม `sdms` ในรายการ Exposed schemas ของการตั้งค่า Data API เพราะ SDMS ต่อฐานข้อมูลตรงจาก Go ไม่ได้ใช้ Data API

ทั้งสองสคริปต์รันซ้ำได้ ถ้ารัน `sdms-schema.sql` ซ้ำ รหัสผ่านของ `sdms_app` จะถูกตั้งใหม่ตามค่าที่ใส่

## 2. ตั้งค่า `.env` ของ SDMS

ใช้ host และ port เดียวกับ `DATABASE_URL` ของ check-in เปลี่ยนเฉพาะ username และรหัสผ่าน

```dotenv
DATABASE_URL=postgresql://sdms_app.<project-ref>:<รหัสผ่าน>@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres?sslmode=require&TimeZone=Asia/Bangkok
DB_SCHEMA=sdms
DB_MAX_OPEN_CONNS=10
DB_MAX_IDLE_CONNS=2
```

- `<project-ref>` คือส่วนหลัง `postgres.` ใน username ของ `DATABASE_URL` ของ check-in
- ถ้ารหัสผ่านมีอักขระพิเศษ เช่น `@ : / ? # %` ต้อง URL-encode ก่อนใส่
- ใช้ connection pooler แบบ session mode (port `5432`) แบบเดียวกับ check-in
- check-in เปิด connection ได้สูงสุด 25 และ SDMS อีก 10 ตรวจ Pool Size ในการตั้งค่า Connection pooling ของ Supabase ให้พอสำหรับทั้งสองระบบ
- หลังสร้าง role หรือเปลี่ยนรหัสผ่าน pooler อาจใช้เวลาสักครู่ก่อนรู้จักรหัสใหม่ ถ้าเจอ `password authentication failed` ให้รอแล้วลองใหม่

## 3. สร้างตารางและแอดมินคนแรก

ต้องรันด้วย `DATABASE_URL` ของ `sdms_app` เพื่อให้ `sdms_app` เป็นเจ้าของตาราง

```powershell
go run ./cmd/migrate
# ควรเห็น: migration completed successfully (schema "sdms")

$env:INITIAL_ADMIN_PASSWORD = "<รหัสผ่านแอดมิน>"
go run ./cmd/create-admin -username sdms.admin -first-name "ผู้ดูแลระบบ" -position "ผู้ดูแลระบบ"
Remove-Item Env:\INITIAL_ADMIN_PASSWORD

go run ./cmd/api
```

ตั้ง username ของแอดมินให้ไม่ซ้ำกับ username ของครูใน check-in ถ้าซ้ำ ครูคนนั้นจะไม่ถูกนำเข้า

## 4. นำเข้าครูจาก check-in

คำสั่ง `import-checkin-teachers` สร้างบัญชี SDMS ให้ครูทุกคนที่มีบัญชีใน check-in และคัดลอก password hash (bcrypt) มาด้วย ครูจึงเข้า SDMS ด้วย username และรหัสผ่านเดียวกับ check-in ได้ทันที บัญชีที่นำเข้าถูกบันทึกในตาราง `checkin_user_links` จึงสั่งซ้ำได้โดยไม่สร้างบัญชีซ้ำ

ต้องรัน `checkin-teacher-view.sql` ในหัวข้อ 1 ก่อน

```powershell
# 1) ดูแผนก่อน ยังไม่บันทึกอะไร
go run ./cmd/import-checkin-teachers -division aca

# 2) ตรวจรายการแล้วบันทึกจริง
go run ./cmd/import-checkin-teachers -division aca -apply
```

**ข้อมูลที่ได้**

| SDMS | มาจาก check-in |
| --- | --- |
| username | `user_accounts.username` แปลงเป็นตัวพิมพ์เล็ก (เข้าระบบด้วยตัวพิมพ์ใหญ่ก็ได้เหมือนเดิม) |
| รหัสผ่าน | password hash เดิม |
| คำนำหน้า ชื่อ นามสกุล | เก็บแยกใน `prefix`, `first_name`, `last_name` ตามข้อมูลจาก check-in และประกอบเป็นชื่อแสดงผล เช่น นายสมชาย ใจดี |
| ตำแหน่ง | `ครู` ตามด้วยกลุ่มสาระ เช่น `ครู คณิตศาสตร์` (เปลี่ยนคำนำด้วย `-position`) |
| ฝ่าย | ถ้ากลุ่มสาระตรงกับชื่อหรือรหัสฝ่าย เช่น `ฝ่ายวิชาการ` ใช้ฝ่ายนั้น ไม่ตรงใช้ฝ่ายจาก `-division` |
| ผู้บริหาร / แอดมิน | ไม่ตั้งให้ แอดมินตั้งเองในหน้าจัดการระบบ |

**ครูที่ถูกข้าม** (รายงานบอกเหตุผลทีละคน แอดมินสร้างบัญชีให้เองในหน้าจัดการระบบได้)

- ยังไม่มีบัญชีใน check-in หรือครู/บัญชีถูกปิดใช้งาน
- username มีอักษรที่ SDMS ไม่รับ SDMS รับเฉพาะ `a-z 0-9 . _ -` ยาว 3–32 ตัว แต่ check-in รับภาษาไทยได้
- username ซ้ำกับบัญชีที่มีใน SDMS อยู่แล้ว คำสั่งนี้ไม่แตะบัญชีที่แอดมินสร้างเอง

**ตัวเลือก**

| flag | ความหมาย |
| --- | --- |
| `-apply` | บันทึกจริง ไม่ใส่ = แสดงแผนอย่างเดียว |
| `-division aca,per` | ฝ่ายของครูที่กลุ่มสาระไม่ตรงกับฝ่ายใด (รหัสฝ่าย: `aca` `bud` `per` `gen`) |
| `-position ครู` | คำนำของตำแหน่ง |
| `-with-prefix` | รองรับไว้เพื่อไม่ให้คำสั่งเดิมพัง แต่ไม่มีผลแล้ว เพราะระบบนำเข้าคำนำหน้าเสมอ |
| `-sync-passwords` | คัดลอกรหัสผ่านล่าสุดจาก check-in ให้บัญชีที่นำเข้าแล้ว |

**สิ่งที่ควรรู้**

- migration รุ่นที่เปลี่ยนชื่อเป็น `prefix`, `first_name`, `last_name` จะเติมส่วนชื่อของบัญชีที่เคยนำเข้าจาก view นี้ให้อัตโนมัติ โดยแก้เฉพาะชื่อที่ยังตรงกับรูปแบบ importer รุ่นเก่า จึงไม่ทับชื่อที่แอดมินแก้เองแล้ว
- รหัสผ่านถูกคัดลอกครั้งเดียว หลังจากนั้นสองระบบแยกกัน ถ้าครูเปลี่ยนรหัสใน check-in ให้สั่ง `go run ./cmd/import-checkin-teachers -sync-passwords -apply` คำสั่งนี้เขียนทับรหัสที่แอดมิน SDMS ตั้งให้ครูที่นำเข้า และให้ครูคนนั้นออกจากระบบทุกเครื่อง
- ครูที่ check-in ยังบังคับให้เปลี่ยนรหัสชั่วคราว จะได้รหัสชั่วคราวนั้นใน SDMS ด้วย รายงานจะเตือนไว้ ให้ครูเปลี่ยนรหัสใน check-in ก่อนแล้วสั่ง `-sync-passwords`
- ครูที่ถูกปิดใช้งานใน check-in ภายหลัง จะไม่ถูกปิดใช้งานใน SDMS อัตโนมัติ รายงานจะเตือนให้แอดมินปิดเอง
- ครูที่นำเข้าหลังหัวข้อถูกสร้างแล้ว ไม่ถูกนับเป็นผู้ต้องส่งของหัวข้อนั้น ตามกติกาเดิมของ SDMS
- view เปิดให้ `sdms_app` อ่าน password hash ของครูได้ ถ้าเลิกใช้การนำเข้าแล้วลบ view ได้ด้วย `DROP VIEW sdms.checkin_teacher_accounts;`

## 5. พัฒนาในเครื่องด้วย docker

ไม่ต้องตั้งค่าเพิ่ม `docker compose up -d postgres` แล้ว `go run ./cmd/migrate` ได้เลย migrate จะสร้าง schema `sdms` ให้เอง และ user `sdms` ของ docker จะเห็น schema นี้ก่อนโดยอัตโนมัติ (search_path เริ่มต้นคือ `"$user", public`)

ฐานใน docker ที่สร้างก่อนรุ่นนี้มีตารางเก่าอยู่ใน `public` ซึ่งไม่ถูกใช้แล้ว ล้างทิ้งได้ด้วย `docker compose down -v` (ข้อมูลในเครื่องหายทั้งหมด)

integration test ใช้ `SDMS_INTEGRATION_DATABASE_URL` เหมือนเดิม ถ้าต่อด้วย role อื่นที่ไม่ใช่ `sdms` หรือ `sdms_app` ให้เติม `&search_path=sdms` ท้าย URL

ชี้ `SDMS_INTEGRATION_DATABASE_URL` ไปที่ฐานใน docker เท่านั้น อย่าชี้ไปที่ Supabase จริง test ของหัวข้อบางตัวสมมติว่าฝ่ายแรกยังไม่มีสมาชิก จึงล้มเมื่อมีครูที่นำเข้าแล้ว และไม่ควรรัน test บนฐานที่ใช้งานจริงอยู่แล้ว

## 6. แก้ปัญหา

| อาการ | วิธีแก้ |
| --- | --- |
| `resolves tables to schema "public" instead of "sdms"` | ต่อด้วย role ผิด เช่น `postgres` ให้ใช้ `sdms_app` **ห้าม** `ALTER ROLE postgres SET search_path` เพราะระบบ check-in จะหาตารางของตัวเองไม่เจอ |
| `database schema "sdms" does not exist` | ยังไม่ได้รัน `sdms-schema.sql` (บน Supabase) หรือ `go run ./cmd/migrate` (ในเครื่อง) |
| `permission denied for schema sdms` | ยังไม่ได้รัน `sdms-schema.sql` หรือ `DATABASE_URL` ใช้ role อื่น |
| `permission denied for table ...` ตอนเปิด API | ตารางถูกสร้างด้วย role อื่น รัน migrate ด้วย `sdms_app` ตั้งแต่แรก |
| `check-in source "checkin_teacher_accounts" was not found` | ยังไม่ได้รัน `checkin-teacher-view.sql` |
| คำสั่งนำเข้าไม่เห็นครูเลย | view ต้องถูกสร้างโดยเจ้าของตารางของ check-in (ปกติคือ `postgres`) สคริปต์ตรวจให้แล้ว ให้รันใหม่ใน SQL Editor |
| `password authentication failed` ผ่าน pooler | ตรวจรูปแบบ username `sdms_app.<project-ref>` และการ URL-encode รหัสผ่าน ถ้าเพิ่งสร้าง role ให้รอสักครู่แล้วลองใหม่ |

## 7. เลิกใช้ฐานร่วม

คำสั่งนี้ลบข้อมูล SDMS ทั้งหมดบน Supabase โดยไม่กระทบ check-in รันใน SQL Editor หลังสำรองข้อมูลแล้วเท่านั้น

```sql
DROP SCHEMA sdms CASCADE;
DROP ROLE sdms_app;
```
