package home

import (
	"context"
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

func TestHomeServiceWithPostgreSQL(t *testing.T) {
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
	// fixture ของ BE-15 ทั้งหมดอยู่ใน outer transaction และถูก rollback หลังทดสอบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order, code").Find(&divisions).Error; err != nil || len(divisions) < 3 {
		t.Fatalf("load divisions = %d, error = %v", len(divisions), err)
	}
	var academicYear model.AcademicYear
	if err := tx.Order("year DESC").First(&academicYear).Error; err != nil {
		t.Fatalf("load academic year: %v", err)
	}

	suffix := uuid.NewString()[:8]
	creator := createHomeUser(t, tx, "be15.creator."+suffix, "ผู้สร้าง BE-15")
	teacher := createHomeUser(t, tx, "be15.teacher."+suffix, "ครู BE-15")
	other := createHomeUser(t, tx, "be15.other."+suffix, "ครูอีกคน BE-15")

	service := NewService(tx)
	service.now = func() time.Time {
		return time.Date(2026, time.October, 13, 12, 0, 0, 0, bangkokLocation)
	}
	executiveProfile := auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: creator.UUID}, IsExec: true}}
	headProfile := auth.Profile{User: creator, HeadDivisions: []model.Division{divisions[0]}}
	baseline, err := service.Get(context.Background(), executiveProfile)
	if err != nil {
		t.Fatalf("Get(baseline) error = %v", err)
	}
	baselineFirstDivision, err := service.DivisionStats(context.Background(), divisions[0].Code, headProfile)
	if err != nil {
		t.Fatalf("DivisionStats(baseline) error = %v", err)
	}

	pending := createHomeTopic(t, tx, divisions[0], academicYear, creator, true, "2026-10-20", "BE-15 pending "+suffix)
	overdue := createHomeTopic(t, tx, divisions[0], academicYear, creator, true, "2026-10-12", "BE-15 overdue "+suffix)
	onTime := createHomeTopic(t, tx, divisions[1], academicYear, creator, true, "2026-10-13", "BE-15 submitted "+suffix)
	late := createHomeTopic(t, tx, divisions[1], academicYear, creator, true, "2026-10-12", "BE-15 late "+suffix)
	closed := createHomeTopic(t, tx, divisions[0], academicYear, creator, false, "2026-10-12", "BE-15 closed "+suffix)
	otherPending := createHomeTopic(t, tx, divisions[0], academicYear, creator, true, "2026-10-20", "BE-15 other "+suffix)
	_ = createHomeTopic(t, tx, divisions[2], academicYear, creator, true, "2026-10-20", "BE-15 no assignee "+suffix)

	for _, topic := range []model.Topic{pending, overdue, onTime, late, closed} {
		createHomeAssignee(t, tx, topic.UUID, teacher.UUID)
	}
	createHomeAssignee(t, tx, otherPending.UUID, other.UUID)
	createHomeSubmission(t, tx, onTime.UUID, teacher.UUID, time.Date(2026, time.October, 13, 10, 0, 0, 0, bangkokLocation))
	createHomeSubmission(t, tx, late.UUID, teacher.UUID, time.Date(2026, time.October, 13, 11, 0, 0, 0, bangkokLocation))

	teacherDashboard, err := service.Get(context.Background(), auth.Profile{User: teacher})
	if err != nil {
		t.Fatalf("Get(teacher) error = %v", err)
	}
	if teacherDashboard.Overview != nil {
		t.Fatal("teacher overview is not nil")
	}
	wantMine := Counts{TopicCount: 4, RequiredCount: 4, SubmittedCount: 2, PendingCount: 1, OverdueCount: 1}
	if teacherDashboard.Personal.Counts != wantMine {
		t.Fatalf("teacher counts = %+v, want %+v", teacherDashboard.Personal.Counts, wantMine)
	}
	if len(teacherDashboard.Personal.PendingTopics) != 2 || teacherDashboard.Personal.PendingTopics[0].UUID != overdue.UUID || teacherDashboard.Personal.PendingTopics[1].UUID != pending.UUID {
		t.Fatalf("teacher pending topics = %+v", teacherDashboard.Personal.PendingTopics)
	}
	if len(teacherDashboard.Personal.RecentSubmissions) != 2 || teacherDashboard.Personal.RecentSubmissions[0].TopicUUID != late.UUID {
		t.Fatalf("teacher recent submissions = %+v", teacherDashboard.Personal.RecentSubmissions)
	}

	headDashboard, err := service.Get(context.Background(), headProfile)
	if err != nil || headDashboard.Overview == nil {
		t.Fatalf("Get(head) overview = %v, error = %v", headDashboard.Overview, err)
	}
	adminDashboard, err := service.Get(context.Background(), auth.Profile{User: model.User{
		UUIDModel: model.UUIDModel{UUID: creator.UUID},
		IsAdmin:   true,
	}})
	if err != nil {
		t.Fatalf("Get(admin) error = %v", err)
	}
	if adminDashboard.Overview != nil {
		t.Fatal("admin-only overview is not nil")
	}

	executiveDashboard, err := service.Get(context.Background(), executiveProfile)
	if err != nil {
		t.Fatalf("Get(executive) error = %v", err)
	}
	if executiveDashboard.Overview == nil || baseline.Overview == nil {
		t.Fatal("executive overview is nil")
	}
	assertCountDelta(t, executiveDashboard.Overview.School, baseline.Overview.School, Counts{
		TopicCount: 6, RequiredCount: 5, SubmittedCount: 2, PendingCount: 2, OverdueCount: 1,
	})

	firstDivision, err := service.DivisionStats(context.Background(), divisions[0].Code, headProfile)
	if err != nil {
		t.Fatalf("DivisionStats(head own division) error = %v", err)
	}
	assertCountDelta(t, firstDivision.Counts, baselineFirstDivision.Counts, Counts{
		TopicCount: 3, RequiredCount: 3, PendingCount: 2, OverdueCount: 1,
	})
	if _, err := service.DivisionStats(context.Background(), divisions[1].Code, headProfile); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DivisionStats(head other division) error = %v, want ErrForbidden", err)
	}
	if _, err := service.DivisionStats(context.Background(), divisions[0].Code, auth.Profile{User: teacher}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DivisionStats(teacher) error = %v, want ErrForbidden", err)
	}
	if _, err := service.DivisionStats(context.Background(), divisions[1].Code, executiveProfile); err != nil {
		t.Fatalf("DivisionStats(executive) error = %v", err)
	}
	if _, err := service.DivisionStats(context.Background(), "missing", executiveProfile); !errors.Is(err, ErrDivisionNotFound) {
		t.Fatalf("DivisionStats(missing) error = %v, want ErrDivisionNotFound", err)
	}
}

