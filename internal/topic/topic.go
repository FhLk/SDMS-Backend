// Package topic รวม use case ของหัวข้อและแบบฟอร์ม โดยไม่ผูกกับ HTTP framework.
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

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maximumTopicFields  = 100
	maximumFieldOptions = 100
	dateLayout          = "2006-01-02"
)

var (
	ErrCurrentAcademicYearNotSet = errors.New("current academic year is not set")
	ErrCreatorNotFound           = errors.New("topic creator not found")
)

type FieldInput struct {
	Position int16
	Type     string
	Label    string
	Required bool
	Options  []string
}

type CreateInput struct {
	DivisionUUID  uuid.UUID
	CreatedByUUID uuid.UUID
	Title         string
	DueDate       string
	IsSecret      bool
	Fields        []FieldInput
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Create ตรวจและ normalize แบบฟอร์มทั้งหมดก่อนเปิด transaction จากนั้นสร้าง
// หัวข้อ ช่อง และรายชื่อผู้ต้องส่งเป็นชุดเดียว เพื่อไม่ทิ้งข้อมูลครึ่งหนึ่ง.
func (s *Service) Create(ctx context.Context, input CreateInput) (model.Topic, error) {
	input = normalizeCreateInput(input)
	dueDate, err := validateCreateInput(input)
	if err != nil {
		return model.Topic{}, err
	}
	// ตรวจด้วยลำดับเดิมก่อนเพื่อให้ error path ตรงกับ request แล้วค่อยเรียงบันทึก.
	sort.SliceStable(input.Fields, func(left, right int) bool {
		return input.Fields[left].Position < input.Fields[right].Position
	})

	var result model.Topic
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		division, err := findDivision(tx, input.DivisionUUID)
		if err != nil {
			return err
		}
		if err := ensureCreatorExists(tx, input.CreatedByUUID); err != nil {
			return err
		}

		// SHARE lock ทำให้หัวข้อได้ปี/ภาคเรียนชุดเดียวกันตลอด transaction
		// และรอหาก BE-07 กำลังสลับปีปัจจุบันด้วย UPDATE lock.
		academicYear, err := findCurrentAcademicYear(tx)
		if err != nil {
			return err
		}

		topic := model.Topic{
			DivisionUUID:     division.UUID,
			AcademicYearUUID: academicYear.UUID,
			Semester:         academicYear.CurrentSemester,
			Title:            input.Title,
			DueDate:          dueDate,
			IsOpen:           true,
			IsSecret:         input.IsSecret,
			CreatedByUUID:    input.CreatedByUUID,
		}
		if err := tx.Create(&topic).Error; err != nil {
			return fmt.Errorf("create topic: %w", err)
		}

		fields, err := createTopicFields(tx, topic.UUID, input.Fields)
		if err != nil {
			return err
		}
		assignees, err := createTopicAssignees(tx, topic.UUID, division.UUID)
		if err != nil {
			return err
		}

		// เติม relation จากข้อมูลที่อ่าน/สร้างใน transaction เดียวกัน
		// เพื่อให้ handler สร้าง response ได้โดยไม่ต้อง query ซ้ำ.
		topic.Division = division
		topic.AcademicYear = academicYear
		topic.Fields = fields
		topic.Assignees = assignees
		result = topic
		return nil
	})
	if err != nil {
		return model.Topic{}, err
	}
	return result, nil
}

func normalizeCreateInput(input CreateInput) CreateInput {
	input.Title = strings.TrimSpace(input.Title)
	input.DueDate = strings.TrimSpace(input.DueDate)
	for index := range input.Fields {
		field := &input.Fields[index]
		field.Type = strings.ToLower(strings.TrimSpace(field.Type))
		field.Label = strings.TrimSpace(field.Label)
		if field.Position == 0 {
			field.Position = int16(index + 1)
		}
		for optionIndex := range field.Options {
			field.Options[optionIndex] = strings.TrimSpace(field.Options[optionIndex])
		}
		if field.Options == nil {
			// เก็บ non-choice options เป็น [] ไม่ใช่ JSON null เพื่อให้ response คงรูปแบบเดียวกัน.
			field.Options = []string{}
		}
	}
	return input
}

