package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUploadServiceWithPostgreSQL(t *testing.T) {
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
	// ทุก row ของ BE-11 อยู่ใน outer transaction นี้และถูก rollback เมื่อ test จบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var division model.Division
	if err := tx.Order("sort_order").First(&division).Error; err != nil {
		t.Fatalf("find division: %v", err)
	}
	suffix := uuid.NewString()[:8]
	user := model.User{
		Username:     "be11." + suffix,
		PasswordHash: "integration-hash",
		FullName:     "ผู้ส่งไฟล์ BE-11",
		Position:     "ครู",
	}
	outsider := model.User{
		Username:     "be11.other." + suffix,
		PasswordHash: "integration-hash",
		FullName:     "ผู้ใช้นอก snapshot",
		Position:     "ครู",
	}
	if err := tx.Create(&[]model.User{user, outsider}).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	// GORM เติม UUID กลับเข้า slice เท่านั้น จึงโหลดด้วย username เพื่อใช้ค่าที่แน่นอน.
	if err := tx.Where("username = ?", user.Username).Take(&user).Error; err != nil {
		t.Fatalf("reload assignee: %v", err)
	}
	if err := tx.Where("username = ?", outsider.Username).Take(&outsider).Error; err != nil {
		t.Fatalf("reload outsider: %v", err)
	}
	if err := tx.Create(&model.UserDivision{UserUUID: user.UUID, DivisionUUID: division.UUID}).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}

	topicService := topic.NewService(tx)
	createdTopic, err := topicService.Create(context.Background(), topic.CreateInput{
		DivisionUUID:  division.UUID,
		CreatedByUUID: user.UUID,
		Title:         "เอกสาร BE-11 " + suffix,
		DueDate:       "2026-10-31",
		Fields: []topic.FieldInput{
			{Position: 1, Type: model.FieldFile, Label: "หลักฐาน", Required: true},
			{Position: 2, Type: model.FieldText, Label: "หมายเหตุ", Required: false},
		},
	})
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}

	store := newMemoryStore()
	service := NewService(tx, store, 24*time.Hour)
	uploaded, err := service.Upload(context.Background(), UploadInput{
		TopicUUID:      createdTopic.UUID,
		FieldUUID:      createdTopic.Fields[0].UUID,
		UploadedByUUID: user.UUID,
		OriginalName:   `C:\fakepath\หลักฐาน.PDF`,
		MimeType:       "application/pdf",
		SizeHint:       8,
		Content:        strings.NewReader("document"),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if uploaded.UUID == uuid.Nil || uploaded.Status != model.FileStaged || uploaded.SizeBytes != 8 {
		t.Fatalf("uploaded file = %+v", uploaded)
	}
	if uploaded.OriginalName != "หลักฐาน.PDF" || !strings.HasSuffix(store.lastPutName(), ".pdf") {
		t.Fatalf("original/storage names = %q / %q", uploaded.OriginalName, store.lastPutName())
	}
	if got, want := string(store.file(uploaded.ObjectKey)), "document"; got != want {
		t.Fatalf("stored content = %q, want %q", got, want)
	}
	wantDivisionFolder := "root/" + strconv.Itoa(int(createdTopic.AcademicYear.Year)) + "/aca"
	if !store.hasFolderPath(wantDivisionFolder) {
		t.Fatalf("folder IDs = %v, want year/division hierarchy", store.folderIDs())
	}
	var persisted model.File
	if err := tx.Where("uuid = ?", uploaded.UUID).Take(&persisted).Error; err != nil {
		t.Fatalf("find uploaded metadata: %v", err)
	}
	if persisted.SubmissionUUID != nil || persisted.Status != model.FileStaged {
		t.Fatalf("persisted file = %+v", persisted)
	}

	_, err = service.Upload(context.Background(), UploadInput{
		TopicUUID:      createdTopic.UUID,
		FieldUUID:      createdTopic.Fields[0].UUID,
		UploadedByUUID: outsider.UUID,
		OriginalName:   "forbidden.pdf",
		MimeType:       "application/pdf",
		SizeHint:       1,
		Content:        strings.NewReader("x"),
	})
	if !errors.Is(err, ErrNotAssignee) {
		t.Fatalf("Upload(outsider) error = %v, want ErrNotAssignee", err)
	}

	downloadable, err := service.Upload(context.Background(), UploadInput{
		TopicUUID:      createdTopic.UUID,
		FieldUUID:      createdTopic.Fields[0].UUID,
		UploadedByUUID: user.UUID,
		OriginalName:   "download.pdf",
		MimeType:       "application/pdf",
		SizeHint:       15,
		Content:        strings.NewReader("download-stream"),
	})
	if err != nil {
		t.Fatalf("Upload(download fixture) error = %v", err)
	}
	submission := model.Submission{
		TopicUUID:   createdTopic.UUID,
		UserUUID:    user.UUID,
		SubmittedAt: time.Now().UTC(),
	}
	if err := tx.Create(&submission).Error; err != nil {
		t.Fatalf("create download submission: %v", err)
	}
	if err := tx.Model(&model.File{}).Where("uuid = ?", downloadable.UUID).Updates(map[string]any{
		"status":          model.FileAttached,
		"submission_uuid": submission.UUID,
	}).Error; err != nil {
		t.Fatalf("attach download file: %v", err)
	}

	publicDownload, err := service.Open(context.Background(), downloadable.UUID, auth.Profile{User: outsider})
	if err != nil {
		t.Fatalf("Open(public file as authenticated outsider) error = %v", err)
	}
	publicContent, err := io.ReadAll(publicDownload.Content)
	if err != nil {
		t.Fatalf("read public download: %v", err)
	}
	if err := publicDownload.Content.Close(); err != nil {
		t.Fatalf("close public download: %v", err)
	}
	if got, want := string(publicContent), "download-stream"; got != want {
		t.Fatalf("public download = %q, want %q", got, want)
	}

	if err := tx.Model(&model.Topic{}).Where("uuid = ?", createdTopic.UUID).Update("is_secret", true).Error; err != nil {
		t.Fatalf("make topic secret: %v", err)
	}
	if _, err := service.Open(context.Background(), downloadable.UUID, auth.Profile{User: outsider}); !errors.Is(err, ErrFileForbidden) {
		t.Fatalf("Open(secret file as outsider) error = %v, want ErrFileForbidden", err)
	}
	if _, err := service.Open(context.Background(), downloadable.UUID, auth.Profile{User: model.User{
		UUIDModel: model.UUIDModel{UUID: outsider.UUID},
		IsAdmin:   true,
	}}); !errors.Is(err, ErrFileForbidden) {
		t.Fatalf("Open(secret file as admin only) error = %v, want ErrFileForbidden", err)
	}
	ownerDownload, err := service.Open(context.Background(), downloadable.UUID, auth.Profile{User: user})
	if err != nil {
		t.Fatalf("Open(secret own file) error = %v", err)
	}
	_ = ownerDownload.Content.Close()
	executiveDownload, err := service.Open(context.Background(), downloadable.UUID, auth.Profile{User: model.User{
		UUIDModel: model.UUIDModel{UUID: outsider.UUID},
		IsExec:    true,
	}})
	if err != nil {
		t.Fatalf("Open(secret file as executive) error = %v", err)
	}
	_ = executiveDownload.Content.Close()
	if _, err := service.Open(context.Background(), uploaded.UUID, auth.Profile{User: user}); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("Open(staged file) error = %v, want ErrFileNotFound", err)
	}

	if err := service.Delete(context.Background(), uploaded.UUID, outsider.UUID); !errors.Is(err, ErrFileForbidden) {
		t.Fatalf("Delete(outsider) error = %v, want ErrFileForbidden", err)
	}
	if err := service.Delete(context.Background(), uploaded.UUID, user.UUID); err != nil {
		t.Fatalf("Delete(owner) error = %v", err)
	}
	if !store.wasTrashed(uploaded.ObjectKey) {
		t.Fatalf("file %q was not trashed", uploaded.ObjectKey)
	}
	if err := tx.Select("status").Where("uuid = ?", uploaded.UUID).Take(&persisted).Error; err != nil {
		t.Fatalf("reload deleted metadata: %v", err)
	}
	if persisted.Status != model.FileTrashed {
		t.Fatalf("status after Delete = %q, want trashed", persisted.Status)
	}

	expired, err := service.Upload(context.Background(), UploadInput{
		TopicUUID:      createdTopic.UUID,
		FieldUUID:      createdTopic.Fields[0].UUID,
		UploadedByUUID: user.UUID,
		OriginalName:   "expired.txt",
		MimeType:       "text/plain",
		SizeHint:       3,
		Content:        strings.NewReader("old"),
	})
	if err != nil {
		t.Fatalf("Upload(expired fixture) error = %v", err)
	}
	fixedNow := time.Date(2026, time.October, 13, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	if err := tx.Model(&model.File{}).Where("uuid = ?", expired.UUID).
		Update("created_at", fixedNow.Add(-25*time.Hour)).Error; err != nil {
		t.Fatalf("age staged upload: %v", err)
	}
	cleaned, err := service.CleanupExpired(context.Background())
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if cleaned != 1 || !store.wasTrashed(expired.ObjectKey) {
		t.Fatalf("cleanup = %d, trashed = %v; want 1/true", cleaned, store.wasTrashed(expired.ObjectKey))
	}
}

type memoryStore struct {
	mu        sync.Mutex
	folders   map[string]string
	files     map[string][]byte
	trashed   map[string]bool
	putNames  []string
	nextIndex int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		folders: map[string]string{},
		files:   map[string][]byte{},
		trashed: map[string]bool{},
	}
}

