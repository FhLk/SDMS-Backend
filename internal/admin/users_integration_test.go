package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUserServiceWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	// ครอบ test ด้วย transaction แล้ว rollback เพื่อไม่ให้ข้อมูลทดสอบค้างในฐานจริง.
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order").Limit(2).Find(&divisions).Error; err != nil {
		t.Fatalf("find seeded divisions: %v", err)
	}
	if len(divisions) != 2 {
		t.Fatalf("seeded divisions = %d, want at least 2", len(divisions))
	}

	service := NewUserService(tx)
	username := "be05." + uuid.NewString()[:8]
	created, err := service.CreateUser(context.Background(), CreateUserInput{
		Username:      username,
		Password:      "old-password",
		FullName:      "ผู้ใช้ทดสอบ BE-05",
		Position:      "ครู",
		DivisionUUIDs: []uuid.UUID{divisions[0].UUID, divisions[1].UUID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.UUID == uuid.Nil || len(created.Divisions) != 2 {
		t.Fatalf("created user = %+v, want UUID and two divisions", created)
	}
	if bcrypt.CompareHashAndPassword([]byte(created.PasswordHash), []byte("old-password")) != nil {
		t.Fatal("created password is not a bcrypt hash of the supplied password")
	}

	_, err = service.CreateUser(context.Background(), CreateUserInput{
		Username: username,
		Password: "other-password",
		FullName: "ชื่อซ้ำ",
		Position: "ครู",
	})
	if !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("duplicate CreateUser() error = %v, want ErrUsernameExists", err)
	}

	// สร้างคำตอบเดิมก่อนปิดบัญชี เพื่อยืนยันว่า service ลบเฉพาะ session
	// และไม่ทำลาย submission หรือ assignment ที่ใช้อ้างอิงประวัติย้อนหลัง.
	var academicYear model.AcademicYear
	if err := tx.Where("is_current = ?", true).First(&academicYear).Error; err != nil {
		t.Fatalf("find current academic year: %v", err)
	}
	topic := model.Topic{
		DivisionUUID:     divisions[0].UUID,
		AcademicYearUUID: academicYear.UUID,
		Semester:         academicYear.CurrentSemester,
		Title:            "หัวข้อทดสอบ BE-05",
		DueDate:          time.Now().AddDate(0, 0, 1),
		IsOpen:           true,
		CreatedByUUID:    created.UUID,
	}
	if err := tx.Create(&topic).Error; err != nil {
		t.Fatalf("create topic: %v", err)
	}
	assignment := model.TopicAssignee{TopicUUID: topic.UUID, UserUUID: created.UUID}
	if err := tx.Create(&assignment).Error; err != nil {
		t.Fatalf("create topic assignment: %v", err)
	}
	submission := model.Submission{
		TopicUUID:   topic.UUID,
		UserUUID:    created.UUID,
		Values:      datatypes.JSON([]byte(`{}`)),
		SubmittedAt: time.Now(),
	}
	if err := tx.Create(&submission).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}

	createIntegrationSession(t, tx, created.UUID, "a")
	inactive := false
	updated, err := service.UpdateUser(context.Background(), created.UUID, UpdateUserInput{IsActive: &inactive})
	if err != nil {
		t.Fatalf("UpdateUser(deactivate) error = %v", err)
	}
	if updated.DeactivatedAt == nil {
		t.Fatal("deactivated_at = nil, want timestamp")
	}
	assertIntegrationSessionCount(t, tx, created.UUID, 0)
	assertIntegrationRowCount(t, tx, &model.Submission{}, "user_uuid = ?", created.UUID, 1)
	assertIntegrationRowCount(t, tx, &model.TopicAssignee{}, "user_uuid = ?", created.UUID, 1)

	createIntegrationSession(t, tx, created.UUID, "b")
	if err := service.ResetPassword(context.Background(), created.UUID, "new-password"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	assertIntegrationSessionCount(t, tx, created.UUID, 0)

	var passwordHash string
	if err := tx.Model(&model.User{}).Select("password_hash").Where("uuid = ?", created.UUID).Scan(&passwordHash).Error; err != nil {
		t.Fatalf("read password hash: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("new-password")) != nil {
		t.Fatal("password was not replaced with the new bcrypt hash")
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("old-password")) == nil {
		t.Fatal("old password still matches after reset")
	}

	users, err := service.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	found := false
	for _, user := range users {
		if user.UUID == created.UUID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ListUsers() did not return the created user")
	}
}

func TestUserImportServiceWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	// ใช้ outer transaction แล้ว rollback หลัง test เพื่อพิสูจน์การบันทึกจริง
	// โดยไม่ทิ้งบัญชีหรือความสัมพันธ์ฝ่ายไว้ในฐานข้อมูลของผู้พัฒนา.
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order").Limit(2).Find(&divisions).Error; err != nil {
		t.Fatalf("find seeded divisions: %v", err)
	}
	if len(divisions) != 2 {
		t.Fatalf("seeded divisions = %d, want at least 2", len(divisions))
	}

	service := NewUserService(tx)
	suffix := uuid.NewString()[:8]
	firstUsername := "be06." + suffix + ".one"
	secondUsername := "be06." + suffix + ".two"
	validCSV := fmt.Sprintf(
		"ชื่อ-สกุล,username,รหัสผ่านเริ่มต้น,ตำแหน่ง,ฝ่าย\nครู หนึ่ง,%s,safe-password,ครู,%s|%s\nครู สอง,%s,other-password,ครู,%s\n",
		firstUsername,
		divisions[0].Code,
		divisions[1].ShortName,
		secondUsername,
		divisions[1].Name,
	)
	result, err := service.ImportUsers(context.Background(), "users.csv", []byte(validCSV))
	if err != nil {
		t.Fatalf("ImportUsers(valid) error = %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("imported = %d, want 2", result.Imported)
	}

	assertIntegrationRowCount(t, tx, &model.User{}, "username IN ?", []string{firstUsername, secondUsername}, 2)
	var firstUser model.User
	if err := tx.Where("username = ?", firstUsername).Take(&firstUser).Error; err != nil {
		t.Fatalf("find imported user: %v", err)
	}
	if firstUser.IsAdmin || firstUser.IsExec {
		t.Fatal("imported user unexpectedly received admin or executive privileges")
	}
	if bcrypt.CompareHashAndPassword([]byte(firstUser.PasswordHash), []byte("safe-password")) != nil {
		t.Fatal("imported password is not stored as the expected bcrypt hash")
	}
	assertIntegrationRowCount(t, tx, &model.UserDivision{}, "user_uuid = ?", firstUser.UUID, 2)

	// เมื่อไฟล์มีทั้งแถวที่ถูกและผิด ต้องรายงาน error แล้วไม่เขียนแม้แต่แถวที่ถูก.
	atomicUsername := "be06." + suffix + ".atomic"
	invalidUsername := "be06." + suffix + ".invalid"
	invalidCSV := fmt.Sprintf(
		"ชื่อ-สกุล,username,รหัสผ่านเริ่มต้น,ตำแหน่ง,ฝ่าย\nครู ผ่าน,%s,safe-password,ครู,%s\nครู ไม่ผ่าน,%s,safe-password,ครู,ฝ่ายที่ไม่มี\n",
		atomicUsername,
		divisions[0].Code,
		invalidUsername,
	)
	_, err = service.ImportUsers(context.Background(), "users.csv", []byte(invalidCSV))
	var importErr *ImportValidationError
	if !errors.As(err, &importErr) || len(importErr.Rows) == 0 {
		t.Fatalf("ImportUsers(invalid) error = %v, want ImportValidationError", err)
	}
	assertIntegrationRowCount(t, tx, &model.User{}, "username IN ?", []string{atomicUsername, invalidUsername}, 0)
}

func createIntegrationSession(t *testing.T, db *gorm.DB, userUUID uuid.UUID, suffix string) {
	t.Helper()
	now := time.Now()
	session := model.Session{
		TokenHash:  "integration-session-" + suffix + "------------------------------------------",
		UserUUID:   userUUID,
		ExpiresAt:  now.Add(time.Hour),
		LastSeenAt: now,
		CreatedAt:  now,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
}

func assertIntegrationSessionCount(t *testing.T, db *gorm.DB, userUUID uuid.UUID, want int64) {
	t.Helper()
	assertIntegrationRowCount(t, db, &model.Session{}, "user_uuid = ?", userUUID, want)
}

func assertIntegrationRowCount(t *testing.T, db *gorm.DB, value any, query string, argument any, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(value).Where(query, argument).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != want {
		t.Fatalf("row count = %d, want %d", count, want)
	}
}
