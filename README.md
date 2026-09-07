# SDMS Backend — Phase 1

Backend สำหรับ **School Document Management System (SDMS)** ตามขอบเขต Phase 1 ของเอกสารนำเสนอโครงการ: โรงเรียนกำหนดหัวข้อและแบบฟอร์ม ครูส่งข้อมูลแบบมีโครงสร้างพร้อมหลักฐาน ผู้บริหารตรวจตามหัวข้อ/รายบุคคล และระบบติดตามได้ว่าใครยังไม่ส่งหรือส่งไม่ครบ

> Phase 1 ไม่ได้พยายามแทน Google Drive และยังไม่รวม Dashboard, Report/Export, Notification หรือ Advanced Search ซึ่งอยู่ใน Phase 2 ของข้อเสนอโครงการ

## Tech stack

- Go **1.26.5** (ตาม `go.mod`)
- Fiber v3
- GORM
- PostgreSQL
- JWT + bcrypt
- Local evidence storage (เปลี่ยน storage implementation ภายหลังได้)

## Phase-1 capabilities

- Authentication: login, JWT, `/auth/me`
- Roles: `ADMIN`, `DIRECTOR`, `QA`, `TEACHER`
- User administration: `ADMIN`
- Topic/Form management: `ADMIN`, `DIRECTOR`
- Read-only review: `ADMIN`, `DIRECTOR`, `QA`
- Teacher-owned submissions/evidence: `TEACHER`
- Academic-year separation (`academic_year`)
- Flexible fields: text, textarea, number, date, select, file
- Unlimited preview fields (`is_preview`)
- Submission create/update/delete for owner
- Required-field validation including **required evidence files**
- Completion states: `NOT_SUBMITTED`, `INCOMPLETE`, `COMPLETE`
- Topic submission-status API covering every active teacher
- Form version + per-submission `form_snapshot`
- File upload/view/download/delete and video byte-range streaming
- Audit log for authenticated API access
- Local PostgreSQL + upload backup/restore scripts
- Production CORS allow-list and production JWT-secret guard

## Project structure

```text
cmd/
  api/                 API entry point + prototype HTML
  seed-admin/          create the first administrator
  seed-director/       create a director account
internal/
  config/
  modules/
    auth/
    health/
    submission/
    topic/
    user/
  platform/
    audit/
    auth/
    database/
    http/
    storage/local/
docs/
  PHASE1_API.md
scripts/
  backup.sh
  restore.sh
```

The code keeps business rules in Domain/Usecase layers and database/Fiber concerns in Repository/Delivery/Platform layers.

## Setup

```bash
cp .env.example .env
docker compose up -d
go mod tidy
go run ./cmd/api
```

Base URL:

```text
http://localhost:8080/api/v1
```

Health check:

```bash
curl http://localhost:8080/api/v1/health
```

## Bootstrap the first administrator

Because account mutation is an `ADMIN` responsibility in the Phase-1 role model, create the first admin from the command line:

```bash
go run ./cmd/seed-admin \
  -username admin \
  -password 'change-this-password' \
  -employee-code ADM001 \
  -prefix นาย \
  -first-name ผู้ดูแล \
  -last-name ระบบ
```

A director can also be seeded directly:

```bash
go run ./cmd/seed-director \
  -username director \
  -password 'change-this-password' \
  -employee-code DIR001 \
  -prefix นาย \
  -first-name ผู้อำนวยการ \
  -last-name โรงเรียน
```

## Role matrix

| Capability | ADMIN | DIRECTOR | QA | TEACHER |
|---|:---:|:---:|:---:|:---:|
| Manage user accounts | ✅ | ❌ | ❌ | ❌ |
| Read user identity for review | ✅ | ✅ | ✅ | ❌ |
| Create/update topic & form | ✅ | ✅ | ❌ | ❌ |
| Review all submissions/evidence | ✅ | ✅ | ✅ | ❌ |
| View submission-status tracking | ✅ | ✅ | ✅ | ❌ |
| Create/update/delete own submission | ❌ | ❌ | ❌ | ✅ |
| Upload/delete own evidence | ❌ | ❌ | ❌ | ✅ |
| Read audit log | ✅ | ❌ | ❌ | ❌ |

