// Package submission จัดการการส่งคำตอบ การแก้คำตอบ และการผูกไฟล์ staged.
package submission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StatusSubmitted = "submitted"
	StatusLate      = "late"
	StatusPending   = "pending"
	StatusOverdue   = "overdue"
)

var (
	ErrTopicNotFound = errors.New("submission topic not found")
	ErrTopicClosed   = errors.New("submission topic is closed")
	ErrNotAssignee   = errors.New("user is not assigned to submission topic")
)

var bangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

type objectRemover interface {
	Trash(context.Context, string) error
}

type Service struct {
	db    *gorm.DB
	store objectRemover
	now   func() time.Time
}

type UpsertInput struct {
	TopicUUID uuid.UUID
	UserUUID  uuid.UUID
	Values    map[string]json.RawMessage
}

type PendingItem struct {
	Topic  model.Topic
	Status string
}

type Detail struct {
	Topic      model.Topic
	Submission *model.Submission
	Files      []model.File
	Status     *string
}

func NewService(db *gorm.DB, store objectRemover) *Service {
	return &Service{db: db, store: store, now: time.Now}
}

// ListPending คืนเฉพาะหัวข้อเปิดที่ผู้ใช้มี assignment แต่ยังไม่มี submission
// โดยเรียงกำหนดส่งใกล้สุดก่อนเพื่อใช้บนหน้า "สิ่งที่ต้องส่ง".
func (s *Service) ListPending(ctx context.Context, userUUID uuid.UUID) ([]PendingItem, error) {
	if userUUID == uuid.Nil {
		return nil, &ValidationError{Fields: []FieldViolation{{Field: "user_uuid", Message: "user UUID is required"}}}
	}
	var topics []model.Topic
	err := s.db.WithContext(ctx).
		Preload("Division").
		Preload("AcademicYear").
		Joins("JOIN topic_assignees ON topic_assignees.topic_uuid = topics.uuid AND topic_assignees.user_uuid = ?", userUUID).
		Where("topics.is_open = ?", true).
		Where("NOT EXISTS (SELECT 1 FROM submissions WHERE submissions.topic_uuid = topics.uuid AND submissions.user_uuid = ?)", userUUID).
		Order("topics.due_date ASC").
		Order("topics.created_at ASC").
		Order("topics.uuid ASC").
		Find(&topics).Error
	if err != nil {
		return nil, fmt.Errorf("list pending submissions: %w", err)
	}

	now := s.currentTime()
	result := make([]PendingItem, 0, len(topics))
	for _, topic := range topics {
		status := StatusPending
		if dateAfterBangkok(now, topic.DueDate) {
			status = StatusOverdue
		}
		result = append(result, PendingItem{Topic: topic, Status: status})
	}
	return result, nil
}

// GetMine คืนแบบฟอร์มและคำตอบของผู้ใช้ ถ้ายังไม่เคยส่ง Submission จะเป็น nil.
func (s *Service) GetMine(ctx context.Context, topicUUID, userUUID uuid.UUID) (Detail, error) {
	if err := validateIdentity(topicUUID, userUUID); err != nil {
		return Detail{}, err
	}
	db := s.db.WithContext(ctx)
	topic, err := loadSubmissionTopic(db, topicUUID, false)
	if err != nil {
		return Detail{}, err
	}
	if err := ensureAssigned(db, topicUUID, userUUID, false); err != nil {
		return Detail{}, err
	}
	return loadDetail(db, topic, userUUID)
}

