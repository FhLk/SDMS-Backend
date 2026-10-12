package admin

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOrganizationServiceWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	// ทุกการเปลี่ยนหัวหน้าและปีอยู่ใน transaction ทดสอบที่ rollback ตอนจบ
	// จึงตรวจ behavior กับ PostgreSQL จริงได้โดยไม่เปลี่ยนข้อมูลของผู้พัฒนา.
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order").Limit(2).Find(&divisions).Error; err != nil {
		t.Fatalf("find divisions: %v", err)
	}
	if len(divisions) != 2 {
		t.Fatalf("divisions = %d, want at least 2", len(divisions))
	}

	suffix := uuid.NewString()[:8]
	inactiveAt := time.Now()
	users := []model.User{
		{Username: "be07." + suffix + ".one", PasswordHash: "integration-hash", FullName: "ครู หนึ่ง", Position: "ครู"},
		{Username: "be07." + suffix + ".two", PasswordHash: "integration-hash", FullName: "ครู สอง", Position: "ครู"},
		{Username: "be07." + suffix + ".exec", PasswordHash: "integration-hash", FullName: "ผู้บริหาร", Position: "ผู้อำนวยการ", IsExec: true},
		{Username: "be07." + suffix + ".off", PasswordHash: "integration-hash", FullName: "บัญชีปิด", Position: "ครู", DeactivatedAt: &inactiveAt},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}

	service := NewOrganizationService(tx)
	updated, err := service.SetDivisionHead(context.Background(), divisions[0].Code, users[0].UUID)
	if err != nil {
		t.Fatalf("SetDivisionHead(first) error = %v", err)
	}
	if updated.HeadUser == nil || updated.HeadUser.UUID != users[0].UUID {
		t.Fatalf("head = %+v, want first teacher", updated.HeadUser)
	}

	// ตั้งคนใหม่ในฝ่ายเดิมต้องแทนที่คนเดิม ไม่เหลือหัวหน้าสองคน.
	updated, err = service.SetDivisionHead(context.Background(), divisions[0].Code, users[1].UUID)
	if err != nil {
		t.Fatalf("SetDivisionHead(replace) error = %v", err)
	}
	if updated.HeadUser == nil || updated.HeadUser.UUID != users[1].UUID {
		t.Fatalf("replacement head = %+v, want second teacher", updated.HeadUser)
	}
	assertDivisionHeadCount(t, tx, users[0].UUID, 0)
	assertDivisionHeadCount(t, tx, users[1].UUID, 1)

	_, err = service.SetDivisionHead(context.Background(), divisions[1].Code, users[1].UUID)
	if !errors.Is(err, ErrUserAlreadyDivisionHead) {
		t.Fatalf("assign same user to another division error = %v, want ErrUserAlreadyDivisionHead", err)
	}
	_, err = service.SetDivisionHead(context.Background(), divisions[1].Code, users[2].UUID)
	if !errors.Is(err, ErrExecutiveCannotHead) {
		t.Fatalf("assign executive error = %v, want ErrExecutiveCannotHead", err)
	}
	_, err = service.SetDivisionHead(context.Background(), divisions[1].Code, users[3].UUID)
	if !errors.Is(err, ErrInactiveUserCannotHead) {
		t.Fatalf("assign inactive user error = %v, want ErrInactiveUserCannotHead", err)
	}

	newYear := availableIntegrationYear(t, tx)
	semesterOne := int16(1)
	createdYear, err := service.CreateAcademicYear(context.Background(), CreateAcademicYearInput{
		Year:            newYear,
		CurrentSemester: &semesterOne,
	})
	if err != nil {
		t.Fatalf("CreateAcademicYear() error = %v", err)
	}
	if createdYear.IsCurrent || createdYear.CurrentSemester != 1 || createdYear.TopicCount != 0 {
		t.Fatalf("created year = %+v, want non-current first semester with no topics", createdYear)
	}
	_, err = service.CreateAcademicYear(context.Background(), CreateAcademicYearInput{Year: newYear})
	if !errors.Is(err, ErrAcademicYearExists) {
		t.Fatalf("duplicate year error = %v, want ErrAcademicYearExists", err)
	}

	topic := model.Topic{
		DivisionUUID:     divisions[0].UUID,
		AcademicYearUUID: createdYear.UUID,
		Semester:         1,
		Title:            "หัวข้อทดสอบ BE-07",
		DueDate:          time.Now().AddDate(0, 0, 7),
		IsOpen:           true,
		CreatedByUUID:    users[0].UUID,
	}
	if err := tx.Create(&topic).Error; err != nil {
		t.Fatalf("create topic: %v", err)
	}

	semesterTwo := int16(2)
	currentYear, err := service.SetCurrentAcademicYear(context.Background(), SetCurrentAcademicYearInput{
		Year:            newYear,
		CurrentSemester: &semesterTwo,
	})
	if err != nil {
		t.Fatalf("SetCurrentAcademicYear() error = %v", err)
	}
	if !currentYear.IsCurrent || currentYear.CurrentSemester != 2 || currentYear.TopicCount != 1 {
		t.Fatalf("current year = %+v, want second semester with one topic", currentYear)
	}
	assertCurrentAcademicYearCount(t, tx, 1)

	years, err := service.ListAcademicYears(context.Background())
	if err != nil {
		t.Fatalf("ListAcademicYears() error = %v", err)
	}
	found := false
	for _, year := range years {
		if year.Year == newYear {
			found = year.IsCurrent && year.TopicCount == 1
			break
		}
	}
	if !found {
		t.Fatalf("ListAcademicYears() = %+v, want new current year with one topic", years)
	}

	missingYear := availableIntegrationYear(t, tx)
	_, err = service.SetCurrentAcademicYear(context.Background(), SetCurrentAcademicYearInput{Year: missingYear})
	if !errors.Is(err, ErrAcademicYearNotFound) {
		t.Fatalf("set missing current year error = %v, want ErrAcademicYearNotFound", err)
	}
	// การหาไม่พบต้อง rollback โดยปีที่เลือกไว้ก่อนหน้ายังคงเป็นปีปัจจุบัน.
	var stillCurrent model.AcademicYear
	if err := tx.Where("year = ? AND is_current = ?", newYear, true).Take(&stillCurrent).Error; err != nil {
		t.Fatalf("current year changed after not-found request: %v", err)
	}
}

func availableIntegrationYear(t *testing.T, db *gorm.DB) int16 {
	t.Helper()
	for year := int16(3000); year >= 2400; year-- {
		var count int64
		if err := db.Model(&model.AcademicYear{}).Where("year = ?", year).Count(&count).Error; err != nil {
			t.Fatalf("check available academic year: %v", err)
		}
		if count == 0 {
			return year
		}
	}
	t.Fatal("no academic year is available for integration test")
	return 0
}

func assertDivisionHeadCount(t *testing.T, db *gorm.DB, userUUID uuid.UUID, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Division{}).Where("head_user_uuid = ?", userUUID).Count(&count).Error; err != nil {
		t.Fatalf("count division heads: %v", err)
	}
	if count != want {
		t.Fatalf("division head count = %d, want %d", count, want)
	}
}

func assertCurrentAcademicYearCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.AcademicYear{}).Where("is_current = ?", true).Count(&count).Error; err != nil {
		t.Fatalf("count current academic years: %v", err)
	}
	if count != want {
		t.Fatalf("current academic year count = %d, want %d", count, want)
	}
}
