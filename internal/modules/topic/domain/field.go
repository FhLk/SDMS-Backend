package domain

import (
	"time"

	"github.com/google/uuid"
)

type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeTextarea FieldType = "textarea"
	FieldTypeNumber   FieldType = "number"
	FieldTypeDate     FieldType = "date"
	FieldTypeSelect   FieldType = "select"
	FieldTypeFile     FieldType = "file"
)

type TopicField struct {
	UID       uuid.UUID
	TopicUID  uuid.UUID
	Label     string
	Type      FieldType
	Required  bool
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t FieldType) IsValid() bool {
	switch t {
	case FieldTypeText,
		FieldTypeTextarea,
		FieldTypeNumber,
		FieldTypeDate,
		FieldTypeSelect,
		FieldTypeFile:
		return true
	default:
		return false
	}
}

func NewTopicField(
	topicUID uuid.UUID,
	label string,
	fieldType FieldType,
	required bool,
	position int,
) (*TopicField, error) {
	if topicUID == uuid.Nil {
		return nil, ErrTopicFieldInvalidTopicUID
	}

	if label == "" {
		return nil, ErrTopicFieldLabelRequired
	}

	if !fieldType.IsValid() {
		return nil, ErrTopicFieldInvalidType
	}

	if position < 0 {
		return nil, ErrTopicFieldInvalidPosition
	}

	return &TopicField{
		UID:      uuid.New(),
		TopicUID: topicUID,
		Label:    label,
		Type:     fieldType,
		Required: required,
		Position: position,
	}, nil
}
