# SDMS REST API

เอกสารนี้เป็นสัญญา API ฉบับรวมของ SDMS ครอบคลุม route ที่ประกาศใช้งานอยู่ใน backend ทั้งหมด รวมถึงการกำหนดผู้ต้องส่งเป็นรายคนในแต่ละหัวข้อ

## ข้อมูลทั่วไป

- Base URL สำหรับ local development ตามค่า default: `http://localhost:8081/api`
- Swagger UI: `http://localhost:8081/api/docs`
- OpenAPI 3.0 YAML: `http://localhost:8081/api/openapi.yaml`
- Authentication ใช้ server-side session cookie ชื่อ `sdms_session`
- Request และ response ปกติใช้ `application/json; charset=utf-8`
- UUID ส่งเป็น JSON string ตามรูปแบบ UUID
- วันที่ใช้รูปแบบ `YYYY-MM-DD` (ค.ศ.)
- วันเวลาใช้รูปแบบ RFC 3339 เช่น `2026-11-07T08:00:00Z`
- การตัดสินกำหนดส่งใช้วันที่ตามเขตเวลา `Asia/Bangkok` และถือว่าส่งทันจนจบวันกำหนดส่ง
- Array ที่ไม่มีข้อมูลจะคืนเป็น `[]` และค่าที่ไม่มีจะคืนเป็น `null` เว้นแต่ระบุไว้เป็นอย่างอื่น
- ตัวอย่างที่ใช้ `{}` แทน object แบบย่อ ให้ดู schema เต็มของ object นั้นจากหัวข้อก่อนหน้า; response จริงจะคืน property ครบ ไม่ได้คืน object ว่าง
- ตัวอย่าง response ของแต่ละ endpoint ด้านล่างแสดงเฉพาะค่าภายใน `data` เพื่อให้อ่าน schema ง่าย ยกเว้นตัวอย่างที่ระบุว่าเป็น envelope เต็ม

ตัวอย่างเรียก API หลัง login ด้วย `curl`:

```bash
curl -c cookies.txt \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"sdms-admin!"}' \
  http://localhost:8081/api/auth/login

curl -b cookies.txt http://localhost:8081/api/me
```

## Authentication และสิทธิ์

ยกเว้น `GET /health`, `POST /auth/login` และ `POST /auth/logout` ทุก endpoint ต้องส่ง session cookie ที่ยังใช้งานได้

| คำที่ใช้ในเอกสาร | สิทธิ์ |
| --- | --- |
| ผู้ใช้ | บัญชีที่ login สำเร็จ |
| ผู้แก้หัวข้อ | แอดมิน, หัวหน้าฝ่ายเจ้าของหัวข้อ หรือผู้สร้างหัวข้อ |
| ผู้จัดการฝ่าย | แอดมิน หรือหัวหน้าฝ่ายนั้น |
| ผู้ดูคำตอบลับทั้งหมด | ผู้บริหาร, หัวหน้าฝ่ายเจ้าของหัวข้อ หรือผู้สร้างหัวข้อ |
| แอดมิน | บัญชีที่มี `is_admin = true` |

หมายเหตุ: แอดมินเป็นสิทธิ์ดูแลระบบ ไม่ได้ทำให้เห็นข้อมูลลับหรือสถิติฝ่ายโดยอัตโนมัติ เว้นแต่ endpoint นั้นกำหนดสิทธิ์แอดมินไว้โดยตรงหรือบัญชีมีบทบาทด้านเนื้อหาร่วมด้วย

## รูปแบบ response

JSON response ที่สำเร็จและมีข้อมูล:

```json
{
  "status": 200,
  "data": {},
  "message": "success"
}
```

JSON response ที่สำเร็จแต่ไม่มีข้อมูล เช่น login, logout, reset password และ delete:

```json
{
  "status": 200,
  "message": "success"
}
```

ทุก error เป็น JSON รูปแบบเดียวกัน โดย `status` ใน body ตรงกับ HTTP status:

```json
{
  "status": 400,
  "message": "at least one assignee is required"
}
```

| HTTP | ความหมาย |
| --- | --- |
| `400` | body, path หรือ query ไม่ถูกต้อง |
| `401` | ไม่มี session, session หมดอายุ หรือ login ไม่สำเร็จ |
| `403` | login แล้วแต่ไม่มีสิทธิ์ |
| `404` | ไม่พบข้อมูล |
| `409` | สถานะปัจจุบันไม่อนุญาตให้ทำรายการ |
| `413` | request หรือไฟล์ใหญ่เกินกำหนด |
| `500` | ข้อผิดพลาดภายในระบบ |
| `503` | service ของ endpoint ยังไม่พร้อม |

## สรุป endpoint ทั้งหมด

### ระบบและบัญชี

| Method | Path | สิทธิ์ | ผลลัพธ์หลัก |
| --- | --- | --- | --- |
| `GET` | `/health` | สาธารณะ | สถานะ API |
| `POST` | `/auth/login` | สาธารณะ | สร้าง session cookie |
| `POST` | `/auth/logout` | สาธารณะ | ยกเลิก session และ cookie |
| `GET` | `/me` | ผู้ใช้ | profile ของผู้ใช้ปัจจุบัน |
| `GET` | `/users` | ผู้ใช้ | directory บุคลากรที่เปิดใช้งาน |
| `GET` | `/years` | ผู้ใช้ | รายการปีการศึกษา |

