package portfolio

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPortfolioServiceWithPostgreSQL(t *testing.T) {
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
	// fixture ของ BE-16 ทั้งหมดอยู่ใน transaction นี้และถูก rollback หลังทดสอบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var division model.Division
	if err := tx.Order("sort_order, code").First(&division).Error; err != nil {
		t.Fatalf("load division: %v", err)
	}
	var firstYear model.AcademicYear
	if err := tx.Order("year DESC").First(&firstYear).Error; err != nil {
		t.Fatalf("load academic year: %v", err)
	}
	secondYear := createPortfolioAcademicYear(t, tx, firstYear.Year)

	suffix := uuid.NewString()[:8]
	owner := createPortfolioUser(t, tx, "be16.owner."+suffix, "เจ้าของแฟ้ม BE-16")
	other := createPortfolioUser(t, tx, "be16.other."+suffix, "ผู้ใช้อื่น BE-16")
	creator := createPortfolioUser(t, tx, "be16.creator."+suffix, "ผู้สร้าง BE-16")

	firstTopic, firstField := createPortfolioTopic(
		t, tx, division, firstYear, creator,
		"แบบรายงาน 100%_ภาษาไทย "+suffix, "2026-10-12", false,
	)
	secondTopic, secondField := createPortfolioTopic(
		t, tx, division, secondYear, creator,
		"รายงานปีอื่น "+suffix, "2026-10-20", true,
	)
	otherTopic, otherField := createPortfolioTopic(
		t, tx, division, firstYear, creator,
		"ข้อมูลของผู้อื่น "+suffix, "2026-10-20", true,
	)

	firstSubmission := createPortfolioSubmission(
		t, tx, firstTopic.UUID, owner.UUID,
		`{"note":"ข้อความภาษาไทยสำหรับค้นหา"}`,
		time.Date(2026, time.October, 13, 3, 0, 0, 0, time.UTC),
	)
	secondSubmission := createPortfolioSubmission(
		t, tx, secondTopic.UUID, owner.UUID,
		`{"note":"ข้อมูลปีอื่น"}`,
		time.Date(2026, time.October, 11, 3, 0, 0, 0, time.UTC),
	)
	otherSubmission := createPortfolioSubmission(
		t, tx, otherTopic.UUID, other.UUID,
		`{"note":"ข้อความภาษาไทยสำหรับค้นหา"}`,
		time.Date(2026, time.October, 10, 3, 0, 0, 0, time.UTC),
	)

	firstContent := []byte("first-owner-content")
	secondContent := []byte("second-owner-content")
	firstFile := createPortfolioFile(t, tx, firstTopic.UUID, firstField.UUID, owner.UUID, &firstSubmission.UUID,
		"be16/first", "หลักฐานภาษาไทย.pdf", model.FileAttached, int64(len(firstContent)))
	secondFile := createPortfolioFile(t, tx, secondTopic.UUID, secondField.UUID, owner.UUID, &secondSubmission.UUID,
		"be16/second", "เอกสารปีอื่น.txt", model.FileAttached, int64(len(secondContent)))
	_ = createPortfolioFile(t, tx, firstTopic.UUID, firstField.UUID, owner.UUID, nil,
		"be16/staged", "ไฟล์ที่ยังไม่ส่ง.txt", model.FileStaged, 10)
	_ = createPortfolioFile(t, tx, firstTopic.UUID, firstField.UUID, owner.UUID, &firstSubmission.UUID,
		"be16/trashed", "ไฟล์ที่ลบแล้ว.txt", model.FileTrashed, 10)
	_ = createPortfolioFile(t, tx, otherTopic.UUID, otherField.UUID, other.UUID, &otherSubmission.UUID,
		"be16/other", "ไฟล์ของคนอื่น.txt", model.FileAttached, 10)

	store := &trackingStore{objects: map[string][]byte{
		"be16/first":  firstContent,
		"be16/second": secondContent,
		"be16/other":  []byte("other-user-content"),
	}}
	service := NewService(tx, store)
	ctx := context.Background()

	all, err := service.List(ctx, ListInput{UserUUID: owner.UUID})
	if err != nil {
		t.Fatalf("List(all) error = %v", err)
	}
	if len(all.Years) != 2 {
		t.Fatalf("year groups = %d, want 2: %+v", len(all.Years), all.Years)
	}
	if all.Years[0].AcademicYear < all.Years[1].AcademicYear {
		t.Fatalf("year groups are not descending: %+v", all.Years)
	}
	firstItem := findPortfolioSubmission(t, all, firstSubmission.UUID)
	if firstItem.IsOpen {
		t.Fatal("closed historic topic was returned as open")
	}
	if firstItem.Status != StatusLate {
		t.Fatalf("first submission status = %q, want %q", firstItem.Status, StatusLate)
	}
	if len(firstItem.Files) != 1 || firstItem.Files[0].UUID != firstFile.UUID {
		t.Fatalf("first submission files = %+v, want only attached file %s", firstItem.Files, firstFile.UUID)
	}
	secondItem := findPortfolioSubmission(t, all, secondSubmission.UUID)
	if len(secondItem.Files) != 1 || secondItem.Files[0].UUID != secondFile.UUID {
		t.Fatalf("second submission files = %+v", secondItem.Files)
	}

	assertPortfolioSearch(t, service, owner.UUID, "100%_ภาษาไทย", firstSubmission.UUID)
	assertPortfolioSearch(t, service, owner.UUID, "ข้อความภาษาไทย", firstSubmission.UUID)
	assertPortfolioSearch(t, service, owner.UUID, "หลักฐานภาษาไทย", firstSubmission.UUID)

	none, err := service.List(ctx, ListInput{UserUUID: owner.UUID, Query: "ไม่มีข้อมูลนี้"})
	if err != nil {
		t.Fatalf("List(no match) error = %v", err)
	}
	if none.Years == nil || len(none.Years) != 0 {
		t.Fatalf("no-match years = %#v, want non-nil empty slice", none.Years)
	}

	filtered, err := service.List(ctx, ListInput{UserUUID: owner.UUID, Year: &secondYear.Year})
	if err != nil {
		t.Fatalf("List(year) error = %v", err)
	}
	if len(filtered.Years) != 1 || filtered.Years[0].AcademicYear != secondYear.Year || len(filtered.Years[0].Submissions) != 1 {
		t.Fatalf("year-filtered portfolio = %+v", filtered)
	}

	allArchive, err := service.OpenArchive(ctx, ArchiveInput{UserUUID: owner.UUID})
	if err != nil {
		t.Fatalf("OpenArchive(all) error = %v", err)
	}
	if allArchive.Filename != "sdms-files-all.zip" {
		t.Fatalf("all archive filename = %q", allArchive.Filename)
	}
	allEntries := readPortfolioArchive(t, allArchive.Content)
	if len(allEntries) != 2 {
		t.Fatalf("all archive entries = %v, want 2", mapKeys(allEntries))
	}
	assertArchiveContains(t, allEntries, firstFile.UUID, firstContent)
	assertArchiveContains(t, allEntries, secondFile.UUID, secondContent)

	yearArchive, err := service.OpenArchive(ctx, ArchiveInput{UserUUID: owner.UUID, Year: &firstYear.Year})
	if err != nil {
		t.Fatalf("OpenArchive(year) error = %v", err)
	}
	if yearArchive.Filename != fmt.Sprintf("sdms-files-%d.zip", firstYear.Year) {
		t.Fatalf("year archive filename = %q", yearArchive.Filename)
	}
	yearEntries := readPortfolioArchive(t, yearArchive.Content)
	if len(yearEntries) != 1 {
		t.Fatalf("year archive entries = %v, want 1", mapKeys(yearEntries))
	}
	assertArchiveContains(t, yearEntries, firstFile.UUID, firstContent)

	store.mu.Lock()
	maxActive := store.maxActive
	active := store.active
	store.mu.Unlock()
	if maxActive != 1 || active != 0 {
		t.Fatalf("storage streams: max=%d active=%d, want max=1 active=0", maxActive, active)
	}
}