func createHomeUser(t *testing.T, db *gorm.DB, username, fullName string) model.User {
	t.Helper()
	user := model.User{Username: username, PasswordHash: "hash", FullName: fullName, Position: "ครู"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func createHomeTopic(t *testing.T, db *gorm.DB, division model.Division, year model.AcademicYear, creator model.User, open bool, dueDate, title string) model.Topic {
	t.Helper()
	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		t.Fatalf("parse due date: %v", err)
	}
	topic := model.Topic{
		DivisionUUID:     division.UUID,
		AcademicYearUUID: year.UUID,
		Semester:         1,
		Title:            title,
		DueDate:          due,
		IsOpen:           true,
		CreatedByUUID:    creator.UUID,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("create topic %s: %v", title, err)
	}
	if !open {
		if err := db.Model(&model.Topic{}).Where("uuid = ?", topic.UUID).Update("is_open", false).Error; err != nil {
			t.Fatalf("close topic %s: %v", title, err)
		}
		topic.IsOpen = false
	}
	return topic
}

func createHomeAssignee(t *testing.T, db *gorm.DB, topicUUID, userUUID uuid.UUID) {
	t.Helper()
	if err := db.Create(&model.TopicAssignee{TopicUUID: topicUUID, UserUUID: userUUID}).Error; err != nil {
		t.Fatalf("create assignee: %v", err)
	}
}

func createHomeSubmission(t *testing.T, db *gorm.DB, topicUUID, userUUID uuid.UUID, submittedAt time.Time) {
	t.Helper()
	submission := model.Submission{
		TopicUUID:   topicUUID,
		UserUUID:    userUUID,
		Values:      datatypes.JSON(`{}`),
		SubmittedAt: submittedAt.UTC(),
		UpdatedAt:   submittedAt.UTC(),
	}
	if err := db.Create(&submission).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}
}

func assertCountDelta(t *testing.T, got, baseline, wantDelta Counts) {
	t.Helper()
	delta := Counts{
		TopicCount:     got.TopicCount - baseline.TopicCount,
		RequiredCount:  got.RequiredCount - baseline.RequiredCount,
		SubmittedCount: got.SubmittedCount - baseline.SubmittedCount,
		PendingCount:   got.PendingCount - baseline.PendingCount,
		OverdueCount:   got.OverdueCount - baseline.OverdueCount,
	}
	if delta != wantDelta {
		t.Fatalf("count delta = %+v, want %+v (got=%+v baseline=%+v)", delta, wantDelta, got, baseline)
	}
}
