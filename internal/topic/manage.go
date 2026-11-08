package topic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTopicNotFound         = errors.New("topic not found")
	ErrFieldTypeLocked       = errors.New("topic field type cannot be changed after a submission")
	ErrTopicHasSubmissions   = errors.New("topic with submissions cannot be deleted")
	ErrAssigneeHasSubmission = errors.New("topic assignee with a submission cannot be removed")
)

// UpdateFieldInput ใช้ UUID แยกช่องเดิมออกจากช่องใหม่: UUID ว่างหมายถึงเพิ่มช่องใหม่.
type UpdateFieldInput struct {
	UUID     uuid.UUID
	Position int16
	Type     string
	Label    string
	Required *bool
	Options  []string
}

// UpdateInput ใช้ pointer เพื่อแยก field ที่ไม่ได้ส่งออกจากค่าศูนย์ที่ผู้ใช้ตั้งใจส่ง.
// หากส่ง Fields หรือ AssigneeUUIDs มา รายการนั้นถือเป็นแบบฟอร์มหรือรายชื่อผู้ต้องส่งฉบับเต็มหลังแก้ไข.
type UpdateInput struct {
	Title         *string
	DueDate       *string
	IsSecret      *bool
	Fields        *[]UpdateFieldInput
	AssigneeUUIDs *[]uuid.UUID
}

// Get โหลดหัวข้อพร้อม relation ที่จำเป็นทั้งสำหรับตรวจสิทธิ์และสร้าง response.
func (s *Service) Get(ctx context.Context, topicUUID uuid.UUID) (model.Topic, error) {
	if topicUUID == uuid.Nil {
		return model.Topic{}, validationError("topic_uuid", "topic UUID is required")
	}
	return loadTopic(s.db.WithContext(ctx), topicUUID)
}

// Update แก้ข้อมูลหัวข้อและ synchronize ช่องฟอร์มทั้งหมดใน transaction เดียว.
// ช่องที่หายจาก request จะถูกลบจริงเมื่อยังไม่มีคำตอบ หรือซ่อนเมื่อเคยมีคำตอบแล้ว.
func (s *Service) Update(ctx context.Context, topicUUID uuid.UUID, input UpdateInput) (model.Topic, error) {
	if topicUUID == uuid.Nil {
		return model.Topic{}, validationError("topic_uuid", "topic UUID is required")
	}
	input = normalizeUpdateInput(input)
	dueDate, err := validateUpdateInput(input)
	if err != nil {
		return model.Topic{}, err
	}
	if input.Fields != nil {
		// validate ด้วยลำดับ request ก่อน เพื่อให้ error path ตรง แล้วจึงเรียงสำหรับบันทึก/response.
		sortUpdateFields(*input.Fields)
	}

	var result model.Topic
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockTopic(tx, topicUUID)
		if err != nil {
			return err
		}

		updates := map[string]any{"updated_at": time.Now().UTC()}
		if input.Title != nil {
			updates["title"] = *input.Title
		}
		if dueDate != nil {
			updates["due_date"] = *dueDate
		}
		if input.IsSecret != nil {
			updates["is_secret"] = *input.IsSecret
		}
		if input.Fields != nil {
			if err := synchronizeTopicFields(tx, topicUUID, *input.Fields); err != nil {
				return err
			}
		}
		if input.AssigneeUUIDs != nil {
			if err := synchronizeTopicAssignees(tx, current, *input.AssigneeUUIDs); err != nil {
				return err
			}
		}

		if err := tx.Model(&model.Topic{}).
			Where("uuid = ?", topicUUID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update topic: %w", err)
		}
		loaded, err := loadTopic(tx, topicUUID)
		if err != nil {
			return err
		}
		result = loaded
		return nil
	})
	if err != nil {
		return model.Topic{}, err
	}
	return result, nil
}