func (s *memoryStore) RootID() string { return "root" }

func (s *memoryStore) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := parentID + "/" + name
	if id := s.folders[key]; id != "" {
		return id, nil
	}
	s.folders[key] = key
	return key, nil
}

func (s *memoryStore) Put(_ context.Context, folderID, name, _ string, _ int64, content io.Reader) (string, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextIndex++
	id := fmt.Sprintf("file-%d", s.nextIndex)
	s.files[id] = bytes.Clone(data)
	s.putNames = append(s.putNames, name)
	return id, nil
}

func (s *memoryStore) Open(_ context.Context, fileID string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, ok := s.files[fileID]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(content))), nil
}

func (s *memoryStore) Trash(_ context.Context, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trashed[fileID] = true
	delete(s.files, fileID)
	return nil
}

func (s *memoryStore) lastPutName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putNames[len(s.putNames)-1]
}

func (s *memoryStore) file(id string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.files[id])
}

func (s *memoryStore) wasTrashed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trashed[id]
}

func (s *memoryStore) hasFolderPath(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, folderID := range s.folders {
		if folderID == id {
			return true
		}
	}
	return false
}

func (s *memoryStore) folderIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0, len(s.folders))
	for _, id := range s.folders {
		result = append(result, id)
	}
	return result
}