### หัวข้อ การส่งงาน และไฟล์

| Method | Path | สิทธิ์ | ผลลัพธ์หลัก |
| --- | --- | --- | --- |
| `GET` | `/divisions` | ผู้ใช้ | รายการฝ่าย |
| `GET` | `/divisions/{code}/topics` | ผู้ใช้ | รายการหัวข้อของฝ่าย |
| `POST` | `/topics` | ผู้จัดการฝ่าย | สร้างหัวข้อ |
| `PATCH` | `/topics/{uuid}` | ผู้แก้หัวข้อ | แก้หัวข้อ |
| `POST` | `/topics/{uuid}/close` | ผู้แก้หัวข้อ | ปิดรับคำตอบ |
| `POST` | `/topics/{uuid}/reopen` | ผู้แก้หัวข้อ | เปิดรับคำตอบอีกครั้ง |
| `DELETE` | `/topics/{uuid}` | ผู้แก้หัวข้อ | ลบหัวข้อที่ยังไม่มีคำตอบ |
| `GET` | `/topics/{uuid}` | ผู้ใช้ตาม scope | รายละเอียดหัวข้อและตารางคำตอบ |
| `GET` | `/topics/{uuid}/pending-names` | ผู้แก้หัวข้อ | รายชื่อผู้ที่ยังไม่ส่ง |
| `GET` | `/me/pending` | ผู้ใช้ | หัวข้อเปิดที่ตนยังไม่ส่ง |
| `GET` | `/topics/{uuid}/my-submission` | ผู้ใช้ | แบบฟอร์มและคำตอบของตน |
| `PUT` | `/topics/{uuid}/my-submission` | ผู้ที่ถูกกำหนด | สร้างหรือแก้คำตอบ |
| `POST` | `/uploads` | ผู้ที่ถูกกำหนด | อัปโหลดไฟล์แบบ staged |
| `DELETE` | `/uploads/{uuid}` | เจ้าของไฟล์ staged | ลบไฟล์ staged |
| `GET` | `/files/{uuid}` | ผู้ใช้ตาม scope | เปิดดูหรือดาวน์โหลดไฟล์ attached |

### หน้าแรก แฟ้ม และค้นหา

| Method | Path | สิทธิ์ | ผลลัพธ์หลัก |
| --- | --- | --- | --- |
| `GET` | `/home` | ผู้ใช้ | dashboard ส่วนตัวและภาพรวมตามสิทธิ์ |
| `GET` | `/divisions/{code}/stats` | ผู้บริหาร/หัวหน้าฝ่าย | สถิติฝ่าย |
| `GET` | `/me/submissions` | ผู้ใช้ | ประวัติการส่งของตน |
| `GET` | `/me/files.zip` | ผู้ใช้ | ดาวน์โหลดไฟล์ของตนเป็น ZIP |
| `GET` | `/search` | ผู้ใช้ | ค้นหัวข้อและคำตอบตาม scope |

### งานแอดมิน

| Method | Path | ผลลัพธ์หลัก |
| --- | --- | --- |
| `GET` | `/admin/users` | รายการผู้ใช้ทั้งหมด |
| `POST` | `/admin/users` | สร้างผู้ใช้ |
| `PATCH` | `/admin/users/{uuid}` | แก้หรือเปิด/ปิดผู้ใช้ |
| `POST` | `/admin/users/{uuid}/password` | ตั้งรหัสผ่านใหม่ |
| `POST` | `/admin/users/import` | นำเข้าผู้ใช้จาก CSV/XLSX |
| `GET` | `/admin/users/import/template.csv` | ดาวน์โหลด CSV template |
| `GET` | `/admin/users/import/template.xlsx` | ดาวน์โหลด XLSX template |
| `GET` | `/admin/divisions` | รายการฝ่ายพร้อมหัวหน้า |
| `PUT` | `/admin/divisions/{code}/head` | ตั้งหัวหน้าฝ่าย |
| `GET` | `/admin/years` | รายการปีพร้อมจำนวนหัวข้อ |
| `POST` | `/admin/years` | สร้างปีการศึกษา |
| `POST` | `/admin/years/{year}/current` | ตั้งปี/ภาคเรียนปัจจุบัน |

ทุก endpoint ในกลุ่ม `/admin` ต้องเป็นแอดมิน

## Object ที่ใช้ร่วมกัน

### Division

```json
{
  "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30001",
  "code": "aca",
  "name": "ฝ่ายวิชาการ",
  "short_name": "วิชาการ"
}
```

บาง endpoint เพิ่ม `sort_order` ใน object นี้

### Topic

```json
{
  "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40001",
  "division": {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30001",
    "code": "aca",
    "name": "ฝ่ายวิชาการ",
    "short_name": "วิชาการ"
  },
  "academic_year": {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40002",
    "year": 2569
  },
  "semester": 1,
  "title": "รายงานผลการดำเนินงานประจำเดือน",
  "due_date": "2026-11-30",
  "is_open": true,
  "is_secret": false,
  "created_by_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30004",
  "fields": [
    {
      "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40003",
      "position": 1,
      "type": "text",
      "label": "ชื่อกิจกรรม",
      "required": true,
      "options": []
    }
  ],
  "assignee_count": 2,
  "assignee_uuids": [
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb30008"
  ],
  "closed_at": null,
  "created_at": "2026-11-07T08:00:00Z",
  "updated_at": "2026-11-07T08:00:00Z"
}
```