// SetOpen ปิดหรือเปิดรับหัวข้อแบบ idempotent และเก็บเวลาปิดล่าสุดไว้แม้เปิดรับใหม่.
func (s *Service) SetOpen(ctx context.Context, topicUUID uuid.UUID, open bool) (model.Topic, error) {
	if topicUUID == uuid.Nil {
		return model.Topic{}, validationError("topic_uuid", "topic UUID is required")
	}

	var result model.Topic
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockTopic(tx, topicUUID)
		if err != nil {
			return err
		}
		// ถ้าสถานะตรงอยู่แล้วให้คืนข้อมูลเดิมโดยไม่ขยับ updated_at หรือ closed_at.
		if current.IsOpen == open {
			loaded, err := loadTopic(tx, topicUUID)
			if err != nil {
				return err
			}
			result = loaded
			return nil
		}
		now := time.Now().UTC()
		updates := map[string]any{"is_open": open, "updated_at": now}
		if !open {
			updates["closed_at"] = now
		}
		if err := tx.Model(&model.Topic{}).
			Where("uuid = ?", topicUUID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("change topic open state: %w", err)
		}
		loaded, err := loadTopic(tx, topicUUID)
		if err != nil {
			return err
		}
		result = loaded
		return nil
	})
	if err != nil {
		return model.Topic{}, err
	}
	return result, nil
}

// Delete ลบหัวข้อได้เฉพาะตอนที่ยังไม่มีคำตอบ เพื่อไม่ทำข้อมูลการส่งสูญหาย.
func (s *Service) Delete(ctx context.Context, topicUUID uuid.UUID) error {
	if topicUUID == uuid.Nil {
		return validationError("topic_uuid", "topic UUID is required")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		topic, err := lockTopic(tx, topicUUID)
		if err != nil {
			return err
		}
		hasSubmissions, err := topicHasSubmissions(tx, topicUUID)
		if err != nil {
			return err
		}
		if hasSubmissions {
			return ErrTopicHasSubmissions
		}
		if err := tx.Delete(&topic).Error; err != nil {
			return fmt.Errorf("delete topic: %w", err)
		}
		return nil
	})
}

func normalizeUpdateInput(input UpdateInput) UpdateInput {
	if input.Title != nil {
		value := strings.TrimSpace(*input.Title)
		input.Title = &value
	}
	if input.DueDate != nil {
		value := strings.TrimSpace(*input.DueDate)
		input.DueDate = &value
	}
	if input.Fields == nil {
		return input
	}

	fields := append([]UpdateFieldInput(nil), (*input.Fields)...)
	for index := range fields {
		field := &fields[index]
		field.Type = strings.ToLower(strings.TrimSpace(field.Type))
		field.Label = strings.TrimSpace(field.Label)
		if field.Position == 0 {
			field.Position = int16(index + 1)
		}
		for optionIndex := range field.Options {
			field.Options[optionIndex] = strings.TrimSpace(field.Options[optionIndex])
		}
		if field.Options == nil {
			field.Options = []string{}
		}
	}
	input.Fields = &fields
	return input
}

func validateUpdateInput(input UpdateInput) (*time.Time, error) {
	if input.Title == nil && input.DueDate == nil && input.IsSecret == nil && input.Fields == nil && input.AssigneeUUIDs == nil {
		return nil, validationError("body", "at least one property is required")
	}
	if input.Title != nil {
		if *input.Title == "" {
			return nil, validationError("title", "title is required")
		}
		if utf8.RuneCountInString(*input.Title) > 200 {
			return nil, validationError("title", "title must not exceed 200 characters")
		}
	}

	var dueDate *time.Time
	if input.DueDate != nil {
		parsed, err := time.Parse(dateLayout, *input.DueDate)
		if err != nil || parsed.Format(dateLayout) != *input.DueDate {
			return nil, validationError("due_date", "due date must use YYYY-MM-DD and be a valid date")
		}
		dueDate = &parsed
	}
	if input.Fields != nil {
		fields := make([]FieldInput, 0, len(*input.Fields))
		seenUUIDs := make(map[uuid.UUID]struct{}, len(*input.Fields))
		for index, field := range *input.Fields {
			if field.UUID != uuid.Nil {
				if _, duplicate := seenUUIDs[field.UUID]; duplicate {
					return nil, validationError(fmt.Sprintf("fields[%d].uuid", index), "field UUID must not be duplicated")
				}
				seenUUIDs[field.UUID] = struct{}{}
			}
			fields = append(fields, FieldInput{
				Position: field.Position,
				Type:     field.Type,
				Label:    field.Label,
				Options:  field.Options,
			})
		}
		if err := validateTopicFields(fields); err != nil {
			return nil, err
		}
	}
	if err := validateAssigneeUUIDs(input.AssigneeUUIDs); err != nil {
		return nil, err
	}
	return dueDate, nil
}