func createPortfolioAcademicYear(t *testing.T, db *gorm.DB, excluded int16) model.AcademicYear {
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

func createPortfolioUser(t *testing.T, db *gorm.DB, username, fullName string) model.User {
	t.Helper()
	user := model.User{Username: username, PasswordHash: "hash", FullName: fullName, Position: "ครู"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func createPortfolioTopic(
	t *testing.T,
	db *gorm.DB,
	division model.Division,
	year model.AcademicYear,
	creator model.User,
	title string,
	dueDate string,
	open bool,
) (model.Topic, model.TopicField) {
	t.Helper()
	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		t.Fatalf("parse due date: %v", err)
	}
	topic := model.Topic{
		DivisionUUID: division.UUID, AcademicYearUUID: year.UUID, Semester: 1,
		Title: title, DueDate: due, IsOpen: true, CreatedByUUID: creator.UUID,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("create topic %q: %v", title, err)
	}
	if !open {
		if err := db.Model(&model.Topic{}).Where("uuid = ?", topic.UUID).Update("is_open", false).Error; err != nil {
			t.Fatalf("close topic %q: %v", title, err)
		}
		topic.IsOpen = false
	}
	field := model.TopicField{
		TopicUUID: topic.UUID, Position: 1, Type: model.FieldFile,
		Label: "ไฟล์หลักฐาน", Required: false, Options: datatypes.JSON(`[]`),
	}
	if err := db.Create(&field).Error; err != nil {
		t.Fatalf("create topic field: %v", err)
	}
	return topic, field
}

func createPortfolioSubmission(t *testing.T, db *gorm.DB, topicUUID, userUUID uuid.UUID, values string, submittedAt time.Time) model.Submission {
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

func createPortfolioFile(
	t *testing.T,
	db *gorm.DB,
	topicUUID, fieldUUID, uploadedByUUID uuid.UUID,
	submissionUUID *uuid.UUID,
	objectKey, originalName, status string,
	size int64,
) model.File {
	t.Helper()
	file := model.File{
		TopicUUID: topicUUID, FieldUUID: fieldUUID, SubmissionUUID: submissionUUID,
		UploadedByUUID: uploadedByUUID, ObjectKey: objectKey, OriginalName: originalName,
		MimeType: "application/octet-stream", SizeBytes: size, Status: status,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create file %q: %v", originalName, err)
	}
	return file
}

func assertPortfolioSearch(t *testing.T, service *Service, userUUID uuid.UUID, query string, wantSubmission uuid.UUID) {
	t.Helper()
	result, err := service.List(context.Background(), ListInput{UserUUID: userUUID, Query: query})
	if err != nil {
		t.Fatalf("List(q=%q) error = %v", query, err)
	}
	if len(result.Years) != 1 || len(result.Years[0].Submissions) != 1 || result.Years[0].Submissions[0].UUID != wantSubmission {
		t.Fatalf("List(q=%q) = %+v, want submission %s", query, result, wantSubmission)
	}
}

func findPortfolioSubmission(t *testing.T, portfolio Portfolio, submissionUUID uuid.UUID) Submission {
	t.Helper()
	for _, year := range portfolio.Years {
		for _, submission := range year.Submissions {
			if submission.UUID == submissionUUID {
				return submission
			}
		}
	}
	t.Fatalf("submission %s not found in %+v", submissionUUID, portfolio)
	return Submission{}
}

func readPortfolioArchive(t *testing.T, content io.ReadCloser) map[string][]byte {
	t.Helper()
	compressed, err := io.ReadAll(content)
	if err != nil {
		t.Fatalf("read archive stream: %v", err)
	}
	if err := content.Close(); err != nil {
		t.Fatalf("close archive stream: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(compressed), int64(len(compressed)))
	if err != nil {
		t.Fatalf("open ZIP: %v", err)
	}
	entries := make(map[string][]byte, len(reader.File))
	for _, entry := range reader.File {
		stream, err := entry.Open()
		if err != nil {
			t.Fatalf("open ZIP entry %q: %v", entry.Name, err)
		}
		body, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatalf("read ZIP entry %q: %v", entry.Name, err)
		}
		entries[entry.Name] = body
	}
	return entries
}

func assertArchiveContains(t *testing.T, entries map[string][]byte, fileUUID uuid.UUID, want []byte) {
	t.Helper()
	for name, content := range entries {
		if strings.Contains(name, fileUUID.String()+"_") {
			if !bytes.Equal(content, want) {
				t.Fatalf("entry %q content = %q, want %q", name, content, want)
			}
			return
		}
	}
	t.Fatalf("archive does not contain file %s: %v", fileUUID, mapKeys(entries))
}

func mapKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