`assignee_uuids` จะมีใน response ของ endpoint จัดการหัวข้อ และใน `GET /topics/{uuid}` เฉพาะเมื่อผู้เรียกมีสิทธิ์แก้หัวข้อ ผู้ใช้ทั่วไปจะไม่เห็น property นี้ แต่ยังเห็น `assignee_count`

### ชนิดของ field และค่าใน submission

`values` เป็น object ที่ใช้ UUID ของ field เป็น key

| `type` | ค่าใน JSON | กฎหลัก |
| --- | --- | --- |
| `text` | string | ไม่เกิน 500 ตัวอักษร |
| `para` | string | ไม่เกิน 5,000 ตัวอักษร |
| `num` | number | รองรับทศนิยม |
| `date` | string `YYYY-MM-DD` | ต้องเป็นวันที่จริง |
| `choice` | string | ต้องตรงกับค่าใน `options` |
| `file` | array ของ file UUID | ไฟล์ต้องเป็น staged/attached ของผู้ส่ง หัวข้อ และ field เดียวกัน |

ตัวอย่าง:

```json
{
  "values": {
    "0199d16f-4f09-7c20-8cc7-e8c3363ea1b0": "PLC ชุมชนการเรียนรู้ทางวิชาชีพ",
    "0199d16f-4f09-7c20-8cc7-e8c3363ea1b1": 12.5,
    "0199d16f-4f09-7c20-8cc7-e8c3363ea1b2": "2026-11-20",
    "0199d16f-4f09-7c20-8cc7-e8c3363ea1b3": "ม.2",
    "0199d16f-4f09-7c20-8cc7-e8c3363ea1b4": [
      "0199d16f-4f09-7c20-8cc7-e8c3363ea1b5"
    ]
  }
}
```

### สถานะการส่ง

| Status | ความหมาย |
| --- | --- |
| `submitted` | ส่งแล้วภายในวันกำหนด |
| `late` | ส่งครั้งแรกหลังวันกำหนด |
| `pending` | ยังไม่ส่ง หัวข้อเปิด และยังไม่เลยกำหนด |
| `overdue` | ยังไม่ส่ง หัวข้อเปิด และเลยกำหนด |
| `not_submitted` | ยังไม่ส่งและหัวข้อปิดแล้ว |

## 1. ระบบและ Authentication

### `GET /health`

ไม่ต้อง login

Response `200 OK`:

```json
{"status":200,"data":{"status":"ok"},"message":"success"}
```

### `POST /auth/login`

Request:

```json
{
  "username": "teacher.one",
  "password": "safe-password"
}
```

รองรับทั้ง JSON และ form body เมื่อสำเร็จ server จะตั้ง HttpOnly session cookie

Response `200 OK`:

```json
{"status":200,"message":"success"}
```

กรณีผิดพลาด: request ไม่ครบตอบ `400`; credential ไม่ถูกต้องตอบ `401`

### `POST /auth/logout`

ยกเลิก session ฝั่ง server ถ้ามี และล้าง session cookie

Response `200 OK`:

```json
{"status":200,"message":"success"}
```

### `GET /me`

Response `200 OK`:

```json
{
  "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
  "username": "teacher.one",
  "prefix": "นาง",
  "first_name": "สมหญิง",
  "last_name": "ใจดี",
  "full_name": "นางสมหญิง ใจดี",
  "role": "division_head",
  "is_exec": false,
  "is_admin": false,
  "divisions": [],
  "head_divisions": []
}
```

`role` เป็นบทบาทหลักที่มีลำดับ `admin` > `executive` > `division_head` > `teacher` ส่วนบทบาทอื่นยังตรวจได้จาก flag และ `head_divisions`

## 2. Directory ฝ่าย และปีการศึกษา

### `GET /users`

คืนเฉพาะผู้ใช้ที่เปิดใช้งาน เหมาะสำหรับตัวเลือกผู้ต้องส่งและตัวกรอง `by` โดยไม่เปิดเผย username, บทบาท หรือสถานะแอดมิน

Response `200 OK`:

```json
[
  {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
    "prefix": "นาง",
    "first_name": "สมหญิง",
    "last_name": "ใจดี",
    "full_name": "นางสมหญิง ใจดี",
    "divisions": [],
    "is_topic_assignee_eligible": true
  }
]
```

`is_topic_assignee_eligible` เป็น `false` สำหรับผู้บริหาร การเลือกผู้ต้องส่งต้องกรองทั้ง flag นี้และ membership ของฝ่าย Backend จะตรวจซ้ำเมื่อบันทึก

### `GET /years`

Response `200 OK`:

```json
[
  {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40002",
    "year": 2569,
    "is_current": true,
    "current_semester": 1
  }
]
```

เรียงจากปีล่าสุดไปเก่าสุด

### `GET /divisions`

Response `200 OK` เป็น array ของ Division ที่มี `sort_order` และเรียงตามค่านี้

### `GET /divisions/{code}/topics`

Query:

| ชื่อ | Required | ความหมาย |
| --- | --- | --- |
| `year` | ไม่ | ปี พ.ศ. ถ้าไม่ส่งจะใช้ปีปัจจุบัน |

Response `200 OK`:

