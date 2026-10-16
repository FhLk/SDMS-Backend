package submission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/authorization"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const StatusNotSubmitted = "not_submitted"

var ErrForbidden = errors.New("submission table access is forbidden")

type TopicTable struct {
	Topic model.Topic
	Rows  []TopicTableRow
}

type TopicTableRow struct {
	User           model.User
	SubmissionUUID *uuid.UUID
	Values         datatypes.JSON
	Files          []model.File
	Status         string
	SubmittedAt    *time.Time
	UpdatedAt      *time.Time
}

type topicTableDatabaseRow struct {
	UserUUID       uuid.UUID
	Username       string
	Prefix         string
	FirstName      string
	LastName       string
	Position       string
	SubmissionUUID *uuid.UUID
	Values         datatypes.JSON
	SubmittedAt    *time.Time
	UpdatedAt      *time.Time
}

// ListTopicTable คืนหนึ่งแถวต่อ snapshot ผู้ต้องส่ง และจำกัด scope ของหัวข้อลับ
// ใน transaction เดียวกับ query เพื่อไม่ให้การเปลี่ยนสิทธิ์ระหว่าง request ทำข้อมูลรั่ว.
func (s *Service) ListTopicTable(ctx context.Context, topicUUID uuid.UUID, profile auth.Profile, statusFilter string) (TopicTable, error) {
	if topicUUID == uuid.Nil {
		return TopicTable{}, &ValidationError{Fields: []FieldViolation{{Field: "topic_uuid", Message: "topic UUID is required"}}}
	}
	statusFilter = strings.ToLower(strings.TrimSpace(statusFilter))
	if statusFilter != "" && statusFilter != "done" && statusFilter != "pending" {
		return TopicTable{}, &ValidationError{Fields: []FieldViolation{{Field: "status", Message: "status must be done or pending"}}}
	}
	if profile.User.UUID == uuid.Nil {
		return TopicTable{}, ErrForbidden
	}

	var result TopicTable
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		topic, err := loadSubmissionTopic(tx, topicUUID, true)
		if err != nil {
			return err
		}
		rows, err := queryTopicTableRows(tx, topic, profile, statusFilter, s.currentTime())
		if err != nil {
			return err
		}
		result = TopicTable{Topic: topic, Rows: rows}
		return nil
	})
	if err != nil {
		return TopicTable{}, err
	}
	return result, nil
}