func synchronizeTopicAssignees(db *gorm.DB, topic model.Topic, requested []uuid.UUID) error {
	current := make([]model.TopicAssignee, 0)
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("topic_uuid = ?", topic.UUID).
		Order("user_uuid").
		Find(&current).Error; err != nil {
		return fmt.Errorf("lock topic assignees: %w", err)
	}

	currentByUser := make(map[uuid.UUID]model.TopicAssignee, len(current))
	for _, assignee := range current {
		currentByUser[assignee.UserUUID] = assignee
	}
	desired := make(map[uuid.UUID]struct{}, len(requested))
	newUserUUIDs := make([]uuid.UUID, 0, len(requested))
	for _, userUUID := range requested {
		desired[userUUID] = struct{}{}
		if _, exists := currentByUser[userUUID]; !exists {
			newUserUUIDs = append(newUserUUIDs, userUUID)
		}
	}

	if len(newUserUUIDs) > 0 {
		eligibleUUIDs, err := listEligibleTopicAssigneeUUIDs(db, &newUserUUIDs)
		if err != nil {
			return err
		}
		eligible := make(map[uuid.UUID]struct{}, len(eligibleUUIDs))
		for _, userUUID := range eligibleUUIDs {
			eligible[userUUID] = struct{}{}
		}
		for index, userUUID := range requested {
			if _, alreadyAssigned := currentByUser[userUUID]; alreadyAssigned {
				continue
			}
			if _, canAssign := eligible[userUUID]; !canAssign {
				return validationError(
					fmt.Sprintf("assignee_uuids[%d]", index),
					"assignee must be an active account",
				)
			}
		}
	}

	removedUserUUIDs := make([]uuid.UUID, 0, len(current))
	for _, assignee := range current {
		if _, keep := desired[assignee.UserUUID]; !keep {
			removedUserUUIDs = append(removedUserUUIDs, assignee.UserUUID)
		}
	}
	if len(removedUserUUIDs) > 0 {
		var submittedUserUUIDs []uuid.UUID
		if err := db.Model(&model.Submission{}).
			Where("topic_uuid = ? AND user_uuid IN ?", topic.UUID, removedUserUUIDs).
			Order("user_uuid").
			Pluck("user_uuid", &submittedUserUUIDs).Error; err != nil {
			return fmt.Errorf("check removed topic assignees: %w", err)
		}
		if len(submittedUserUUIDs) > 0 {
			return ErrAssigneeHasSubmission
		}
		if err := db.Where("topic_uuid = ? AND user_uuid IN ?", topic.UUID, removedUserUUIDs).
			Delete(&model.TopicAssignee{}).Error; err != nil {
			return fmt.Errorf("remove topic assignees: %w", err)
		}
	}

	if len(newUserUUIDs) > 0 {
		assignees := make([]model.TopicAssignee, 0, len(newUserUUIDs))
		for _, userUUID := range newUserUUIDs {
			assignees = append(assignees, model.TopicAssignee{TopicUUID: topic.UUID, UserUUID: userUUID})
		}
		sort.Slice(assignees, func(left, right int) bool {
			return assignees[left].UserUUID.String() < assignees[right].UserUUID.String()
		})
		if err := db.Create(&assignees).Error; err != nil {
			return fmt.Errorf("add topic assignees: %w", err)
		}
	}
	return nil
}

