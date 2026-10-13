package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MaxFileSize       int64 = 50 * 1024 * 1024
	cleanupBatchLimit       = 100
)

var (
	ErrTopicNotFound = errors.New("upload topic not found")
	ErrTopicClosed   = errors.New("upload topic is closed")
	ErrFieldNotFound = errors.New("upload field not found")
	ErrFieldNotFile  = errors.New("upload field does not accept files")
	ErrNotAssignee   = errors.New("user is not assigned to topic")
	ErrFileNotFound  = errors.New("uploaded file not found")
	ErrFileForbidden = errors.New("uploaded file belongs to another user")
	ErrFileAttached  = errors.New("attached file cannot be deleted from upload endpoint")
	ErrFileTooLarge  = errors.New("uploaded file exceeds 50 MiB")
	ErrFileEmpty     = errors.New("uploaded file is empty")
	ErrInvalidUpload = errors.New("invalid upload input")
)

type UploadInput struct {
	TopicUUID      uuid.UUID
	FieldUUID      uuid.UUID
	UploadedByUUID uuid.UUID
	OriginalName   string
	MimeType       string
	SizeHint       int64
	Content        io.Reader
}

type Service struct {
	db        *gorm.DB
	store     ObjectStore
	stagedTTL time.Duration
	now       func() time.Time
}

func NewService(db *gorm.DB, store ObjectStore, stagedTTL time.Duration) *Service {
	return &Service{db: db, store: store, stagedTTL: stagedTTL, now: time.Now}
}

// Upload ตรวจสิทธิ์และปลายทางก่อนส่ง stream ไป storage จากนั้นบันทึก metadata แบบ staged.
// ถ้าฐานข้อมูลบันทึกไม่สำเร็จ จะ trash object ที่สร้างแล้วเพื่อไม่ทิ้งไฟล์กำพร้า.
func (s *Service) Upload(ctx context.Context, input UploadInput) (model.File, error) {
	input.OriginalName = cleanOriginalName(input.OriginalName)
	input.MimeType = strings.TrimSpace(input.MimeType)
	if input.MimeType == "" {
		input.MimeType = "application/octet-stream"
	}
	if err := validateUploadInput(input); err != nil {
		return model.File{}, err
	}

	target, err := s.loadTarget(ctx, input.TopicUUID, input.FieldUUID, input.UploadedByUUID)
	if err != nil {
		return model.File{}, err
	}
	folderID, err := s.ensureFolderHierarchy(ctx, &target)
	if err != nil {
		return model.File{}, err
	}

	fileUUID, err := uuid.NewV7()
	if err != nil {
		return model.File{}, fmt.Errorf("generate upload UUID: %w", err)
	}
	stagedContent, actualSize, err := spoolUpload(input.Content)
	if err != nil {
		return model.File{}, err
	}
	defer func() {
		_ = stagedContent.Close()
		_ = os.Remove(stagedContent.Name())
	}()

	storageName := fileUUID.String() + safeExtension(input.OriginalName)
	objectKey, err := s.store.Put(ctx, folderID, storageName, input.MimeType, actualSize, stagedContent)
	if err != nil {
		return model.File{}, err
	}

	result := model.File{
		UUIDModel:      model.UUIDModel{UUID: fileUUID},
		TopicUUID:      input.TopicUUID,
		FieldUUID:      input.FieldUUID,
		UploadedByUUID: input.UploadedByUUID,
		ObjectKey:      objectKey,
		OriginalName:   input.OriginalName,
		MimeType:       input.MimeType,
		SizeBytes:      actualSize,
		Status:         model.FileStaged,
	}
	// ตรวจสถานะซ้ำใน transaction ป้องกันหัวข้อปิดหรือ field ถูกซ่อนระหว่าง upload ไป storage.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateTarget(tx, input.TopicUUID, input.FieldUUID, input.UploadedByUUID, true); err != nil {
			return err
		}
		if err := tx.Create(&result).Error; err != nil {
			return fmt.Errorf("create uploaded file metadata: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.File{}, s.trashAfterRejectedUpload(ctx, objectKey, err)
	}
	return result, nil
}

