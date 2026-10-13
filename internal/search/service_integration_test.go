package search

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSearchServiceWithPostgreSQL(t *testing.T) {
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
	// fixture ทั้งหมดของ BE-17 อยู่ใน transaction และถูก rollback หลังทดสอบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var divisions []model.Division
	if err := tx.Order("sort_order, code").Find(&divisions).Error; err != nil || len(divisions) < 2 {
		t.Fatalf("load divisions = %d, error = %v", len(divisions), err)
	}
	var firstYear model.AcademicYear
	if err := tx.Order("year DESC").First(&firstYear).Error; err != nil {
		t.Fatalf("load academic year: %v", err)
	}
	secondYear := createSearchAcademicYear(t, tx, firstYear.Year)

	suffix := uuid.NewString()[:8]
	marker := "BE17-" + suffix
	viewer := createSearchUser(t, tx, "be17.viewer."+suffix, "ครูผู้ค้น "+marker)
	submitter := createSearchUser(t, tx, "be17.submit."+suffix, "ครูผู้ส่ง "+marker)
	creator := createSearchUser(t, tx, "be17.creator."+suffix, "ผู้สร้าง "+marker)
	head := createSearchUser(t, tx, "be17.head."+suffix, "หัวหน้าฝ่าย "+marker)
	executive := createSearchUser(t, tx, "be17.exec."+suffix, "ผู้บริหาร "+marker)
	unrelated := createSearchUser(t, tx, "be17.other."+suffix, "ครูไม่เกี่ยวข้อง "+marker)

	publicOne, publicOneField := createSearchTopic(
		t, tx, divisions[0], firstYear, creator, 1,
		"ค้นหา 100%_ภาษาไทย "+marker, "2026-10-12", false,
	)
	if err := tx.Model(&model.Topic{}).Where("uuid = ?", publicOne.UUID).Update("is_open", false).Error; err != nil {
		t.Fatalf("close first public topic: %v", err)
	}
	publicTwo, publicTwoField := createSearchTopic(
		t, tx, divisions[1], secondYear, creator, 2,
		"หัวข้อทั่วไปสอง "+marker, "2026-10-20", false,
	)
	secret, secretField := createSearchTopic(
		t, tx, divisions[0], firstYear, creator, 1,
		"หัวข้อลับ "+marker, "2026-10-20", true,
	)

	publicOneSubmission := createSearchSubmission(
		t, tx, publicOne.UUID, submitter.UUID,
		`{"note":"ค่าที่กรอกเฉพาะหนึ่ง"}`,
		time.Date(2026, time.October, 11, 3, 0, 0, 0, time.UTC),
	)
	secretOwnSubmission := createSearchSubmission(
		t, tx, secret.UUID, viewer.UUID,
		`{"note":"คำตอบลับของฉัน"}`,
		time.Date(2026, time.October, 12, 3, 0, 0, 0, time.UTC),
	)
	secretOtherSubmission := createSearchSubmission(
		t, tx, secret.UUID, submitter.UUID,
		`{"note":"คำตอบลับห้ามรั่ว"}`,
		time.Date(2026, time.October, 13, 3, 0, 0, 0, time.UTC),
	)
	publicTwoSubmission := createSearchSubmission(
		t, tx, publicTwo.UUID, submitter.UUID,
		`{"note":"ข้อมูลทั่วไปสอง"}`,
		time.Date(2026, time.October, 14, 3, 0, 0, 0, time.UTC),
	)

	publicFile := createSearchFile(t, tx, publicOne.UUID, publicOneField.UUID, submitter.UUID, &publicOneSubmission.UUID,
		"หลักฐานภาษาไทย "+marker+".pdf", model.FileAttached)
	_ = createSearchFile(t, tx, publicOne.UUID, publicOneField.UUID, submitter.UUID, nil,
		"ไฟล์ staged ห้ามค้น "+marker+".txt", model.FileStaged)
	_ = createSearchFile(t, tx, publicTwo.UUID, publicTwoField.UUID, submitter.UUID, &publicTwoSubmission.UUID,
		"เอกสารทั่วไปสอง "+marker+".txt", model.FileAttached)
	_ = createSearchFile(t, tx, secret.UUID, secretField.UUID, viewer.UUID, &secretOwnSubmission.UUID,
		"ไฟล์ลับของฉัน "+marker+".txt", model.FileAttached)
	_ = createSearchFile(t, tx, secret.UUID, secretField.UUID, submitter.UUID, &secretOtherSubmission.UUID,
		"ไฟล์ลับห้ามรั่ว "+marker+".txt", model.FileAttached)

	service := NewService(tx)
	regularProfile := auth.Profile{User: viewer}
	creatorProfile := auth.Profile{User: creator}
	headProfile := auth.Profile{User: head, HeadDivisions: []model.Division{divisions[0]}}
	executiveProfile := auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: executive.UUID}, IsExec: true}}
	adminOnlyProfile := auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: viewer.UUID}, IsAdmin: true}}
	unrelatedProfile := auth.Profile{User: unrelated}

	regular, err := service.Search(context.Background(), Input{
		Profile: regularProfile, Query: marker, Page: 1, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("Search(regular page 1) error = %v", err)
	}
	if regular.Total != 3 || regular.TotalPages != 2 || len(regular.Items) != 2 {
		t.Fatalf("regular page 1 = %+v, want total=3 pages=2 items=2", regular)
	}
	if regular.Items[0].SubmissionUUID != publicTwoSubmission.UUID || regular.Items[1].SubmissionUUID != secretOwnSubmission.UUID {
		t.Fatalf("regular page 1 order = %v", searchItemUUIDs(regular.Items))
	}
	regularSecondPage, err := service.Search(context.Background(), Input{
		Profile: regularProfile, Query: marker, Page: 2, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("Search(regular page 2) error = %v", err)
	}
	if regularSecondPage.Total != 3 || len(regularSecondPage.Items) != 1 || regularSecondPage.Items[0].SubmissionUUID != publicOneSubmission.UUID {
		t.Fatalf("regular page 2 = %+v", regularSecondPage)
	}
	if regularSecondPage.Items[0].Topic.IsOpen {
		t.Fatal("closed topic was omitted or returned as open")
	}
	if len(regularSecondPage.Items[0].Files) != 1 || regularSecondPage.Items[0].Files[0].UUID != publicFile.UUID {
		t.Fatalf("public search files = %+v, want only attached file %s", regularSecondPage.Items[0].Files, publicFile.UUID)
	}

	assertSearchVisible(t, service, creatorProfile, marker, []uuid.UUID{
		publicOneSubmission.UUID, publicTwoSubmission.UUID, secretOwnSubmission.UUID, secretOtherSubmission.UUID,
	})
	assertSearchVisible(t, service, headProfile, marker, []uuid.UUID{
		publicOneSubmission.UUID, publicTwoSubmission.UUID, secretOwnSubmission.UUID, secretOtherSubmission.UUID,
	})
	assertSearchVisible(t, service, executiveProfile, marker, []uuid.UUID{
		publicOneSubmission.UUID, publicTwoSubmission.UUID, secretOwnSubmission.UUID, secretOtherSubmission.UUID,
	})
	assertSearchVisible(t, service, adminOnlyProfile, marker, []uuid.UUID{
		publicOneSubmission.UUID, publicTwoSubmission.UUID, secretOwnSubmission.UUID,
	})
	assertSearchVisible(t, service, unrelatedProfile, marker, []uuid.UUID{
		publicOneSubmission.UUID, publicTwoSubmission.UUID,
	})

	assertSearchOne(t, service, Input{Profile: regularProfile, Query: "100%_ภาษาไทย"}, publicOneSubmission.UUID)
	assertSearchOne(t, service, Input{Profile: regularProfile, Query: "ค่าที่กรอกเฉพาะหนึ่ง"}, publicOneSubmission.UUID)
	assertSearchOne(t, service, Input{Profile: regularProfile, Query: "หลักฐานภาษาไทย " + marker}, publicOneSubmission.UUID)
	assertSearchOne(t, service, Input{
		Profile: regularProfile, Query: divisions[1].Name, Year: &secondYear.Year, By: &submitter.UUID,
	}, publicTwoSubmission.UUID)
	assertSearchOne(t, service, Input{
		Profile: regularProfile, Query: submitter.FullName, Year: &secondYear.Year,
	}, publicTwoSubmission.UUID)

	assertSearchEmpty(t, service, Input{Profile: regularProfile, Query: "คำตอบลับห้ามรั่ว"})
	assertSearchEmpty(t, service, Input{Profile: regularProfile, Query: "ไฟล์ลับห้ามรั่ว " + marker})
	assertSearchEmpty(t, service, Input{Profile: regularProfile, Query: "ไฟล์ staged ห้ามค้น " + marker})
	assertSearchOne(t, service, Input{Profile: creatorProfile, Query: "คำตอบลับห้ามรั่ว"}, secretOtherSubmission.UUID)
	assertSearchOne(t, service, Input{Profile: creatorProfile, Query: "ไฟล์ลับห้ามรั่ว " + marker}, secretOtherSubmission.UUID)

	assertSearchOne(t, service, Input{Profile: regularProfile, Query: marker, Division: divisions[1].Code}, publicTwoSubmission.UUID)
	assertSearchOne(t, service, Input{Profile: regularProfile, Query: marker, Year: &secondYear.Year}, publicTwoSubmission.UUID)
	semesterTwo := int16(2)
	assertSearchOne(t, service, Input{Profile: regularProfile, Query: marker, Semester: &semesterTwo}, publicTwoSubmission.UUID)
	assertSearchVisibleWithInput(t, service, Input{Profile: regularProfile, Query: marker, By: &submitter.UUID}, []uuid.UUID{
		publicOneSubmission.UUID, publicTwoSubmission.UUID,
	})
	assertSearchOne(t, service, Input{Profile: regularProfile, Query: marker, By: &viewer.UUID}, secretOwnSubmission.UUID)
}

func createSearchAcademicYear(t *testing.T, db *gorm.DB, excluded int16) model.AcademicYear {
	t.Helper()
	for candidate := int16(2400); candidate <= 3000; candidate++ {
		if candidate == excluded {
			continue
		}
		var count int64
		if err := db.Model(&model.AcademicYear{}).Where("year = ?", candidate).Count(&count).Error; err != nil {
			t.Fatalf("check academic year %d: %v", candidate, err)
		}
		if count == 0 {
			year := model.AcademicYear{Year: candidate, CurrentSemester: 1}
			if err := db.Create(&year).Error; err != nil {
				t.Fatalf("create academic year %d: %v", candidate, err)
			}
			return year
		}
	}
	t.Fatal("no unused academic year available")
	return model.AcademicYear{}
}

func createSearchUser(t *testing.T, db *gorm.DB, username, fullName string) model.User {
	t.Helper()
	user := model.User{Username: username, PasswordHash: "hash", FullName: fullName, Position: "ครู"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func createSearchTopic(
	t *testing.T,
	db *gorm.DB,
	division model.Division,
	year model.AcademicYear,
	creator model.User,
	semester int16,
	title, dueDate string,
	secret bool,
) (model.Topic, model.TopicField) {
	t.Helper()
	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		t.Fatalf("parse due date: %v", err)
	}
	topic := model.Topic{
		DivisionUUID: division.UUID, AcademicYearUUID: year.UUID, Semester: semester,
		Title: title, DueDate: due, IsOpen: true, IsSecret: secret, CreatedByUUID: creator.UUID,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("create topic %q: %v", title, err)
	}
	field := model.TopicField{
		TopicUUID: topic.UUID, Position: 1, Type: model.FieldFile,
		Label: "ไฟล์หลักฐาน", Required: false, Options: datatypes.JSON(`[]`),
	}
	if err := db.Create(&field).Error; err != nil {
		t.Fatalf("create field for topic %q: %v", title, err)
	}
	return topic, field
}

func createSearchSubmission(t *testing.T, db *gorm.DB, topicUUID, userUUID uuid.UUID, values string, submittedAt time.Time) model.Submission {
	t.Helper()
	submission := model.Submission{
		TopicUUID: topicUUID, UserUUID: userUUID, Values: datatypes.JSON(values),
		SubmittedAt: submittedAt, UpdatedAt: submittedAt,
	}
	if err := db.Create(&submission).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}
	return submission
}

func createSearchFile(
	t *testing.T,
	db *gorm.DB,
	topicUUID, fieldUUID, uploadedByUUID uuid.UUID,
	submissionUUID *uuid.UUID,
	originalName, status string,
) model.File {
	t.Helper()
	file := model.File{
		TopicUUID: topicUUID, FieldUUID: fieldUUID, SubmissionUUID: submissionUUID,
		UploadedByUUID: uploadedByUUID, ObjectKey: "be17/" + uuid.NewString(),
		OriginalName: originalName, MimeType: "application/octet-stream", SizeBytes: 10, Status: status,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create file %q: %v", originalName, err)
	}
	return file
}

func assertSearchOne(t *testing.T, service *Service, input Input, want uuid.UUID) {
	t.Helper()
	result, err := service.Search(context.Background(), input)
	if err != nil {
		t.Fatalf("Search(%+v) error = %v", input, err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].SubmissionUUID != want {
		t.Fatalf("Search(%+v) = total %d items %v, want %s", input, result.Total, searchItemUUIDs(result.Items), want)
	}
}

func assertSearchEmpty(t *testing.T, service *Service, input Input) {
	t.Helper()
	result, err := service.Search(context.Background(), input)
	if err != nil {
		t.Fatalf("Search(%+v) error = %v", input, err)
	}
	if result.Total != 0 || result.TotalPages != 0 || result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("Search(%+v) = %+v, want non-nil empty result", input, result)
	}
}

func assertSearchVisible(t *testing.T, service *Service, profile auth.Profile, query string, want []uuid.UUID) {
	t.Helper()
	assertSearchVisibleWithInput(t, service, Input{Profile: profile, Query: query}, want)
}

func assertSearchVisibleWithInput(t *testing.T, service *Service, input Input, want []uuid.UUID) {
	t.Helper()
	input.PageSize = MaxPageSize
	result, err := service.Search(context.Background(), input)
	if err != nil {
		t.Fatalf("Search(%+v) error = %v", input, err)
	}
	got := searchItemUUIDs(result.Items)
	sort.Slice(got, func(left, right int) bool { return got[left].String() < got[right].String() })
	sort.Slice(want, func(left, right int) bool { return want[left].String() < want[right].String() })
	if result.Total != int64(len(want)) || len(got) != len(want) {
		t.Fatalf("Search(%+v) total/items = %d/%v, want %d/%v", input, result.Total, got, len(want), want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Search(%+v) items = %v, want %v", input, got, want)
		}
	}
}

func searchItemUUIDs(items []Item) []uuid.UUID {
	values := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		values = append(values, item.SubmissionUUID)
	}
	return values
}