```json
{
  "division": {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30001",
    "code": "aca",
    "name": "ฝ่ายวิชาการ",
    "short_name": "วิชาการ",
    "sort_order": 1
  },
  "academic_year": {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40002",
    "year": 2569,
    "is_current": true
  },
  "topics": [
    {
      "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40001",
      "semester": 1,
      "title": "รายงานประจำเดือน",
      "due_date": "2026-11-30",
      "is_open": true,
      "is_secret": false,
      "created_by_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30004",
      "submitted_count": 1,
      "assignee_count": 2,
      "is_assigned": true,
      "my_status": "pending",
      "my_submitted_at": null,
      "closed_at": null,
      "created_at": "2026-11-07T08:00:00Z",
      "updated_at": "2026-11-07T08:00:00Z"
    }
  ]
}
```

ผู้ที่ไม่อยู่ในรายชื่อผู้ต้องส่งจะได้ `is_assigned: false`, `my_status: null` และ `my_submitted_at: null`

## 3. จัดการหัวข้อ

### `POST /topics`

ผู้เรียกต้องเป็นแอดมินหรือหัวหน้าฝ่ายที่ตรงกับ `division_uuid`

Request:

```json
{
  "division_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30001",
  "title": "รายงานผลการดำเนินงานประจำเดือน",
  "due_date": "2026-11-30",
  "is_secret": false,
  "assignee_uuids": [
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb30008"
  ],
  "fields": [
    {
      "position": 1,
      "type": "text",
      "label": "ชื่อกิจกรรม",
      "required": true,
      "options": []
    },
    {
      "position": 2,
      "type": "file",
      "label": "หลักฐาน",
      "required": false,
      "options": []
    }
  ]
}
```

กฎสำคัญ:

- `title` ยาวไม่เกิน 200 ตัวอักษร
- ต้องมี field 1-100 ช่อง; label ยาวไม่เกิน 120 ตัวอักษรและ position ห้ามซ้ำ
- `required` มีค่าเริ่มต้นเป็น `true`
- `choice` ต้องมีตัวเลือกไม่ซ้ำ 2-100 ค่า แต่ละค่าไม่เกิน 120 ตัวอักษร
- `assignee_uuids` ต้องมีอย่างน้อย 1 UUID และห้ามซ้ำ
- ผู้ต้องส่งที่เลือกต้องเป็นบัญชีที่เปิดใช้งาน ไม่ใช่ผู้บริหาร และเป็นสมาชิกฝ่ายของหัวข้อ
- หากไม่ส่ง `assignee_uuids` ระบบเลือกสมาชิกฝ่ายที่เข้าเงื่อนไขทั้งหมดเพื่อรองรับ client รุ่นเดิม
- ปีและภาคเรียนมาจากปีการศึกษาปัจจุบัน

Response `200 OK`: Topic พร้อม `assignee_uuids`

กรณีผิดพลาดที่สำคัญ: ไม่พบฝ่ายหรือปีปัจจุบัน, ไม่มีผู้ต้องส่งที่เข้าเงื่อนไข หรือข้อมูลไม่ผ่าน validation

### `PATCH /topics/{uuid}`

ส่งเฉพาะ property ที่ต้องแก้ได้:

```json
{
  "title": "รายงานฉบับแก้ไข",
  "due_date": "2026-12-15",
  "is_secret": true,
  "assignee_uuids": [
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007"
  ]
}
```

- ต้องส่งอย่างน้อยหนึ่งใน `title`, `due_date`, `is_secret`, `fields`, `assignee_uuids`
- `assignee_uuids` เป็นรายชื่อฉบับเต็ม: UUID ที่หายไปจะถูกถอด และ UUID ใหม่จะถูกเพิ่ม
- UUID ที่เพิ่มใหม่ต้องเป็นสมาชิกฝ่ายที่เปิดใช้งานและไม่ใช่ผู้บริหาร ส่วนผู้ที่อยู่ในรายชื่อเดิมสามารถคงไว้ได้แม้สถานะสมาชิกเปลี่ยนภายหลัง
- ถอดผู้ที่มี submission แล้วไม่ได้และตอบ `409`
- ถ้าไม่ส่ง `assignee_uuids` รายชื่อเดิมจะไม่เปลี่ยน
- ถ้าส่ง `fields` ต้องส่งฟอร์มฉบับเต็ม ช่องเดิมต้องมี `uuid`; ช่องใหม่ไม่ส่ง `uuid`; ช่องเดิมที่ไม่อยู่ในรายการจะถูกลบหรือซ่อนหากมีคำตอบแล้ว
- เปลี่ยน `type` ของ field หลังมี submission ไม่ได้และตอบ `409`
- ช่องเดิมที่ไม่ส่ง `required` จะคงค่าเดิม ช่องใหม่ใช้ `true`

Response `200 OK`: Topic ล่าสุดพร้อม `assignee_uuids`

### `POST /topics/{uuid}/close`

ไม่ใช้ request body ปิดรับคำตอบและคืน Topic ล่าสุดเป็น `200 OK`

### `POST /topics/{uuid}/reopen`

ไม่ใช้ request body เปิดรับคำตอบอีกครั้งและคืน Topic ล่าสุดเป็น `200 OK` ค่า `closed_at` ยังคงเวลาปิดล่าสุด

### `DELETE /topics/{uuid}`

