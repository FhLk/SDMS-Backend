package upload

import (
	"context"
	"fmt"
	"io"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/authorization"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OpenedFile รวม metadata ที่ใช้สร้าง response header กับ stream จาก storage.
// ObjectKey ไม่ถูกส่งออกเป็น JSON จึงไม่เปิดเผยตำแหน่งจริงของ local storage หรือ R2.
type OpenedFile struct {
	File    model.File
	Content io.ReadCloser
}

// Open ตรวจสิทธิ์ด้วยกติกาเดียวกับตารางคำตอบก่อนเปิด stream จาก storage.
// การล็อกแบบ SHARE ทำให้การแก้ submission ไม่สามารถ trash ไฟล์ระหว่างตรวจสิทธิ์และเปิด stream ได้.
func (s *Service) Open(ctx context.Context, fileUUID uuid.UUID, profile auth.Profile) (OpenedFile, error) {
	if fileUUID == uuid.Nil {
		return OpenedFile{}, ErrInvalidUpload
	}
	if profile.User.UUID == uuid.Nil {
		return OpenedFile{}, ErrFileForbidden
	}

	var opened OpenedFile
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.File
		result := tx.Clauses(clause.Locking{Strength: "SHARE"}).
			Where("uuid = ? AND status = ?", fileUUID, model.FileAttached).
			Limit(1).Find(&file)
		if result.Error != nil {
			return fmt.Errorf("find downloadable file: %w", result.Error)
		}
		if result.RowsAffected == 0 || file.SubmissionUUID == nil {
			return ErrFileNotFound
		}

		var topic model.Topic
		result = tx.Clauses(clause.Locking{Strength: "SHARE"}).
			Select("uuid", "division_uuid", "created_by_uuid", "is_secret").
			Where("uuid = ?", file.TopicUUID).Limit(1).Find(&topic)
		if result.Error != nil {
			return fmt.Errorf("find downloadable file topic: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrFileNotFound
		}

		var submission model.Submission
		result = tx.Clauses(clause.Locking{Strength: "SHARE"}).
			Select("uuid", "topic_uuid", "user_uuid").
			Where("uuid = ? AND topic_uuid = ?", *file.SubmissionUUID, file.TopicUUID).
			Limit(1).Find(&submission)
		if result.Error != nil {
			return fmt.Errorf("find downloadable file submission: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrFileNotFound
		}

		if !authorization.CanViewAllTopicSubmissions(profile, topic) && submission.UserUUID != profile.User.UUID {
			return ErrFileForbidden
		}

		content, err := s.store.Open(ctx, file.ObjectKey)
		if err != nil {
			return fmt.Errorf("open downloadable file: %w", err)
		}
		opened = OpenedFile{File: file, Content: content}
		return nil
	})
	if err != nil {
		if opened.Content != nil {
			_ = opened.Content.Close()
		}
		return OpenedFile{}, err
	}
	return opened, nil
}
