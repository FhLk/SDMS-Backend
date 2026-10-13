package submission

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSubmissionServiceWithPostgreSQL(t *testing.T) {
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
	// fixture ของ BE-12 อยู่ใน outer transaction และถูก rollback ทั้งหมดเมื่อ test จบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var division model.Division
	if err := tx.Order("sort_order, code").First(&division).Error; err != nil {
		t.Fatalf("find division: %v", err)
	}
	suffix := uuid.NewString()[:8]
	users := []model.User{
		{Username: "be12.creator." + suffix, PasswordHash: "hash", FullName: "ผู้สร้าง BE-12", Position: "ครู"},
		{Username: "be12.assignee." + suffix, PasswordHash: "hash", FullName: "ผู้ส่ง BE-12", Position: "ครู"},
		{Username: "be12.outsider." + suffix, PasswordHash: "hash", FullName: "คนนอก BE-12", Position: "ครู"},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	for _, user := range users[:2] {
		if err := tx.Create(&model.UserDivision{UserUUID: user.UUID, DivisionUUID: division.UUID}).Error; err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}

	topicService := topic.NewService(tx)
	createdTopic, err := topicService.Create(context.Background(), topic.CreateInput{
		DivisionUUID:  division.UUID,
		CreatedByUUID: users[0].UUID,
		Title:         "แบบส่ง BE-12 " + suffix,
		DueDate:       "2026-10-12",
		Fields: []topic.FieldInput{
			{Position: 1, Type: model.FieldText, Label: "ชื่อกิจกรรม", Required: true},
			{Position: 2, Type: model.FieldNum, Label: "จำนวน", Required: true},
			{Position: 3, Type: model.FieldChoice, Label: "ระดับ", Required: true, Options: []string{"ม.1", "ม.2"}},
			{Position: 4, Type: model.FieldFile, Label: "หลักฐาน", Required: true},
			{Position: 5, Type: model.FieldPara, Label: "หมายเหตุ", Required: false},
		},
	})
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	fileField := createdTopic.Fields[3]
	files := []model.File{
		newStagedFile(createdTopic.UUID, fileField.UUID, users[1].UUID, "be12/file-1.pdf", "one.pdf"),
		newStagedFile(createdTopic.UUID, fileField.UUID, users[1].UUID, "be12/file-2.pdf", "two.pdf"),
	}
	if err := tx.Create(&files).Error; err != nil {
		t.Fatalf("create staged files: %v", err)
	}

	store := &recordingRemover{removed: make(map[string]bool)}
	service := NewService(tx, store)
	firstNow := time.Date(2026, time.October, 13, 3, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return firstNow }

	pending, err := service.ListPending(context.Background(), users[1].UUID)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 1 || pending[0].Topic.UUID != createdTopic.UUID || pending[0].Status != StatusOverdue {
		t.Fatalf("pending = %+v", pending)
	}
	before, err := service.GetMine(context.Background(), createdTopic.UUID, users[1].UUID)
	if err != nil || before.Submission != nil {
		t.Fatalf("GetMine(before) = %+v, %v", before, err)
	}

	_, err = service.Upsert(context.Background(), UpsertInput{
		TopicUUID: createdTopic.UUID,
		UserUUID:  users[1].UUID,
		Values:    map[string]json.RawMessage{},
	})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Fields) != 4 {
		t.Fatalf("Upsert(invalid) error = %#v, want four required-field violations", err)
	}

	firstValues := validIntegrationValues(createdTopic.Fields, files[0].UUID, "กิจกรรมแรก", "หมายเหตุเดิม")
	created, err := service.Upsert(context.Background(), UpsertInput{
		TopicUUID: createdTopic.UUID,
		UserUUID:  users[1].UUID,
		Values:    firstValues,
	})
	if err != nil {
		t.Fatalf("Upsert(create) error = %v", err)
	}
	if created.Submission == nil || created.Status == nil || *created.Status != StatusLate {
		t.Fatalf("created detail = %+v", created)
	}
	firstSubmissionUUID := created.Submission.UUID
	firstSubmittedAt := created.Submission.SubmittedAt
	assertFileState(t, tx, files[0].UUID, model.FileAttached, &firstSubmissionUUID)

	pending, err = service.ListPending(context.Background(), users[1].UUID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("ListPending(after submit) = %+v, %v", pending, err)
	}

	// จำลองหัวหน้าลบช่อง optional หลังมีคำตอบ ช่องนี้ต้องถูกเก็บเป็น hidden value
	// เมื่อผู้ใช้แก้คำตอบโดยส่งเฉพาะช่องที่ยังแสดงอยู่.
	hiddenField := createdTopic.Fields[4]
	hiddenAt := firstNow.Add(time.Hour)
	if err := tx.Model(&model.TopicField{}).Where("uuid = ?", hiddenField.UUID).Update("hidden_at", hiddenAt).Error; err != nil {
		t.Fatalf("hide answered field: %v", err)
	}
	secondNow := firstNow.Add(2 * time.Hour)
	service.now = func() time.Time { return secondNow }
	secondValues := validIntegrationValues(createdTopic.Fields[:4], files[1].UUID, "กิจกรรมแก้ไข", "")
	updated, err := service.Upsert(context.Background(), UpsertInput{
		TopicUUID: createdTopic.UUID,
		UserUUID:  users[1].UUID,
		Values:    secondValues,
	})
	if err != nil {
		t.Fatalf("Upsert(update) error = %v", err)
	}
	if updated.Submission == nil || updated.Submission.UUID != firstSubmissionUUID || !updated.Submission.SubmittedAt.Equal(firstSubmittedAt) {
		t.Fatalf("updated submission identity/time = %+v", updated.Submission)
	}
	var updatedValues map[string]json.RawMessage
	if err := json.Unmarshal(updated.Submission.Values, &updatedValues); err != nil {
		t.Fatalf("decode updated values: %v", err)
	}
	if got := string(updatedValues[hiddenField.UUID.String()]); got != `"หมายเหตุเดิม"` {
		t.Fatalf("hidden value = %s, want preserved old value", got)
	}
	assertFileState(t, tx, files[0].UUID, model.FileTrashed, &firstSubmissionUUID)
	assertFileState(t, tx, files[1].UUID, model.FileAttached, &firstSubmissionUUID)
	if !store.wasRemoved(files[0].ObjectKey) {
		t.Fatalf("removed object %q was not deleted", files[0].ObjectKey)
	}

	if _, err := service.GetMine(context.Background(), createdTopic.UUID, users[2].UUID); !errors.Is(err, ErrNotAssignee) {
		t.Fatalf("GetMine(outsider) error = %v, want ErrNotAssignee", err)
	}
	if _, err := topicService.SetOpen(context.Background(), createdTopic.UUID, false); err != nil {
		t.Fatalf("close topic: %v", err)
	}
	if _, err := service.Upsert(context.Background(), UpsertInput{TopicUUID: createdTopic.UUID, UserUUID: users[1].UUID, Values: secondValues}); !errors.Is(err, ErrTopicClosed) {
		t.Fatalf("Upsert(closed) error = %v, want ErrTopicClosed", err)
	}
}