Response `200 OK` ใช้ success envelope แบบไม่มี `data`; หัวข้อที่มี submission แล้วลบไม่ได้และตอบ `409`

## 4. การส่งคำตอบและตารางคำตอบ

### `GET /me/pending`

คืนหัวข้อที่เปิดรับ ซึ่งผู้ใช้ถูกกำหนดให้ส่งแต่ยังไม่มี submission เรียงตามกำหนดส่ง

Response `200 OK`:

```json
[
  {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40001",
    "division": {},
    "academic_year": {"uuid":"0199d6d0-bec9-7aa0-b2d8-96dc6fb40002","year":2569},
    "semester": 1,
    "title": "รายงานประจำเดือน",
    "due_date": "2026-11-30",
    "is_secret": false,
    "status": "pending",
    "created_by_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30004"
  }
]
```

`status` ใน endpoint นี้เป็น `pending` หรือ `overdue`

### `GET /topics/{uuid}/my-submission`

ผู้ใช้ต้องอยู่ในรายชื่อผู้ต้องส่ง

Response `200 OK`:

```json
{
  "topic": {},
  "submission": {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb50001",
    "values": {},
    "files": [
      {
        "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb50002",
        "field_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40004",
        "original_name": "evidence.pdf",
        "mime_type": "application/pdf",
        "size_bytes": 10240,
        "position": 1
      }
    ],
    "status": "submitted",
    "submitted_at": "2026-11-07T08:00:00Z",
    "updated_at": "2026-11-07T08:00:00Z"
  }
}
```

ถ้ายังไม่เคยส่ง `submission` เป็น `null`

### `PUT /topics/{uuid}/my-submission`

Request ใช้รูปแบบ `values` ตามตารางชนิด field ด้านบน:

```json
{
  "values": {
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb40003": "คำตอบ",
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb40004": [
      "0199d6d0-bec9-7aa0-b2d8-96dc6fb50002"
    ]
  }
}
```

Response `200 OK` มีรูปแบบเดียวกับ `GET /topics/{uuid}/my-submission`

- สร้างเมื่อยังไม่มี submission และแก้ของเดิมเมื่อมีแล้ว
- `submitted_at` เก็บเวลาส่งครั้งแรกและไม่เปลี่ยนเมื่อแก้
- ต้องเป็นผู้ต้องส่งและหัวข้อต้องเปิดอยู่
- field ที่ required ต้องมีค่าถูกชนิด
- validation ไม่ผ่านตอบ `400` ด้วย error envelope มาตรฐาน
- ไฟล์ staged ที่อ้างถึงจะเปลี่ยนเป็น attached; ไฟล์เดิมที่ถอดออกจะถูก trashed

กรณีผิดพลาด: ไม่พบหัวข้อ, หัวข้อปิด, ไม่มีสิทธิ์ หรือข้อมูลไม่ผ่าน validation

### `GET /topics/{uuid}`

คืน Topic และหนึ่งแถวต่อผู้ต้องส่ง

Query:

| `status` | ความหมาย |
| --- | --- |
| ไม่ส่ง | ทุกสถานะ |
| `done` | `submitted` และ `late` |
| `pending` | `pending`, `overdue` และ `not_submitted` |

Response `200 OK`:

```json
{
  "topic": {},
  "rows": [
    {
      "user": {
        "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
        "username": "teacher.one",
        "prefix": "นาง",
        "first_name": "สมหญิง",
        "last_name": "ใจดี",
        "full_name": "นางสมหญิง ใจดี"
      },
      "submission_uuid": null,
      "values": {
        "0199d6d0-bec9-7aa0-b2d8-96dc6fb40003": null
      },
      "files": [],
      "status": "pending",
      "submitted_at": null,
      "updated_at": null
    }
  ]
}
```

หัวข้อทั่วไปเปิดให้ผู้ใช้ที่ login เห็นทุกแถว หัวข้อลับเปิดทุกแถวเฉพาะผู้บริหาร หัวหน้าฝ่าย และผู้สร้าง ผู้ต้องส่งทั่วไปเห็นเฉพาะแถวตนเอง ส่วนผู้ที่ไม่ถูกกำหนดจะได้ `rows: []`

### `GET /topics/{uuid}/pending-names`

ใช้ได้เฉพาะผู้แก้หัวข้อ คืน array ของ user object แบบเดียวกับ `rows[].user` สำหรับผู้ต้องส่งที่ยังไม่มี submission

## 5. อัปโหลด เปิดดู และลบไฟล์

### `POST /uploads`

Content-Type: `multipart/form-data`

| Field | ชนิด | ความหมาย |
| --- | --- | --- |
| `topic_uuid` | UUID | หัวข้อปลายทาง |
| `field_uuid` | UUID | field ชนิด `file` |
| `file` | file | หนึ่งไฟล์ ขนาด 1 byte ถึง 50 MiB |

ตัวอย่าง:

```bash
curl -b cookies.txt \
  -F "topic_uuid=0199d6d0-bec9-7aa0-b2d8-96dc6fb40001" \
  -F "field_uuid=0199d6d0-bec9-7aa0-b2d8-96dc6fb40004" \
  -F "file=@evidence.pdf" \
  http://localhost:8081/api/uploads
```

Response `200 OK`:

```json
{
  "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb50002",
  "topic_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40001",
  "field_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40004",
  "original_name": "evidence.pdf",
  "mime_type": "application/pdf",
  "size_bytes": 10240,
  "status": "staged",
  "created_at": "2026-11-07T08:00:00Z"
}
```