## Academic year and form history

A new topic requires `academic_year` and starts at `form_version = 1`.

> **Existing database migration:** `AutoMigrate` adds the new `academic_year` column with `UNSPECIFIED` for rows that existed before this revision. Before pilot/production use, update those old topics to the correct academic year. Existing submissions receive `form_version = 1`; their old values remain readable, but only submissions created/updated after this revision have a full historical `form_snapshot`.

```json
{
  "academic_year": "2569",
  "name": "งานอบรมและพัฒนาตนเอง",
  "description": "หลักฐานปีการศึกษา 2569"
}
```

Use:

```http
GET /api/v1/topics?academic_year=2569
```

When a teacher submits, the submission records both `form_version` and a full `form_snapshot`. After submissions exist, changes that could invalidate historical data are blocked: field deletion, type changes, required/select-option changes, adding a new required field, changing the topic's academic year, or deleting the topic.

## Submission status tracking

Reviewers can call:

```http
GET /api/v1/topics/:topicID/submissions/status
```

Example states:

```text
ครู A  COMPLETE
ครู B  INCOMPLETE
ครู C  NOT_SUBMITTED
```

`INCOMPLETE` includes missing required file fields; merely creating a submission is no longer treated as complete when evidence is still missing.

Phase 1 currently treats **all active TEACHER accounts** as expected submitters for a topic. Per-topic teacher/group assignment can be added later if the school requires different target groups for different topics.

## Teacher submission management

```http
POST   /api/v1/topics/:topicID/submissions
PUT    /api/v1/topics/:topicID/submissions/:submissionID
DELETE /api/v1/topics/:topicID/submissions/:submissionID
```

Ownership is enforced from the authenticated JWT user; the client cannot impersonate another teacher by supplying a different `submitted_by`.

## Evidence files

Supported extensions:

```text
.pdf .doc .docx .xls .xlsx .ppt .pptx .csv .txt
.png .jpg .jpeg .webp
.mp4 .webm .mov .m4v
```

View/download endpoints retain authentication/ownership checks and `/view` supports byte ranges for browser video playback.

## Audit log

Authenticated API access is written to `audit_logs` with user ID, method, path, HTTP status, IP and timestamp.

```http
GET /api/v1/audit-logs?limit=100
GET /api/v1/audit-logs?user_uid=<uuid>&method=GET
```

Only `ADMIN` can query the audit-log endpoint.

## Backup and restore

For the current local-file deployment model:

```bash
./scripts/backup.sh
./scripts/restore.sh backups/<timestamp>
```

A backup contains a PostgreSQL custom dump plus the configured `UPLOAD_DIR`. Test restoration periodically before relying on it for production recovery.

## Security notes

- Do not distribute or commit `.env` containing real credentials.
- Set `CORS_ALLOWED_ORIGINS` to the real frontend origin(s) in production.
- Set a long random `AUTH_JWT_SECRET`. The API refuses to start in `APP_ENV=production` when the value is a known placeholder or shorter than 32 characters.
- Upload validation currently uses the allowed extension list plus the browser/server content type; add file-signature (magic-byte) validation and login rate limiting before a public production rollout.
- Local storage is appropriate for pilot/single-instance use; production deployments should have persistent storage, backup monitoring, and a tested recovery process.

## Development commands

```bash
docker compose up -d
go fmt ./...
go test ./...
go build ./...
go run ./cmd/api
```

See [`docs/PHASE1_API.md`](docs/PHASE1_API.md) for the changed API behavior.

## Phase 2 (not implemented in this revision)

The project proposal places these features in Phase 2:

- Dashboard ภาพรวม
- Report / Export
- Notification
- Advanced cross-topic Search / Filter

They are intentionally left out of this Phase-1 backend revision rather than being silently treated as missing Phase-1 work.