// Delete ใช้ row lock กัน BE-12 นำไฟล์ไป attach พร้อมกับการลบไฟล์ staged.
func (s *Service) Delete(ctx context.Context, fileUUID, userUUID uuid.UUID) error {
	if fileUUID == uuid.Nil || userUUID == uuid.Nil {
		return ErrInvalidUpload
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.File
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uuid = ?", fileUUID).Limit(1).Find(&file)
		if query.Error != nil {
			return fmt.Errorf("find uploaded file: %w", query.Error)
		}
		if query.RowsAffected == 0 {
			return ErrFileNotFound
		}
		if file.UploadedByUUID != userUUID {
			return ErrFileForbidden
		}
		switch file.Status {
		case model.FileTrashed:
			return nil
		case model.FileAttached:
			return ErrFileAttached
		case model.FileStaged:
			if err := s.store.Trash(ctx, file.ObjectKey); err != nil {
				return err
			}
			if err := tx.Model(&file).Update("status", model.FileTrashed).Error; err != nil {
				return fmt.Errorf("mark uploaded file trashed: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("unknown file status %q", file.Status)
		}
	})
}

// CleanupExpired trash ไฟล์ staged ที่ไม่ได้ผูกกับ submission ภายใน TTL.
// จำกัดจำนวนต่อรอบเพื่อไม่ให้ process cleanup ยึด connection เป็นเวลานานเกินไป.
func (s *Service) CleanupExpired(ctx context.Context) (int, error) {
	cutoff := s.now().UTC().Add(-s.stagedTTL)
	cleaned := 0
	for cleaned < cleanupBatchLimit {
		found, err := s.cleanupOne(ctx, cutoff)
		if err != nil {
			return cleaned, err
		}
		if !found {
			break
		}
		cleaned++
	}
	return cleaned, nil
}

func (s *Service) cleanupOne(ctx context.Context, cutoff time.Time) (bool, error) {
	found := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.File
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND created_at < ?", model.FileStaged, cutoff).
			Order("created_at, uuid").Limit(1).Find(&file)
		if query.Error != nil {
			return fmt.Errorf("find expired staged upload: %w", query.Error)
		}
		if query.RowsAffected == 0 {
			return nil
		}
		found = true
		if err := s.store.Trash(ctx, file.ObjectKey); err != nil {
			return err
		}
		if err := tx.Model(&file).Update("status", model.FileTrashed).Error; err != nil {
			return fmt.Errorf("mark expired upload trashed: %w", err)
		}
		return nil
	})
	return found, err
}

type uploadTarget struct {
	topic model.Topic
}

func (s *Service) loadTarget(ctx context.Context, topicUUID, fieldUUID, userUUID uuid.UUID) (uploadTarget, error) {
	if topicUUID == uuid.Nil || fieldUUID == uuid.Nil || userUUID == uuid.Nil {
		return uploadTarget{}, ErrInvalidUpload
	}
	var topic model.Topic
	query := s.db.WithContext(ctx).
		Preload("AcademicYear").Preload("Division").
		Where("uuid = ?", topicUUID).Limit(1).Find(&topic)
	if query.Error != nil {
		return uploadTarget{}, fmt.Errorf("find upload topic: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return uploadTarget{}, ErrTopicNotFound
	}
	if err := validateTarget(s.db.WithContext(ctx), topicUUID, fieldUUID, userUUID, false); err != nil {
		return uploadTarget{}, err
	}
	return uploadTarget{topic: topic}, nil
}

func validateTarget(db *gorm.DB, topicUUID, fieldUUID, userUUID uuid.UUID, lock bool) error {
	var topic model.Topic
	query := db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "SHARE"})
	}
	result := query.Select("uuid", "is_open").Where("uuid = ?", topicUUID).Limit(1).Find(&topic)
	if result.Error != nil {
		return fmt.Errorf("validate upload topic: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTopicNotFound
	}
	if !topic.IsOpen {
		return ErrTopicClosed
	}

	var field model.TopicField
	result = db.Select("uuid", "type", "hidden_at").
		Where("uuid = ? AND topic_uuid = ?", fieldUUID, topicUUID).Limit(1).Find(&field)
	if result.Error != nil {
		return fmt.Errorf("validate upload field: %w", result.Error)
	}
	if result.RowsAffected == 0 || field.HiddenAt != nil {
		return ErrFieldNotFound
	}
	if field.Type != model.FieldFile {
		return ErrFieldNotFile
	}

	var assigneeCount int64
	if err := db.Model(&model.TopicAssignee{}).
		Where("topic_uuid = ? AND user_uuid = ?", topicUUID, userUUID).
		Count(&assigneeCount).Error; err != nil {
		return fmt.Errorf("validate upload assignee: %w", err)
	}
	if assigneeCount == 0 {
		return ErrNotAssignee
	}
	return nil
}