ไฟล์ staged ที่ไม่ถูกแนบกับ submission จะหมดอายุตามค่าตั้งต้น 24 ชั่วโมง

กรณีผิดพลาด: ไฟล์ใหญ่เกิน/ว่าง, ไม่พบหัวข้อหรือ field, field ไม่ใช่ชนิดไฟล์, หัวข้อปิด หรือไม่มีสิทธิ์

### `DELETE /uploads/{uuid}`

ลบได้เฉพาะไฟล์ staged ของผู้ใช้เอง Response `200 OK` ใช้ success envelope แบบไม่มี `data`; ไฟล์ attached ต้องถอดผ่านการแก้ submission และจะตอบ `409`

### `GET /files/{uuid}`

Query `download=true` บังคับ `Content-Disposition: attachment` ถ้าไม่ส่ง ระบบแสดง PDF, รูป raster และวิดีโอแบบ inline ส่วน SVG และชนิดอื่นดาวน์โหลดเป็นค่าเริ่มต้น

Response เป็น binary stream พร้อม header:

- `Content-Type`
- `Content-Disposition`
- `Cache-Control: private, no-store`
- `X-Content-Type-Options: nosniff`

เปิดได้เฉพาะไฟล์สถานะ attached และใช้ scope เดียวกับตารางคำตอบ หัวข้อลับที่ไม่มีสิทธิ์ตอบ `403`; UUID ที่ไม่มีหรือเป็น staged/trashed ตอบ `404`

## 6. หน้าแรกและสถิติ

### `GET /home`

Response `200 OK`:

```json
{
  "my": {
    "counts": {
      "topic_count": 4,
      "required_count": 4,
      "submitted_count": 2,
      "pending_count": 1,
      "overdue_count": 1
    },
    "pending_topics": [],
    "recent_submissions": [],
    "divisions": [
      {
        "division": {},
        "counts": {}
      }
    ]
  },
  "overview": {
    "school": {},
    "divisions": []
  }
}
```

- `pending_topics[]`: `uuid`, `title`, `division`, `academic_year`, `semester`, `due_date`, `is_secret`, `status`
- `recent_submissions[]`: `uuid`, `topic_uuid`, `title`, `division`, `academic_year`, `semester`, `due_date`, `status`, `submitted_at`, `updated_at`; สูงสุด 5 รายการ
- `overview` มีเฉพาะผู้บริหารและหัวหน้าฝ่าย ผู้ใช้อื่นได้ `null`
- counts ทั้งหมดนับเฉพาะหัวข้อที่เปิดอยู่

### `GET /divisions/{code}/stats`

ผู้บริหารดูได้ทุกฝ่าย หัวหน้าฝ่ายดูได้เฉพาะฝ่ายตน

Response `200 OK`:

```json
{
  "division": {},
  "stats": {
    "topic_count": 4,
    "required_count": 10,
    "submitted_count": 7,
    "pending_count": 2,
    "overdue_count": 1
  }
}
```

กรณีผิดพลาด: ไม่มีสิทธิ์ตอบ `403`; ไม่พบฝ่ายตอบ `404`

## 7. แฟ้มของฉัน

### `GET /me/submissions`

Query:

| ชื่อ | Required | ความหมาย |
| --- | --- | --- |
| `year` | ไม่ | ปี พ.ศ. |
| `q` | ไม่ | ค้นบางส่วนในชื่อหัวข้อ, JSON คำตอบ และชื่อไฟล์ |

Response `200 OK`:

```json
{
  "years": [
    {
      "academic_year": 2569,
      "submissions": [
        {
          "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb50001",
          "topic_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40001",
          "topic_title": "รายงานประจำเดือน",
          "division": {},
          "semester": 1,
          "due_date": "2026-11-30",
          "is_open": true,
          "is_secret": false,
          "values": {},
          "status": "submitted",
          "submitted_at": "2026-11-07T08:00:00Z",
          "updated_at": "2026-11-07T08:00:00Z",
          "files": [
            {
              "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb50002",
              "field_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40004",
              "original_name": "evidence.pdf",
              "mime_type": "application/pdf",
              "size_bytes": 10240,
              "position": 1,
              "created_at": "2026-11-07T08:00:00Z"
            }
          ]
        }
      ]
    }
  ]
}
```

รวมประวัติจากหัวข้อที่ปิดแล้วและเรียงปีล่าสุดก่อน

### `GET /me/files.zip`

Query `year` เป็นปี พ.ศ. และไม่บังคับ หากไม่ส่งจะรวมทุกปี

Response `200 OK` เป็น `application/zip` แบบ stream พร้อม `Content-Disposition: attachment` ภายในจัดไฟล์ใต้โฟลเดอร์ `ปี/ฝ่าย/หัวข้อ/`

## 8. ค้นหา

### `GET /search`

Query:

| ชื่อ | ค่าเริ่มต้น | กฎ |
| --- | --- | --- |
| `q` | ว่าง | ไม่เกิน 200 ตัวอักษร |
| `division` | ทุกฝ่าย | รหัสฝ่าย ไม่เกิน 8 ตัวอักษร |
| `year` | ทุกปี | ปี พ.ศ. 2400-3000 |
| `semester` | ทุกภาค | `1` หรือ `2` |
| `by` | ทุกคน | UUID ของผู้ส่งจาก `/users` |
| `page` | `1` | 1-1,000,000 |
| `page_size` | `20` | 1-100 |

