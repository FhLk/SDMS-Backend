package submission

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTopicSubmissionTableWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	// ข้อมูลจำลอง BE-13 ถูก rollback ทั้งชุด จึงไม่เปลี่ยนข้อมูลจริงหรือ Docker volume.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var division model.Division
	if err := tx.Order("sort_order, code").First(&division).Error; err != nil {
		t.Fatalf("find division: %v", err)
	}
	var academicYear model.AcademicYear
	if err := tx.Where("is_current = ?", true).First(&academicYear).Error; err != nil {
		t.Fatalf("find current academic year: %v", err)
	}
	suffix := uuid.NewString()[:8]
	users := []model.User{
		{Username: "be13.creator." + suffix, PasswordHash: "hash", FullName: "ผู้สร้าง BE-13", Position: "หัวหน้าฝ่าย"},
		{Username: "be13.ontime." + suffix, PasswordHash: "hash", FullName: "ก ส่งทัน", Position: "ครู"},
		{Username: "be13.late." + suffix, PasswordHash: "hash", FullName: "ข ส่งช้า", Position: "ครู"},
		{Username: "be13.pending." + suffix, PasswordHash: "hash", FullName: "ค ยังไม่ส่ง", Position: "ครู"},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	assignees := users[1:]
	pastTopic, pastFields := createTableTopic(t, tx, division, academicYear, users[0], assignees, false, true, "2026-10-12", "ตาราง BE-13 "+suffix)

	onTimeAt := time.Date(2026, time.October, 12, 16, 59, 0, 0, time.UTC)
	lateAt := time.Date(2026, time.October, 12, 17, 0, 0, 0, time.UTC)
	submissions := []model.Submission{
		newTableSubmission(pastTopic.UUID, users[1].UUID, pastFields[0].UUID, "ส่งทัน", onTimeAt),
		newTableSubmission(pastTopic.UUID, users[2].UUID, pastFields[0].UUID, "ส่งช้า", lateAt),
	}
	if err := tx.Create(&submissions).Error; err != nil {
		t.Fatalf("create submissions: %v", err)
	}
	fileUUID := uuid.New()
	file := model.File{
		UUIDModel:      model.UUIDModel{UUID: fileUUID},
		TopicUUID:      pastTopic.UUID,
		FieldUUID:      pastFields[1].UUID,
		SubmissionUUID: &submissions[0].UUID,
		UploadedByUUID: users[1].UUID,
		ObjectKey:      "be13/attached.pdf",
		OriginalName:   "หลักฐาน.pdf",
		MimeType:       "application/pdf",
		SizeBytes:      10,
		Status:         model.FileAttached,
	}
	if err := tx.Create(&file).Error; err != nil {
		t.Fatalf("create attached file: %v", err)
	}

	service := NewService(tx, nil)
	service.now = func() time.Time {
		return time.Date(2026, time.October, 13, 12, 0, 0, 0, bangkokLocation)
	}
	regularProfile := auth.Profile{User: users[3]}
	creatorProfile := auth.Profile{User: users[0]}

	table, err := service.ListTopicTable(context.Background(), pastTopic.UUID, regularProfile, "")
	if err != nil {
		t.Fatalf("ListTopicTable() error = %v", err)
	}
	if len(table.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(table.Rows))
	}
	assertTableRow(t, table.Rows, users[1].UUID, StatusSubmitted, true, fileUUID)
	assertTableRow(t, table.Rows, users[2].UUID, StatusLate, true, uuid.Nil)
	assertTableRow(t, table.Rows, users[3].UUID, StatusOverdue, false, uuid.Nil)

	done, err := service.ListTopicTable(context.Background(), pastTopic.UUID, regularProfile, "done")
	if err != nil || len(done.Rows) != 2 {
		t.Fatalf("done rows = %d, error = %v", len(done.Rows), err)
	}
	pending, err := service.ListTopicTable(context.Background(), pastTopic.UUID, regularProfile, "pending")
	if err != nil || len(pending.Rows) != 1 || pending.Rows[0].User.UUID != users[3].UUID {
		t.Fatalf("pending table = %+v, error = %v", pending.Rows, err)
	}

	secretTopic, _ := createTableTopic(t, tx, division, academicYear, users[0], assignees, true, true, "2026-10-20", "หัวข้อลับ BE-13 "+suffix)
	secretOwn, err := service.ListTopicTable(context.Background(), secretTopic.UUID, regularProfile, "")
	if err != nil || len(secretOwn.Rows) != 1 || secretOwn.Rows[0].User.UUID != users[3].UUID {
		t.Fatalf("secret own rows = %+v, error = %v", secretOwn.Rows, err)
	}
	secretAll, err := service.ListTopicTable(context.Background(), secretTopic.UUID, creatorProfile, "")
	if err != nil || len(secretAll.Rows) != 3 {
		t.Fatalf("secret creator rows = %d, error = %v", len(secretAll.Rows), err)
	}
	if secretAll.Rows[0].Status != StatusPending {
		t.Fatalf("future open status = %q, want pending", secretAll.Rows[0].Status)
	}

	closedTopic, _ := createTableTopic(t, tx, division, academicYear, users[0], []model.User{users[3]}, false, false, "2026-10-12", "หัวข้อปิด BE-13 "+suffix)
	closed, err := service.ListTopicTable(context.Background(), closedTopic.UUID, regularProfile, "")
	if err != nil || len(closed.Rows) != 1 || closed.Rows[0].Status != StatusNotSubmitted {
		t.Fatalf("closed rows = %+v, error = %v", closed.Rows, err)
	}

	names, err := service.ListPendingNames(context.Background(), pastTopic.UUID, creatorProfile)
	if err != nil || len(names) != 1 || names[0].UUID != users[3].UUID {
		t.Fatalf("pending names = %+v, error = %v", names, err)
	}
	if _, err := service.ListPendingNames(context.Background(), pastTopic.UUID, regularProfile); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListPendingNames(teacher) error = %v, want ErrForbidden", err)
	}
}