func (s *Service) ensureFolderHierarchy(ctx context.Context, target *uploadTarget) (string, error) {
	yearName := strconv.Itoa(int(target.topic.AcademicYear.Year))
	yearID, err := s.store.EnsureFolder(ctx, s.store.RootID(), yearName)
	if err != nil {
		return "", err
	}
	divisionID, err := s.store.EnsureFolder(ctx, yearID, target.topic.Division.Code)
	if err != nil {
		return "", err
	}
	// ใช้ UUID เป็น prefix ของหัวข้อเพื่อให้คงที่แม้เปลี่ยนชื่อ และไม่ชนกับหัวข้อชื่อซ้ำ.
	return s.store.EnsureFolder(ctx, divisionID, target.topic.UUID.String())
}

func (s *Service) trashAfterRejectedUpload(ctx context.Context, objectID string, cause error) error {
	if trashErr := s.store.Trash(ctx, objectID); trashErr != nil {
		return errors.Join(cause, trashErr)
	}
	return cause
}

func validateUploadInput(input UploadInput) error {
	if input.TopicUUID == uuid.Nil || input.FieldUUID == uuid.Nil || input.UploadedByUUID == uuid.Nil || input.Content == nil {
		return ErrInvalidUpload
	}
	if input.OriginalName == "" || utf8.RuneCountInString(input.OriginalName) > 255 {
		return ErrInvalidUpload
	}
	if utf8.RuneCountInString(input.MimeType) > 120 {
		return ErrInvalidUpload
	}
	if input.SizeHint > MaxFileSize {
		return ErrFileTooLarge
	}
	if input.SizeHint == 0 {
		return ErrFileEmpty
	}
	return nil
}

func cleanOriginalName(value string) string {
	// แปลง backslash ก่อนใช้ path.Base เพื่อรองรับชื่อจากทั้ง Windows และ Unix client.
	return strings.TrimSpace(path.Base(strings.ReplaceAll(value, `\`, "/")))
}

func safeExtension(filename string) string {
	extension := strings.ToLower(path.Ext(filename))
	if len(extension) < 2 || len(extension) > 16 {
		return ""
	}
	for _, char := range extension[1:] {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return ""
		}
	}
	return extension
}

func spoolUpload(content io.Reader) (*os.File, int64, error) {
	temporary, err := os.CreateTemp("", "sdms-upload-*")
	if err != nil {
		return nil, 0, fmt.Errorf("create temporary upload: %w", err)
	}
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
	}

	// พักไฟล์ไม่เกิน limit+1 ลงไฟล์ชั่วคราว เพื่อรู้ขนาดจริงและให้ AWS SDK retry stream ได้.
	size, err := io.Copy(temporary, io.LimitReader(content, MaxFileSize+1))
	if err != nil {
		cleanup()
		return nil, 0, fmt.Errorf("stage uploaded file: %w", err)
	}
	if size > MaxFileSize {
		cleanup()
		return nil, 0, ErrFileTooLarge
	}
	if size == 0 {
		cleanup()
		return nil, 0, ErrFileEmpty
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, 0, fmt.Errorf("rewind temporary upload: %w", err)
	}
	return temporary, size, nil
}
