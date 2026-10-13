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
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestListDivisionTopicsWithPostgreSQL(t *testing.T) {
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
	// rollback fixtures ของ BE-10 ทั้งหมดเพื่อไม่แก้ข้อมูลจริงของผู้ใช้.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order, code").Find(&divisions).Error; err != nil {
		t.Fatalf("find divisions: %v", err)
	}
	if len(divisions) < 2 {
		t.Fatalf("divisions = %d, want at least 2", len(divisions))
	}
	var currentYear model.AcademicYear
	if err := tx.Where("is_current = ?", true).First(&currentYear).Error; err != nil {
		t.Fatalf("find current academic year: %v", err)
	}

	suffix := uuid.NewString()[:8]
	inactiveAt := time.Now().UTC()
	users := []model.User{
		{Username: "be10." + suffix + ".creator", PasswordHash: "integration-hash", FullName: "ผู้สร้าง", Position: "ครู"},
		{Username: "be10." + suffix + ".target", PasswordHash: "integration-hash", FullName: "ผู้ใช้ปัจจุบัน", Position: "ครู"},
		{Username: "be10." + suffix + ".exec", PasswordHash: "integration-hash", FullName: "ผู้บริหาร", Position: "ผู้อำนวยการ", IsExec: true},
		{Username: "be10." + suffix + ".inactive", PasswordHash: "integration-hash", FullName: "บัญชีปิด", Position: "ครู", DeactivatedAt: &inactiveAt},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	for _, user := range users {
		if err := tx.Create(&model.UserDivision{UserUUID: user.UUID, DivisionUUID: divisions[0].UUID}).Error; err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}

	service := NewService(tx)
	service.now = func() time.Time {
		return time.Date(2026, time.October, 13, 12, 0, 0, 0, topicBangkokLocation)
	}
	topics := []model.Topic{
		createListIntegrationTopic(t, service, divisions[0].UUID, users[0].UUID, "BE-10 submitted "+suffix, "2026-10-10"),
		createListIntegrationTopic(t, service, divisions[0].UUID, users[0].UUID, "BE-10 late "+suffix, "2026-10-11"),
		createListIntegrationTopic(t, service, divisions[0].UUID, users[0].UUID, "BE-10 overdue "+suffix, "2026-10-12"),
		createListIntegrationTopic(t, service, divisions[0].UUID, users[0].UUID, "BE-10 pending "+suffix, "2026-10-14"),
		createListIntegrationTopic(t, service, divisions[0].UUID, users[0].UUID, "BE-10 closed "+suffix, "2026-10-09"),
	}
	if _, err := service.SetOpen(context.Background(), topics[4].UUID, false); err != nil {
		t.Fatalf("close topic: %v", err)
	}
	submissions := []model.Submission{
		{
			TopicUUID:   topics[0].UUID,
			UserUUID:    users[1].UUID,
			Values:      datatypes.JSON([]byte(`{}`)),
			SubmittedAt: time.Date(2026, time.October, 10, 16, 59, 0, 0, time.UTC),
		},
		{
			TopicUUID:   topics[1].UUID,
			UserUUID:    users[1].UUID,
			Values:      datatypes.JSON([]byte(`{}`)),
			SubmittedAt: time.Date(2026, time.October, 11, 17, 0, 0, 0, time.UTC),
		},
	}
	if err := tx.Create(&submissions).Error; err != nil {
		t.Fatalf("create submissions: %v", err)
	}

	listedDivisions, err := service.ListDivisions(context.Background())
	if err != nil {
		t.Fatalf("ListDivisions() error = %v", err)
	}
	for index := 1; index < len(listedDivisions); index++ {
		if listedDivisions[index-1].SortOrder > listedDivisions[index].SortOrder {
			t.Fatalf("divisions are not sorted: %+v", listedDivisions)
		}
	}

	result, err := service.ListDivisionTopics(context.Background(), divisions[0].Code, nil, users[1].UUID)
	if err != nil {
		t.Fatalf("ListDivisionTopics() error = %v", err)
	}
	if result.Division.UUID != divisions[0].UUID || result.AcademicYear.UUID != currentYear.UUID {
		t.Fatalf("division/year = %+v / %+v", result.Division, result.AcademicYear)
	}
	if len(result.Topics) != len(topics) {
		t.Fatalf("topics = %d, want %d", len(result.Topics), len(topics))
	}
	wantStatuses := []string{StatusSubmitted, StatusLate, StatusOverdue, StatusPending, StatusNotSubmitted}
	for index, item := range result.Topics {
		if item.UUID != topics[index].UUID {
			t.Fatalf("topic %d UUID = %s, want %s", index, item.UUID, topics[index].UUID)
		}
		if item.AssigneeCount != 2 {
			t.Fatalf("topic %d assignee_count = %d, want active non-executive snapshot size 2", index, item.AssigneeCount)
		}
		wantSubmitted := int64(0)
		if index < 2 {
			wantSubmitted = 1
		}
		if item.SubmittedCount != wantSubmitted {
			t.Fatalf("topic %d submitted_count = %d, want %d", index, item.SubmittedCount, wantSubmitted)
		}
		if !item.IsAssigned || item.MyStatus == nil || *item.MyStatus != wantStatuses[index] {
			t.Fatalf("topic %d assignment/status = %v/%v, want true/%s", index, item.IsAssigned, item.MyStatus, wantStatuses[index])
		}
	}
	if result.Topics[4].IsOpen {
		t.Fatal("closed topic was not sorted at the end")
	}

	// ผู้บริหารไม่อยู่ใน snapshot แต่ยังดูรายการได้ และ x/y ต้องไม่เปลี่ยนตามผู้ดู.
	executiveResult, err := service.ListDivisionTopics(context.Background(), divisions[0].Code, &currentYear.Year, users[2].UUID)
	if err != nil {
		t.Fatalf("ListDivisionTopics(executive) error = %v", err)
	}
	for index, item := range executiveResult.Topics {
		if item.IsAssigned || item.MyStatus != nil {
			t.Fatalf("executive topic %d assignment/status = %v/%v, want false/nil", index, item.IsAssigned, item.MyStatus)
		}
		if item.AssigneeCount != result.Topics[index].AssigneeCount || item.SubmittedCount != result.Topics[index].SubmittedCount {
			t.Fatalf("counts changed by viewer: target=%+v executive=%+v", result.Topics[index], item)
		}
	}

	_, err = service.ListDivisionTopics(context.Background(), "missing", nil, users[1].UUID)
	if !errors.Is(err, admin.ErrDivisionNotFound) {
		t.Fatalf("missing division error = %v, want ErrDivisionNotFound", err)
	}
	invalidYear := int16(2300)
	_, err = service.ListDivisionTopics(context.Background(), divisions[0].Code, &invalidYear, users[1].UUID)
	var validationErr *admin.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "year" {
		t.Fatalf("invalid year error = %v, want year ValidationError", err)
	}

	missingYear := findUnusedAcademicYear(t, tx)
	_, err = service.ListDivisionTopics(context.Background(), divisions[0].Code, &missingYear, users[1].UUID)
	if !errors.Is(err, ErrTopicAcademicYearNotFound) {
		t.Fatalf("missing year error = %v, want ErrTopicAcademicYearNotFound", err)
	}
}

func createListIntegrationTopic(t *testing.T, service *Service, divisionUUID, creatorUUID uuid.UUID, title, dueDate string) model.Topic {
	t.Helper()
	created, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisionUUID,
		CreatedByUUID: creatorUUID,
		Title:         title,
		DueDate:       dueDate,
		Fields:        []FieldInput{{Type: model.FieldText, Label: "รายละเอียด"}},
	})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", title, err)
	}
	return created
}

func findUnusedAcademicYear(t *testing.T, db *gorm.DB) int16 {
	t.Helper()
	var years []int16
	if err := db.Model(&model.AcademicYear{}).Pluck("year", &years).Error; err != nil {
		t.Fatalf("list academic years: %v", err)
	}
	existing := make(map[int16]struct{}, len(years))
	for _, year := range years {
		existing[year] = struct{}{}
	}
	for year := int16(2400); year <= 3000; year++ {
		if _, found := existing[year]; !found {
			return year
		}
	}
	t.Fatal("no unused academic year available for test")
	return 0
}