func createTableTopic(t *testing.T, db *gorm.DB, division model.Division, academicYear model.AcademicYear, creator model.User, assignees []model.User, secret, open bool, dueDate, title string) (model.Topic, []model.TopicField) {
	t.Helper()
	due, err := time.Parse(dateLayout, dueDate)
	if err != nil {
		t.Fatalf("parse due date: %v", err)
	}
	topic := model.Topic{
		DivisionUUID:     division.UUID,
		AcademicYearUUID: academicYear.UUID,
		Semester:         academicYear.CurrentSemester,
		Title:            title,
		DueDate:          due,
		IsOpen:           open,
		IsSecret:         secret,
		CreatedByUUID:    creator.UUID,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("create topic: %v", err)
	}
	// GORM ใช้ default true แทน zero value ตอน Create จึงกำหนด false ชัดเจนสำหรับ fixture หัวข้อปิด.
	if !open {
		if err := db.Model(&topic).Update("is_open", false).Error; err != nil {
			t.Fatalf("close fixture topic: %v", err)
		}
		topic.IsOpen = false
	}
	fields := []model.TopicField{
		{TopicUUID: topic.UUID, Position: 1, Type: model.FieldText, Label: "รายละเอียด", Required: true, Options: datatypes.JSON([]byte(`[]`))},
		{TopicUUID: topic.UUID, Position: 2, Type: model.FieldFile, Label: "หลักฐาน", Required: false, Options: datatypes.JSON([]byte(`[]`))},
	}
	if err := db.Create(&fields).Error; err != nil {
		t.Fatalf("create fields: %v", err)
	}
	assignments := make([]model.TopicAssignee, 0, len(assignees))
	for _, user := range assignees {
		assignments = append(assignments, model.TopicAssignee{TopicUUID: topic.UUID, UserUUID: user.UUID})
	}
	if err := db.Create(&assignments).Error; err != nil {
		t.Fatalf("create assignees: %v", err)
	}
	return topic, fields
}

func newTableSubmission(topicUUID, userUUID, fieldUUID uuid.UUID, value string, submittedAt time.Time) model.Submission {
	values, _ := json.Marshal(map[string]any{fieldUUID.String(): value})
	return model.Submission{
		TopicUUID:   topicUUID,
		UserUUID:    userUUID,
		Values:      datatypes.JSON(values),
		SubmittedAt: submittedAt,
	}
}

func assertTableRow(t *testing.T, rows []TopicTableRow, userUUID uuid.UUID, status string, submitted bool, fileUUID uuid.UUID) {
	t.Helper()
	for _, row := range rows {
		if row.User.UUID != userUUID {
			continue
		}
		if row.Status != status || (row.SubmissionUUID != nil) != submitted {
			t.Fatalf("row = %+v, want status %q submitted %v", row, status, submitted)
		}
		if fileUUID != uuid.Nil && (len(row.Files) != 1 || row.Files[0].UUID != fileUUID) {
			t.Fatalf("row files = %+v, want %s", row.Files, fileUUID)
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(row.Values, &values); err != nil {
			t.Fatalf("decode row values: %v", err)
		}
		if len(values) != 2 {
			t.Fatalf("row values = %s, want every active field", row.Values)
		}
		return
	}
	t.Fatalf("row for user %s not found", userUUID)
}