func synchronizeTopicFields(db *gorm.DB, topicUUID uuid.UUID, inputs []UpdateFieldInput) error {
	var currentFields []model.TopicField
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("topic_uuid = ? AND hidden_at IS NULL", topicUUID).
		Order("position").
		Find(&currentFields).Error; err != nil {
		return fmt.Errorf("lock topic fields: %w", err)
	}

	hasSubmissions, err := topicHasSubmissions(db, topicUUID)
	if err != nil {
		return err
	}
	answeredFields, err := answeredTopicFields(db, topicUUID)
	if err != nil {
		return err
	}

	currentByUUID := make(map[uuid.UUID]model.TopicField, len(currentFields))
	for _, field := range currentFields {
		currentByUUID[field.UUID] = field
	}
	retained := make(map[uuid.UUID]struct{}, len(inputs))
	newFields := make([]FieldInput, 0, len(inputs))

	for index, input := range inputs {
		if input.UUID == uuid.Nil {
			required := true
			if input.Required != nil {
				required = *input.Required
			}
			newFields = append(newFields, FieldInput{
				Position: input.Position,
				Type:     input.Type,
				Label:    input.Label,
				Required: required,
				Options:  input.Options,
			})
			continue
		}

		current, exists := currentByUUID[input.UUID]
		if !exists {
			return validationError(fmt.Sprintf("fields[%d].uuid", index), "field does not belong to this topic or is hidden")
		}
		if hasSubmissions && current.Type != input.Type {
			return ErrFieldTypeLocked
		}
		options, err := json.Marshal(input.Options)
		if err != nil {
			return fmt.Errorf("encode topic field options: %w", err)
		}
		required := current.Required
		if input.Required != nil {
			required = *input.Required
		}
		if err := db.Model(&model.TopicField{}).
			Where("uuid = ?", current.UUID).
			Updates(map[string]any{
				"position": input.Position,
				"type":     input.Type,
				"label":    input.Label,
				"required": required,
				"options":  datatypes.JSON(options),
			}).Error; err != nil {
			return fmt.Errorf("update topic field: %w", err)
		}
		retained[current.UUID] = struct{}{}
	}

	// ช่องเดิมที่ไม่ได้ส่งกลับมาคือช่องที่ผู้ใช้ลบ หากเคยมีคำตอบให้ซ่อน
	// เพื่อรักษา key ใน submissions.values; ช่องที่ไม่เคยมีคำตอบลบแถวจริงได้.
	hiddenAt := time.Now().UTC()
	for _, current := range currentFields {
		if _, keep := retained[current.UUID]; keep {
			continue
		}
		if _, answered := answeredFields[current.UUID.String()]; answered {
			if err := db.Model(&model.TopicField{}).
				Where("uuid = ?", current.UUID).
				Update("hidden_at", hiddenAt).Error; err != nil {
				return fmt.Errorf("hide answered topic field: %w", err)
			}
			continue
		}
		if err := db.Delete(&current).Error; err != nil {
			return fmt.Errorf("delete unused topic field: %w", err)
		}
	}

	if len(newFields) > 0 {
		if _, err := createTopicFields(db, topicUUID, newFields); err != nil {
			return err
		}
	}
	return nil
}

func topicHasSubmissions(db *gorm.DB, topicUUID uuid.UUID) (bool, error) {
	var exists bool
	if err := db.Raw(
		"SELECT EXISTS (SELECT 1 FROM submissions WHERE topic_uuid = ?)",
		topicUUID,
	).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check topic submissions: %w", err)
	}
	return exists, nil
}

func answeredTopicFields(db *gorm.DB, topicUUID uuid.UUID) (map[string]struct{}, error) {
	// ให้ PostgreSQL แตกเฉพาะ key ของ JSONB เพื่อลดการส่งข้อความคำตอบทั้งหมดกลับเข้า Go.
	var fieldUUIDs []string
	if err := db.Raw(`
		SELECT DISTINCT answer.key
		FROM submissions
		CROSS JOIN LATERAL jsonb_object_keys(submissions.values) AS answer(key)
		WHERE submissions.topic_uuid = ?
	`, topicUUID).Scan(&fieldUUIDs).Error; err != nil {
		return nil, fmt.Errorf("list topic answers: %w", err)
	}
	answered := make(map[string]struct{}, len(fieldUUIDs))
	for _, fieldUUID := range fieldUUIDs {
		answered[fieldUUID] = struct{}{}
	}
	return answered, nil
}

func lockTopic(db *gorm.DB, topicUUID uuid.UUID) (model.Topic, error) {
	var topic model.Topic
	query := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("uuid = ?", topicUUID).
		Limit(1).
		Find(&topic)
	if query.Error != nil {
		return model.Topic{}, fmt.Errorf("lock topic: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return model.Topic{}, ErrTopicNotFound
	}
	return topic, nil
}

func loadTopic(db *gorm.DB, topicUUID uuid.UUID) (model.Topic, error) {
	var topic model.Topic
	query := db.
		Preload("Division").
		Preload("AcademicYear").
		Preload("Fields", func(query *gorm.DB) *gorm.DB {
			return query.Where("hidden_at IS NULL").Order("position")
		}).
		Preload("Assignees", func(query *gorm.DB) *gorm.DB {
			return query.Order("user_uuid")
		}).
		Where("uuid = ?", topicUUID).
		Limit(1).
		Find(&topic)
	if query.Error != nil {
		return model.Topic{}, fmt.Errorf("load topic: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return model.Topic{}, ErrTopicNotFound
	}
	return topic, nil
}

func sortUpdateFields(fields []UpdateFieldInput) {
	sort.SliceStable(fields, func(left, right int) bool {
		return fields[left].Position < fields[right].Position
	})
}