// ListPendingNames คืนผู้ใช้ใน snapshot ที่ยังไม่มี submission และอนุญาตเฉพาะ
// ผู้ที่แก้หัวข้อนั้นได้ตาม policy กลางของระบบ.
func (s *Service) ListPendingNames(ctx context.Context, topicUUID uuid.UUID, profile auth.Profile) ([]model.User, error) {
	if topicUUID == uuid.Nil {
		return nil, &ValidationError{Fields: []FieldViolation{{Field: "topic_uuid", Message: "topic UUID is required"}}}
	}
	if profile.User.UUID == uuid.Nil {
		return nil, ErrForbidden
	}

	users := make([]model.User, 0)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		topic, err := loadSubmissionTopic(tx, topicUUID, true)
		if err != nil {
			return err
		}
		if !authorization.CanEditTopic(profile, topic) {
			return ErrForbidden
		}
		if err := tx.Table("users").
			Select("users.*").
			Joins("JOIN topic_assignees ON topic_assignees.user_uuid = users.uuid AND topic_assignees.topic_uuid = ?", topic.UUID).
			Joins("LEFT JOIN submissions ON submissions.topic_uuid = topic_assignees.topic_uuid AND submissions.user_uuid = topic_assignees.user_uuid").
			Where("submissions.uuid IS NULL").
			Order("users.first_name, users.last_name, users.username, users.uuid").
			Find(&users).Error; err != nil {
			return fmt.Errorf("list pending submission names: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

func queryTopicTableRows(db *gorm.DB, topic model.Topic, profile auth.Profile, statusFilter string, now time.Time) ([]TopicTableRow, error) {
	databaseRows := make([]topicTableDatabaseRow, 0)
	query := db.Table("topic_assignees").
		Select(`
			users.uuid AS user_uuid,
			users.username,
			users.prefix,
			users.first_name,
			users.last_name,
			users.position,
			submissions.uuid AS submission_uuid,
			submissions.values,
			submissions.submitted_at,
			submissions.updated_at`).
		Joins("JOIN users ON users.uuid = topic_assignees.user_uuid").
		Joins("LEFT JOIN submissions ON submissions.topic_uuid = topic_assignees.topic_uuid AND submissions.user_uuid = topic_assignees.user_uuid").
		Where("topic_assignees.topic_uuid = ?", topic.UUID)
	if topic.IsSecret && !authorization.CanViewAllTopicSubmissions(profile, topic) {
		query = query.Where("topic_assignees.user_uuid = ?", profile.User.UUID)
	}
	switch statusFilter {
	case "done":
		query = query.Where("submissions.uuid IS NOT NULL")
	case "pending":
		query = query.Where("submissions.uuid IS NULL")
	}
	if err := query.Order("users.first_name, users.last_name, users.username, users.uuid").Scan(&databaseRows).Error; err != nil {
		return nil, fmt.Errorf("list topic submission rows: %w", err)
	}

	filesBySubmission, err := listTableFiles(db, topic.Fields, databaseRows)
	if err != nil {
		return nil, err
	}
	rows := make([]TopicTableRow, 0, len(databaseRows))
	for _, row := range databaseRows {
		values, err := tableValues(topic.Fields, row.Values)
		if err != nil {
			return nil, err
		}
		files := make([]model.File, 0)
		if row.SubmissionUUID != nil {
			files = filesBySubmission[*row.SubmissionUUID]
			if files == nil {
				files = make([]model.File, 0)
			}
		}
		rows = append(rows, TopicTableRow{
			User: model.User{
				UUIDModel: model.UUIDModel{UUID: row.UserUUID},
				Username:  row.Username,
				Prefix:    row.Prefix,
				FirstName: row.FirstName,
				LastName:  row.LastName,
				Position:  row.Position,
			},
			SubmissionUUID: row.SubmissionUUID,
			Values:         values,
			Files:          files,
			Status:         tableRowStatus(row.SubmittedAt, topic, now),
			SubmittedAt:    row.SubmittedAt,
			UpdatedAt:      row.UpdatedAt,
		})
	}
	return rows, nil
}

func listTableFiles(db *gorm.DB, fields []model.TopicField, rows []topicTableDatabaseRow) (map[uuid.UUID][]model.File, error) {
	submissionUUIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		if row.SubmissionUUID != nil {
			submissionUUIDs = append(submissionUUIDs, *row.SubmissionUUID)
		}
	}
	result := make(map[uuid.UUID][]model.File, len(submissionUUIDs))
	if len(submissionUUIDs) == 0 || len(fields) == 0 {
		return result, nil
	}
	fieldUUIDs := make([]uuid.UUID, 0, len(fields))
	for _, field := range fields {
		fieldUUIDs = append(fieldUUIDs, field.UUID)
	}
	var files []model.File
	if err := db.Where("submission_uuid IN ? AND field_uuid IN ? AND status = ?", submissionUUIDs, fieldUUIDs, model.FileAttached).
		Order("submission_uuid, field_uuid, position, created_at, uuid").Find(&files).Error; err != nil {
		return nil, fmt.Errorf("list topic table files: %w", err)
	}
	for _, file := range files {
		if file.SubmissionUUID != nil {
			result[*file.SubmissionUUID] = append(result[*file.SubmissionUUID], file)
		}
	}
	return result, nil
}

func tableValues(fields []model.TopicField, stored datatypes.JSON) (datatypes.JSON, error) {
	previous := make(map[string]json.RawMessage)
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &previous); err != nil {
			return nil, fmt.Errorf("decode table submission values: %w", err)
		}
	}
	values := make(map[string]json.RawMessage, len(fields))
	for _, field := range fields {
		value, exists := previous[field.UUID.String()]
		if !exists {
			value = json.RawMessage("null")
		}
		values[field.UUID.String()] = value
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode table submission values: %w", err)
	}
	return datatypes.JSON(encoded), nil
}

func tableRowStatus(submittedAt *time.Time, topic model.Topic, now time.Time) string {
	if submittedAt != nil {
		return submissionStatus(*submittedAt, topic.DueDate)
	}
	if !topic.IsOpen {
		return StatusNotSubmitted
	}
	if dateAfterBangkok(now, topic.DueDate) {
		return StatusOverdue
	}
	return StatusPending
}