func validateCreateInput(input CreateInput) (time.Time, error) {
	if input.DivisionUUID == uuid.Nil {
		return time.Time{}, validationError("division_uuid", "division UUID is required")
	}
	if input.CreatedByUUID == uuid.Nil {
		return time.Time{}, validationError("created_by_uuid", "creator UUID is required")
	}
	if input.Title == "" {
		return time.Time{}, validationError("title", "title is required")
	}
	if utf8.RuneCountInString(input.Title) > 200 {
		return time.Time{}, validationError("title", "title must not exceed 200 characters")
	}

	dueDate, err := time.Parse(dateLayout, input.DueDate)
	if err != nil || dueDate.Format(dateLayout) != input.DueDate {
		return time.Time{}, validationError("due_date", "due date must use YYYY-MM-DD and be a valid date")
	}
	if err := validateTopicFields(input.Fields); err != nil {
		return time.Time{}, err
	}
	return dueDate, nil
}

func validateTopicFields(fields []FieldInput) error {
	if len(fields) == 0 {
		return validationError("fields", "at least one field is required")
	}
	if len(fields) > maximumTopicFields {
		return validationError("fields", fmt.Sprintf("fields must not exceed %d items", maximumTopicFields))
	}

	positions := make(map[int16]struct{}, len(fields))
	for index, field := range fields {
		path := fmt.Sprintf("fields[%d]", index)
		if field.Position < 1 {
			return validationError(path+".position", "field position must start at 1")
		}
		if _, duplicate := positions[field.Position]; duplicate {
			return validationError(path+".position", "field position must not be duplicated")
		}
		positions[field.Position] = struct{}{}

		if !validFieldType(field.Type) {
			return validationError(path+".type", "field type must be text, para, num, date, choice, or file")
		}
		if field.Label == "" {
			return validationError(path+".label", "field label is required")
		}
		if utf8.RuneCountInString(field.Label) > 120 {
			return validationError(path+".label", "field label must not exceed 120 characters")
		}

		if field.Type != model.FieldChoice {
			if len(field.Options) > 0 {
				return validationError(path+".options", "options are allowed only for choice fields")
			}
			continue
		}
		if err := validateChoiceOptions(path, field.Options); err != nil {
			return err
		}
	}
	return nil
}