// Upsert สร้างหรือแก้คำตอบใน transaction เดียวกับการผูกไฟล์ staged.
// การแก้ไขจะเปลี่ยนเฉพาะ values/updated_at และคง submitted_at ครั้งแรกไว้เสมอ.
func (s *Service) Upsert(ctx context.Context, input UpsertInput) (Detail, error) {
	if err := validateIdentity(input.TopicUUID, input.UserUUID); err != nil {
		return Detail{}, err
	}
	if input.Values == nil {
		return Detail{}, &ValidationError{Fields: []FieldViolation{{Field: "values", Message: "values object is required"}}}
	}

	var detail Detail
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		topic, err := loadSubmissionTopic(tx, input.TopicUUID, true)
		if err != nil {
			return err
		}
		if !topic.IsOpen {
			return ErrTopicClosed
		}
		// UPDATE lock ของ assignment ทำให้การส่งครั้งแรกพร้อมกันของคนเดียวกันเรียงคิว
		// ก่อนอ่าน submission จึงไม่ชน unique key แล้วกลายเป็น 500.
		if err := ensureAssigned(tx, input.TopicUUID, input.UserUUID, true); err != nil {
			return err
		}

		existing, err := findSubmission(tx, input.TopicUUID, input.UserUUID, true)
		if err != nil {
			return err
		}
		normalized, references, violations := normalizeSubmissionValues(input.Values, topic.Fields)
		fileViolations, err := validateFileReferences(tx, input, existing, references)
		if err != nil {
			return err
		}
		violations = append(violations, fileViolations...)
		if len(violations) > 0 {
			return &ValidationError{Fields: violations}
		}
		if err := preserveHiddenValues(existing, topic.Fields, normalized); err != nil {
			return err
		}

		encoded, err := json.Marshal(normalized)
		if err != nil {
			return fmt.Errorf("encode submission values: %w", err)
		}
		now := s.currentTime().UTC()
		current := existing
		if current == nil {
			current = &model.Submission{
				TopicUUID:   input.TopicUUID,
				UserUUID:    input.UserUUID,
				Values:      datatypes.JSON(encoded),
				SubmittedAt: now,
				UpdatedAt:   now,
			}
			if err := tx.Create(current).Error; err != nil {
				return fmt.Errorf("create submission: %w", err)
			}
		} else {
			if err := tx.Model(current).Updates(map[string]any{
				"values":     datatypes.JSON(encoded),
				"updated_at": now,
			}).Error; err != nil {
				return fmt.Errorf("update submission: %w", err)
			}
			current.Values = datatypes.JSON(encoded)
			current.UpdatedAt = now
		}

		if err := s.reconcileFiles(ctx, tx, current, topic.Fields, references); err != nil {
			return err
		}
		loaded, err := loadDetail(tx, topic, input.UserUUID)
		if err != nil {
			return err
		}
		detail = loaded
		return nil
	})
	if err != nil {
		return Detail{}, err
	}
	return detail, nil
}

func validateIdentity(topicUUID, userUUID uuid.UUID) error {
	violations := make([]FieldViolation, 0, 2)
	if topicUUID == uuid.Nil {
		violations = append(violations, FieldViolation{Field: "topic_uuid", Message: "topic UUID is required"})
	}
	if userUUID == uuid.Nil {
		violations = append(violations, FieldViolation{Field: "user_uuid", Message: "user UUID is required"})
	}
	if len(violations) > 0 {
		return &ValidationError{Fields: violations}
	}
	return nil
}

func loadSubmissionTopic(db *gorm.DB, topicUUID uuid.UUID, lock bool) (model.Topic, error) {
	var topic model.Topic
	query := db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "SHARE"})
	}
	result := query.
		Preload("Division").
		Preload("AcademicYear").
		Preload("Fields", func(fields *gorm.DB) *gorm.DB {
			return fields.Where("hidden_at IS NULL").Order("position, uuid")
		}).
		Preload("Assignees").
		Where("uuid = ?", topicUUID).
		Limit(1).
		Find(&topic)
	if result.Error != nil {
		return model.Topic{}, fmt.Errorf("load submission topic: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Topic{}, ErrTopicNotFound
	}
	return topic, nil
}

func ensureAssigned(db *gorm.DB, topicUUID, userUUID uuid.UUID, lock bool) error {
	var assignee model.TopicAssignee
	query := db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	result := query.Where("topic_uuid = ? AND user_uuid = ?", topicUUID, userUUID).Limit(1).Find(&assignee)
	if result.Error != nil {
		return fmt.Errorf("find submission assignee: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotAssignee
	}
	return nil
}

func findSubmission(db *gorm.DB, topicUUID, userUUID uuid.UUID, lock bool) (*model.Submission, error) {
	var submission model.Submission
	query := db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	result := query.Where("topic_uuid = ? AND user_uuid = ?", topicUUID, userUUID).Limit(1).Find(&submission)
	if result.Error != nil {
		return nil, fmt.Errorf("find my submission: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &submission, nil
}

func loadDetail(db *gorm.DB, topic model.Topic, userUUID uuid.UUID) (Detail, error) {
	submission, err := findSubmission(db, topic.UUID, userUUID, false)
	if err != nil {
		return Detail{}, err
	}
	if submission == nil {
		return Detail{Topic: topic}, nil
	}
	files := make([]model.File, 0)
	if err := db.Where("submission_uuid = ? AND status = ?", submission.UUID, model.FileAttached).
		Order("field_uuid, position, created_at, uuid").Find(&files).Error; err != nil {
		return Detail{}, fmt.Errorf("list submission files: %w", err)
	}
	status := submissionStatus(submission.SubmittedAt, topic.DueDate)
	return Detail{Topic: topic, Submission: submission, Files: files, Status: &status}, nil
}

func validateFileReferences(db *gorm.DB, input UpsertInput, existing *model.Submission, references []fileReference) ([]FieldViolation, error) {
	if len(references) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(references))
	for _, reference := range references {
		ids = append(ids, reference.UUID)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left].String() < ids[right].String() })

	var files []model.File
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("uuid IN ?", ids).Order("uuid").Find(&files).Error; err != nil {
		return nil, fmt.Errorf("lock submitted files: %w", err)
	}
	byUUID := make(map[uuid.UUID]model.File, len(files))
	for _, file := range files {
		byUUID[file.UUID] = file
	}

	violations := make([]FieldViolation, 0)
	seen := make(map[uuid.UUID]string, len(references))
	for _, reference := range references {
		if firstPath, duplicate := seen[reference.UUID]; duplicate {
			violations = append(violations, fieldViolation(reference.FieldUUID, reference.Path, "file is already used at "+firstPath))
			continue
		}
		seen[reference.UUID] = reference.Path
		file, exists := byUUID[reference.UUID]
		if !exists || file.UploadedByUUID != input.UserUUID || file.TopicUUID != input.TopicUUID || file.FieldUUID != reference.FieldUUID {
			violations = append(violations, fieldViolation(reference.FieldUUID, reference.Path, "file is unavailable for this field"))
			continue
		}
		validStaged := file.Status == model.FileStaged && file.SubmissionUUID == nil
		validAttached := existing != nil && file.Status == model.FileAttached && file.SubmissionUUID != nil && *file.SubmissionUUID == existing.UUID
		if !validStaged && !validAttached {
			violations = append(violations, fieldViolation(reference.FieldUUID, reference.Path, "file is unavailable for this submission"))
		}
	}
	return violations, nil
}

