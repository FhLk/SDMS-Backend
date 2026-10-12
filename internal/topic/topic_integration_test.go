package topic

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTopicServiceWithPostgreSQL(t *testing.T) {
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
	// rollback ผู้ใช้ หัวข้อ fields และ assignees ทั้งหมดเมื่อ test จบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order").Limit(2).Find(&divisions).Error; err != nil {
		t.Fatalf("find divisions: %v", err)
	}
	if len(divisions) != 2 {
		t.Fatalf("divisions = %d, want at least 2", len(divisions))
	}
	var currentYear model.AcademicYear
	if err := tx.Where("is_current = ?", true).Take(&currentYear).Error; err != nil {
		t.Fatalf("find current academic year: %v", err)
	}

	suffix := uuid.NewString()[:8]
	inactiveAt := time.Now()
	users := []model.User{
		{Username: "be08." + suffix + ".admin", PasswordHash: "integration-hash", FullName: "ผู้สร้าง", Position: "แอดมิน", IsAdmin: true},
		{Username: "be08." + suffix + ".teacher", PasswordHash: "integration-hash", FullName: "ครูที่ต้องส่ง", Position: "ครู"},
		{Username: "be08." + suffix + ".exec", PasswordHash: "integration-hash", FullName: "ผู้บริหาร", Position: "ผู้อำนวยการ", IsExec: true},
		{Username: "be08." + suffix + ".inactive", PasswordHash: "integration-hash", FullName: "บัญชีปิด", Position: "ครู", DeactivatedAt: &inactiveAt},
		{Username: "be08." + suffix + ".other", PasswordHash: "integration-hash", FullName: "ครูต่างฝ่าย", Position: "ครู"},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	memberships := []model.UserDivision{
		{UserUUID: users[0].UUID, DivisionUUID: divisions[0].UUID},
		{UserUUID: users[1].UUID, DivisionUUID: divisions[0].UUID},
		{UserUUID: users[2].UUID, DivisionUUID: divisions[0].UUID},
		{UserUUID: users[3].UUID, DivisionUUID: divisions[0].UUID},
		{UserUUID: users[4].UUID, DivisionUUID: divisions[1].UUID},
	}
	if err := tx.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}

	service := NewService(tx)
	title := "BE-08 " + suffix
	created, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         " " + title + " ",
		DueDate:       "2026-10-31",
		IsSecret:      true,
		Fields: []FieldInput{
			{Position: 6, Type: model.FieldFile, Label: "หลักฐาน", Required: true},
			{Position: 1, Type: model.FieldText, Label: "ชื่อกิจกรรม", Required: true},
			{Position: 5, Type: model.FieldChoice, Label: "ระดับชั้น", Required: true, Options: []string{"ม.1", "ม.2"}},
			{Position: 3, Type: model.FieldNum, Label: "จำนวน", Required: false},
			{Position: 2, Type: model.FieldPara, Label: "รายละเอียด", Required: false},
			{Position: 4, Type: model.FieldDate, Label: "วันที่", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.UUID == uuid.Nil || created.Title != title || !created.IsOpen || !created.IsSecret {
		t.Fatalf("created topic = %+v", created)
	}
	if created.AcademicYearUUID != currentYear.UUID || created.Semester != currentYear.CurrentSemester {
		t.Fatalf("period = year %s semester %d, want %s semester %d", created.AcademicYearUUID, created.Semester, currentYear.UUID, currentYear.CurrentSemester)
	}
	if len(created.Fields) != 6 {
		t.Fatalf("fields = %d, want 6", len(created.Fields))
	}
	for index, field := range created.Fields {
		if field.Position != int16(index+1) {
			t.Fatalf("field %d position = %d, want %d", index, field.Position, index+1)
		}
	}
	if len(created.Assignees) != 2 {
		t.Fatalf("assignees = %d, want active non-executive members only", len(created.Assignees))
	}

	assertTopicRelatedCount(t, tx, &model.TopicField{}, "topic_uuid = ?", 6, created.UUID)
	assertTopicRelatedCount(t, tx, &model.TopicField{}, "topic_uuid = ? AND required = ?", 2, created.UUID, false)
	assertTopicRelatedCount(t, tx, &model.TopicAssignee{}, "topic_uuid = ?", 2, created.UUID)
	var assigneeUUIDs []uuid.UUID
	if err := tx.Model(&model.TopicAssignee{}).
		Where("topic_uuid = ?", created.UUID).
		Order("user_uuid").
		Pluck("user_uuid", &assigneeUUIDs).Error; err != nil {
		t.Fatalf("list assignee UUIDs: %v", err)
	}
	wantAssignees := map[uuid.UUID]bool{users[0].UUID: false, users[1].UUID: false}
	for _, userUUID := range assigneeUUIDs {
		if _, expected := wantAssignees[userUUID]; expected {
			wantAssignees[userUUID] = true
		}
	}
	for userUUID, found := range wantAssignees {
		if !found {
			t.Errorf("active division member %s is missing from assignees: %v", userUUID, assigneeUUIDs)
		}
	}

	// validation ต้องเกิดก่อนเริ่มเขียน จึงไม่มีหัวข้อค้างเมื่อ choice มีตัวเลือกไม่ครบ.
	invalidTitle := title + " invalid"
	_, err = service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         invalidTitle,
		DueDate:       "2026-10-31",
		Fields: []FieldInput{{
			Type: model.FieldChoice, Label: "ตัวเลือกไม่ครบ", Options: []string{"ข้อเดียว"},
		}},
	})
	var validationErr *admin.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "fields[0].options" {
		t.Fatalf("invalid choice error = %v, want fields[0].options ValidationError", err)
	}
	assertTopicRelatedCount(t, tx, &model.Topic{}, "title = ?", 0, invalidTitle)

	// หากไม่มีปีปัจจุบัน transaction ต้องหยุดก่อนสร้าง topic.
	if err := tx.Model(&model.AcademicYear{}).
		Where("is_current = ?", true).
		Update("is_current", false).Error; err != nil {
		t.Fatalf("clear current year for test: %v", err)
	}
	noYearTitle := title + " no year"
	_, err = service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         noYearTitle,
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Type: model.FieldText, Label: "ข้อความ"}},
	})
	if !errors.Is(err, ErrCurrentAcademicYearNotSet) {
		t.Fatalf("Create(no current year) error = %v, want ErrCurrentAcademicYearNotSet", err)
	}
	assertTopicRelatedCount(t, tx, &model.Topic{}, "title = ?", 0, noYearTitle)
}

func assertTopicRelatedCount(t *testing.T, db *gorm.DB, value any, query string, want int64, arguments ...any) {
	t.Helper()
	var count int64
	if err := db.Model(value).Where(query, arguments...).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != want {
		t.Fatalf("row count = %d, want %d", count, want)
	}
}