func validateChoiceOptions(path string, options []string) error {
	if len(options) < 2 {
		return validationError(path+".options", "choice field requires at least two options")
	}
	if len(options) > maximumFieldOptions {
		return validationError(path+".options", fmt.Sprintf("choice options must not exceed %d items", maximumFieldOptions))
	}
	seen := make(map[string]struct{}, len(options))
	for index, option := range options {
		optionPath := fmt.Sprintf("%s.options[%d]", path, index)
		if option == "" {
			return validationError(optionPath, "choice option is required")
		}
		if utf8.RuneCountInString(option) > 120 {
			return validationError(optionPath, "choice option must not exceed 120 characters")
		}
		key := strings.ToLower(option)
		if _, duplicate := seen[key]; duplicate {
			return validationError(optionPath, "choice option must not be duplicated")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validFieldType(value string) bool {
	switch value {
	case model.FieldText, model.FieldPara, model.FieldNum, model.FieldDate, model.FieldChoice, model.FieldFile:
		return true
	default:
		return false
	}
}

func validationError(field, message string) error {
	return &admin.ValidationError{Field: field, Message: message}
}

func findDivision(db *gorm.DB, divisionUUID uuid.UUID) (model.Division, error) {
	var division model.Division
	query := db.Where("uuid = ?", divisionUUID).Limit(1).Find(&division)
	if query.Error != nil {
		return model.Division{}, fmt.Errorf("find topic division: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return model.Division{}, admin.ErrDivisionNotFound
	}
	return division, nil
}

func ensureCreatorExists(db *gorm.DB, creatorUUID uuid.UUID) error {
	var count int64
	if err := db.Model(&model.User{}).Where("uuid = ?", creatorUUID).Count(&count).Error; err != nil {
		return fmt.Errorf("find topic creator: %w", err)
	}
	if count == 0 {
		return ErrCreatorNotFound
	}
	return nil
}

func findCurrentAcademicYear(db *gorm.DB) (model.AcademicYear, error) {
	var academicYear model.AcademicYear
	query := db.Clauses(clause.Locking{Strength: "SHARE"}).
		Where("is_current = ?", true).
		Limit(1).
		Find(&academicYear)
	if query.Error != nil {
		return model.AcademicYear{}, fmt.Errorf("find current academic year: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return model.AcademicYear{}, ErrCurrentAcademicYearNotSet
	}
	return academicYear, nil
}

func createTopicFields(db *gorm.DB, topicUUID uuid.UUID, inputs []FieldInput) ([]model.TopicField, error) {
	fields := make([]model.TopicField, 0, len(inputs))
	for _, input := range inputs {
		options, err := json.Marshal(input.Options)
		if err != nil {
			return nil, fmt.Errorf("encode topic field options: %w", err)
		}
		fields = append(fields, model.TopicField{
			TopicUUID: topicUUID,
			Position:  input.Position,
			Type:      input.Type,
			Label:     input.Label,
			Required:  input.Required,
			Options:   datatypes.JSON(options),
		})
	}
	if err := db.Create(&fields).Error; err != nil {
		return nil, fmt.Errorf("create topic fields: %w", err)
	}

	// TopicField กำหนด default ของ required เป็น true ไว้ที่ฐานข้อมูล จึงทำให้ GORM
	// มองค่า false เป็น zero value และแทนด้วยค่า default ตอน Create เราจึงอัปเดต
	// ฟิลด์ optional อย่างชัดเจนภายใน transaction เดิม เพื่อเก็บค่าที่ผู้ใช้ส่งมาให้ถูกต้อง.
	optionalFieldUUIDs := make([]uuid.UUID, 0, len(fields))
	for index := range fields {
		if inputs[index].Required {
			continue
		}
		optionalFieldUUIDs = append(optionalFieldUUIDs, fields[index].UUID)
		fields[index].Required = false
	}
	if len(optionalFieldUUIDs) > 0 {
		if err := db.Model(&model.TopicField{}).
			Where("uuid IN ?", optionalFieldUUIDs).
			Update("required", false).Error; err != nil {
			return nil, fmt.Errorf("mark topic fields optional: %w", err)
		}
	}
	return fields, nil
}

func createTopicAssignees(db *gorm.DB, topicUUID, divisionUUID uuid.UUID) ([]model.TopicAssignee, error) {
	var userUUIDs []uuid.UUID
	if err := db.Table("users").
		Select("users.uuid").
		Joins("JOIN user_divisions ON user_divisions.user_uuid = users.uuid").
		Where("user_divisions.division_uuid = ?", divisionUUID).
		Where("users.is_exec = ?", false).
		Where("users.deactivated_at IS NULL").
		Order("users.uuid").
		Scan(&userUUIDs).Error; err != nil {
		return nil, fmt.Errorf("list topic assignees: %w", err)
	}

	assignees := make([]model.TopicAssignee, 0, len(userUUIDs))
	for _, userUUID := range userUUIDs {
		assignees = append(assignees, model.TopicAssignee{TopicUUID: topicUUID, UserUUID: userUUID})
	}
	if len(assignees) == 0 {
		return assignees, nil
	}
	if err := db.Create(&assignees).Error; err != nil {
		return nil, fmt.Errorf("create topic assignees: %w", err)
	}
	return assignees, nil
}