`q` ค้นในชื่อหัวข้อ ฝ่าย ผู้ส่ง ค่า JSON และชื่อไฟล์แบบไม่แยกตัวพิมพ์ รองรับภาษาไทย

Response `200 OK`:

```json
{
  "items": [
    {
      "submission_uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb50001",
      "topic": {
        "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40001",
        "title": "รายงานประจำเดือน",
        "division": {},
        "academic_year": 2569,
        "semester": 1,
        "due_date": "2026-11-30",
        "is_open": true,
        "is_secret": false
      },
      "submitted_by": {
        "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
        "username": "teacher.one",
        "prefix": "นาง",
        "first_name": "สมหญิง",
        "last_name": "ใจดี",
        "full_name": "นางสมหญิง ใจดี"
      },
      "values": {},
      "files": [],
      "status": "submitted",
      "submitted_at": "2026-11-07T08:00:00Z",
      "updated_at": "2026-11-07T08:00:00Z"
    }
  ],
  "topics": [],
  "topic_total": 0,
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

`topics` คืนหัวข้อที่ชื่อตรง `q` สูงสุด 20 รายการและใช้ schema เดียวกับ `items[].topic`; ถ้าไม่ส่ง `q` จะเป็น `[]` การนับและ pagination เกิดหลังตรวจ scope จึงไม่เปิดเผยจำนวนข้อมูลลับ ผู้ใช้ทั่วไปเห็นคำตอบลับเฉพาะของตน

## 9. จัดการผู้ใช้สำหรับแอดมิน

### `GET /admin/users`

Response `200 OK` เป็น array:

```json
[
  {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
    "username": "teacher.one",
    "prefix": "นาง",
    "first_name": "สมหญิง",
    "last_name": "ใจดี",
    "full_name": "นางสมหญิง ใจดี",
    "is_exec": false,
    "is_admin": false,
    "is_active": true,
    "divisions": [],
    "deactivated_at": null,
    "created_at": "2026-11-07T08:00:00Z",
    "updated_at": "2026-11-07T08:00:00Z"
  }
]
```

### `POST /admin/users`

Request:

```json
{
  "username": "teacher.one",
  "password": "safe-password",
  "prefix": "นาง",
  "first_name": "สมหญิง",
  "last_name": "ใจดี",
  "division_uuids": [
    "0199d6d0-bec9-7aa0-b2d8-96dc6fb30001"
  ],
  "is_exec": false,
  "is_admin": false
}
```

- username ยาว 3-32 ตัว ใช้ตัวพิมพ์เล็ก ตัวเลข `.`, `_`, `-`
- password อย่างน้อย 8 ตัวอักษรและไม่เกิน 72 bytes
- `prefix` ไม่เกิน 30 ตัวอักษร, `first_name` บังคับและไม่เกิน 120, `last_name` ไม่เกิน 120
- ยังรับ `full_name` เพื่อรองรับ client เก่า แต่ห้ามส่งพร้อม `prefix`, `first_name` หรือ `last_name`

Response `200 OK`: admin user object

กรณีผิดพลาด: ข้อมูลไม่ผ่าน validation, ไม่พบฝ่าย หรือ username ซ้ำ (`409`)

### `PATCH /admin/users/{uuid}`

ส่งอย่างน้อยหนึ่ง property:

```json
{
  "username": "teacher.updated",
  "prefix": "นาง",
  "first_name": "สมหญิง",
  "last_name": "ใจดี",
  "division_uuids": [],
  "is_exec": false,
  "is_admin": false,
  "is_active": true
}
```

Response `200 OK`: admin user object ล่าสุด ใช้ `is_active: false` เพื่อปิดบัญชีโดยไม่ลบประวัติ

### `POST /admin/users/{uuid}/password`

Request:

```json
{"password":"new-safe-password"}
```

Response `200 OK`:

```json
{"status":200,"message":"success"}
```

การตั้งรหัสผ่านใหม่ยกเลิก session เดิมทั้งหมดของผู้ใช้นั้น

### `POST /admin/users/import`

Content-Type: `multipart/form-data` ใช้ field ชื่อ `file` รองรับ `.csv` และ `.xlsx` ขนาดไม่เกิน 3 MiB และไม่เกิน 2,000 แถว

หัวตาราง:

| คำนำหน้า | ชื่อ | นามสกุล | username | รหัสผ่านเริ่มต้น | ฝ่าย |
| --- | --- | --- | --- | --- | --- |

คอลัมน์ฝ่ายรับรหัส ชื่อเต็ม หรือชื่อย่อ หลายฝ่ายคั่นด้วย `|`, `,` หรือ `;` ผู้ใช้ที่นำเข้าเป็นบุคลากรทั่วไป ไม่เป็นผู้บริหารหรือแอดมิน

Response `200 OK`:

```json
{"imported":25}
```

หากมีข้อมูลผิดจะไม่บันทึกทั้งไฟล์:

```json
{
  "status": 400,
  "message": "Import file contains invalid rows"
}
```

กรณีผิดพลาดอื่น: ไม่ได้ส่งไฟล์, ไฟล์ใหญ่เกิน, อ่านไฟล์ไม่ได้ หรือชนิดไฟล์ไม่รองรับ

### `GET /admin/users/import/template.csv`

Response `200 OK` เป็น binary attachment ชนิด `text/csv` ชื่อ `sdms-users-template.csv`

### `GET /admin/users/import/template.xlsx`

Response `200 OK` เป็น binary XLSX attachment ชื่อ `sdms-users-template.xlsx`

## 10. ฝ่ายและปีการศึกษาสำหรับแอดมิน

### `GET /admin/divisions`

Response `200 OK`:

```json
[
  {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30001",
    "code": "aca",
    "name": "ฝ่ายวิชาการ",
    "short_name": "วิชาการ",
    "sort_order": 1,
    "head_user": {
      "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb30007",
      "username": "teacher.one",
      "prefix": "นาง",
      "first_name": "สมหญิง",
      "last_name": "ใจดี",
      "full_name": "นางสมหญิง ใจดี"
    }
  }
]
```

ฝ่ายที่ยังไม่มีหัวหน้าจะได้ `head_user: null`

### `PUT /admin/divisions/{code}/head`

Request:

```json
{"user_uuid":"0199d6d0-bec9-7aa0-b2d8-96dc6fb30007"}
```

ผู้ใช้ต้องเปิดใช้งาน ไม่ใช่ผู้บริหาร และยังไม่เป็นหัวหน้าฝ่ายอื่น Response `200 OK` เป็น admin division object ล่าสุด

กรณีผิดพลาด: ไม่พบฝ่าย/ผู้ใช้, ผู้ใช้เป็นผู้บริหาร, บัญชีถูกปิด หรือเป็นหัวหน้าฝ่ายอื่นอยู่แล้ว

### `GET /admin/years`

Response `200 OK` เรียงปีล่าสุดก่อน:

```json
[
  {
    "uuid": "0199d6d0-bec9-7aa0-b2d8-96dc6fb40002",
    "year": 2569,
    "is_current": true,
    "current_semester": 1,
    "topic_count": 10,
    "created_at": "2026-05-01T00:00:00Z"
  }
]
```

### `POST /admin/years`

Request:

```json
{
  "year": 2570,
  "current_semester": 1
}
```

`year` ต้องอยู่ระหว่าง 2400-3000 และ semester เป็น 1 หรือ 2 หากไม่ส่ง semester ใช้ 1 ปีใหม่จะยังไม่เป็นปีปัจจุบัน

Response `200 OK`: academic year object แบบเดียวกับ `/admin/years`

กรณีผิดพลาด: ข้อมูลไม่ผ่าน validation หรือปีการศึกษาซ้ำ (`409`)

### `POST /admin/years/{year}/current`

Body ไม่บังคับ:

```json
{"current_semester":2}
```

ถ้าไม่ส่ง body จะใช้ semester เดิมของปีนั้น ตั้งสำเร็จแล้วปีอื่นจะถูกเปลี่ยนเป็น `is_current: false`

Response `200 OK`: academic year object ล่าสุด

กรณีผิดพลาด: ปีหรือภาคเรียนไม่ถูกต้อง หรือไม่พบปีการศึกษา

## HTTP status เฉพาะโดเมน

error response จะไม่มี field `code` หรือ `details`; client ใช้ HTTP status ร่วมกับ `message`

| HTTP | กรณี |
| --- | --- |
| `400` | UUID/ปี/ฝ่ายไม่ถูกต้อง, field ไม่ใช่ชนิดไฟล์ หรือ import ไม่ใช่ CSV/XLSX |
| `404` | ไม่พบผู้ใช้ ฝ่าย ปีการศึกษา หัวข้อ field หรือไฟล์ |
| `409` | username/ปีซ้ำ, ยังไม่มีปีปัจจุบัน, ไม่มีผู้ต้องส่ง, พยายามถอดผู้ที่ส่งแล้ว, เปลี่ยนชนิด field หลังมีคำตอบ, ลบหัวข้อที่มีคำตอบ, ส่งหลังปิดหัวข้อ หรือลบไฟล์ attached โดยตรง |

## Flow แนะนำสำหรับ frontend

### สร้างหรือแก้หัวข้อ

1. โหลด `GET /divisions` และ `GET /users`
2. กรองผู้ใช้ที่ `is_topic_assignee_eligible = true` และมี division ตรงกับหัวข้อ
3. ส่ง UUID ที่เลือกทั้งหมดใน `assignee_uuids` ตอน `POST /topics`
4. หน้าแก้ไขโหลด `GET /topics/{uuid}` และตั้งค่าตัวเลือกจาก `topic.assignee_uuids`
5. ตอนบันทึก `PATCH /topics/{uuid}` ให้ส่งรายชื่อฉบับเต็ม
6. หากได้ `409` ตอนแก้รายชื่อ ให้โหลดข้อมูลใหม่และไม่อนุญาตให้ถอดผู้ที่ส่งแล้ว

### ส่งคำตอบที่มีไฟล์

1. โหลด `GET /topics/{uuid}/my-submission`
2. อัปโหลดแต่ละไฟล์ด้วย `POST /uploads`
3. นำ `uuid` ของไฟล์ใส่ใน array ของ field ชนิด `file`
4. ส่ง `PUT /topics/{uuid}/my-submission`
5. เมื่อแก้ submission ให้ส่ง UUID ของไฟล์ที่ต้องการคงไว้ทั้งหมด