func newStagedFile(topicUUID, fieldUUID, userUUID uuid.UUID, objectKey, name string) model.File {
	return model.File{
		TopicUUID:      topicUUID,
		FieldUUID:      fieldUUID,
		UploadedByUUID: userUUID,
		ObjectKey:      objectKey,
		OriginalName:   name,
		MimeType:       "application/pdf",
		SizeBytes:      10,
		Status:         model.FileStaged,
	}
}

func validIntegrationValues(fields []model.TopicField, fileUUID uuid.UUID, text, note string) map[string]json.RawMessage {
	values := map[string]json.RawMessage{
		fields[0].UUID.String(): json.RawMessage(`"` + text + `"`),
		fields[1].UUID.String(): json.RawMessage(`12.5`),
		fields[2].UUID.String(): json.RawMessage(`"ม.2"`),
		fields[3].UUID.String(): json.RawMessage(`["` + fileUUID.String() + `"]`),
	}
	if len(fields) > 4 && note != "" {
		values[fields[4].UUID.String()] = json.RawMessage(`"` + note + `"`)
	}
	return values
}

func assertFileState(t *testing.T, db *gorm.DB, fileUUID uuid.UUID, status string, submissionUUID *uuid.UUID) {
	t.Helper()
	var file model.File
	if err := db.Where("uuid = ?", fileUUID).Take(&file).Error; err != nil {
		t.Fatalf("find file %s: %v", fileUUID, err)
	}
	if file.Status != status || file.SubmissionUUID == nil || submissionUUID == nil || *file.SubmissionUUID != *submissionUUID {
		t.Fatalf("file state = status %q submission %v, want %q/%v", file.Status, file.SubmissionUUID, status, submissionUUID)
	}
}

type recordingRemover struct {
	mu      sync.Mutex
	removed map[string]bool
}

func (r *recordingRemover) Trash(_ context.Context, objectKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed[objectKey] = true
	return nil
}

func (r *recordingRemover) wasRemoved(objectKey string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.removed[objectKey]
}
