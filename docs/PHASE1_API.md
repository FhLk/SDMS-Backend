# SDMS Phase 1 API additions

This document describes the Phase-1 behavior added to match the project proposal.

## Roles

- `ADMIN`: account administration, audit-log access, and system administration.
- `DIRECTOR`: manages topics/forms and reviews submissions.
- `QA`: read-only review of users/submissions/evidence.
- `TEACHER`: creates, edits, deletes, and views only their own submissions/evidence.

## Academic year and form version

Create a topic with `academic_year`:

```http
POST /api/v1/topics
Authorization: Bearer <token>
Content-Type: application/json

{
  "academic_year": "2569",
  "name": "หลักฐานงานประกันคุณภาพ",
  "description": "หลักฐานประจำปีการศึกษา 2569"
}
```

Filter topics by year:

```http
GET /api/v1/topics?academic_year=2569
```

Each topic has `form_version`. Each submission stores both the version number and a `form_snapshot`, so the form structure used when the teacher submitted remains available historically.

After submissions exist, validation-affecting form changes are blocked. A required field cannot be added, an existing field cannot be deleted, and type/required/select-option changes return HTTP `409 Conflict`. This prevents historical submissions from becoming invalid.

## Existing-database migration note

`AutoMigrate` adds the new columns/tables without deleting current submissions. Existing topics receive `academic_year = "UNSPECIFIED"` and should be corrected before pilot/production use. Existing submissions remain readable with `form_version = 1`; a full `form_snapshot` is guaranteed for submissions created or updated after this revision.

## Teacher submission management

```http
POST   /api/v1/topics/:topicID/submissions
PUT    /api/v1/topics/:topicID/submissions/:submissionID
DELETE /api/v1/topics/:topicID/submissions/:submissionID
```

Teachers can only update/delete their own submissions. Structured values are replaced on `PUT`; already-uploaded evidence remains attached unless the teacher deletes/replaces the evidence separately.

## Completion status

Submission responses now include:

```json
{
  "completion_status": "INCOMPLETE",
  "missing_required_fields": [
    {"field_uid":"...", "label":"ไฟล์หลักฐาน", "type":"file"}
  ]
}
```

A required file field is not considered complete until an evidence file has actually been uploaded.

## Director/QA tracking

```http
GET /api/v1/topics/:topicID/submissions/status
```

The endpoint returns every active `TEACHER` account and one of:

- `NOT_SUBMITTED`
- `INCOMPLETE`
- `COMPLETE`

Phase 1 treats every active teacher as an expected submitter for every topic. Per-topic teacher/group assignment is not part of the proposal's Phase-1 feature list and is not introduced here.

## Evidence files

Supported extensions:

`.pdf .doc .docx .xls .xlsx .ppt .pptx .csv .txt .png .jpg .jpeg .webp .mp4 .webm .mov .m4v`

The existing inline view/download endpoints remain available and video byte-range streaming is preserved.

## Audit trail

Authenticated requests under the protected domain API group (topics, users, submissions, evidence, and audit-log access) are recorded in `audit_logs` with user, method, path, status, IP address and timestamp. `ADMIN` can inspect recent entries:

```http
GET /api/v1/audit-logs?limit=100
GET /api/v1/audit-logs?user_uid=<uuid>&method=GET
```

## Backup / restore

For deployments that keep evidence in local storage:

```bash
./scripts/backup.sh
./scripts/restore.sh backups/<timestamp>
```

The backup contains both a PostgreSQL custom-format dump and the local upload directory.

## Intentionally left for Phase 2

The proposal explicitly places Dashboard, report/export, notifications, and advanced cross-topic search/filter in Phase 2. They are therefore not implemented as Phase-1 backend requirements in this revision.