func preserveHiddenValues(existing *model.Submission, activeFields []model.TopicField, normalized map[string]json.RawMessage) error {
	if existing == nil || len(existing.Values) == 0 {
		return nil
	}
	var previous map[string]json.RawMessage
	if err := json.Unmarshal(existing.Values, &previous); err != nil {
		return fmt.Errorf("decode existing submission values: %w", err)
	}
	active := make(map[string]struct{}, len(activeFields))
	for _, field := range activeFields {
		active[field.UUID.String()] = struct{}{}
	}
	for key, value := range previous {
		if _, isActive := active[key]; !isActive {
			normalized[key] = value
		}
	}
	return nil
}

func (s *Service) reconcileFiles(ctx context.Context, db *gorm.DB, submission *model.Submission, activeFields []model.TopicField, references []fileReference) error {
	currentFiles := make([]model.File, 0)
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("submission_uuid = ? AND status = ?", submission.UUID, model.FileAttached).
		Order("uuid").Find(&currentFiles).Error; err != nil {
		return fmt.Errorf("lock attached submission files: %w", err)
	}
	requested := make(map[uuid.UUID]fileReference, len(references))
	for _, reference := range references {
		requested[reference.UUID] = reference
	}
	active := make(map[uuid.UUID]struct{}, len(activeFields))
	for _, field := range activeFields {
		active[field.UUID] = struct{}{}
	}

	// เก็บไฟล์ของช่องที่ถูกซ่อนไว้เพื่อรักษาประวัติ แต่ลบ object ของช่องปัจจุบัน
	// ที่ผู้ใช้เอาออกจากคำตอบฉบับแก้ไขแล้ว.
	for _, file := range currentFiles {
		if _, fieldIsActive := active[file.FieldUUID]; !fieldIsActive {
			continue
		}
		if _, keep := requested[file.UUID]; keep {
			continue
		}
		if s.store == nil {
			return errors.New("submission storage is unavailable")
		}
		if err := s.store.Trash(ctx, file.ObjectKey); err != nil {
			return err
		}
		if err := db.Model(&model.File{}).Where("uuid = ?", file.UUID).
			Update("status", model.FileTrashed).Error; err != nil {
			return fmt.Errorf("mark removed submission file trashed: %w", err)
		}
	}

	for _, reference := range references {
		if err := db.Model(&model.File{}).Where("uuid = ?", reference.UUID).
			Updates(map[string]any{
				"submission_uuid": submission.UUID,
				"status":          model.FileAttached,
				"position":        reference.Position,
			}).Error; err != nil {
			return fmt.Errorf("attach submitted file: %w", err)
		}
	}
	return nil
}

func (s *Service) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func submissionStatus(submittedAt, dueDate time.Time) string {
	if dateAfterBangkok(submittedAt, dueDate) {
		return StatusLate
	}
	return StatusSubmitted
}

func dateAfterBangkok(value, dueDate time.Time) bool {
	local := value.In(bangkokLocation)
	valueDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, bangkokLocation)
	due := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, bangkokLocation)
	return valueDate.After(due)
}
