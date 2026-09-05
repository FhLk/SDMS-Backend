package domain

import (
	"strings"
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
	IsPreview bool
	Position  int
	Options   []SelectOption
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SelectOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
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
	return NewTopicFieldWithOptions(
		topicUID,
		label,
		fieldType,
		required,
		position,
		nil,
	)
}

func NewTopicFieldWithOptions(
	topicUID uuid.UUID,
	label string,
	fieldType FieldType,
	required bool,
	position int,
	options []SelectOption,
) (*TopicField, error) {

	if err := validateTopicField(
		topicUID,
		label,
		fieldType,
		position,
	); err != nil {
		return nil, err
	}

	normalizedOptions, err := normalizeSelectOptions(
		fieldType,
		options,
	)
	if err != nil {
		return nil, err
	}

	return &TopicField{
		UID:      uuid.New(),
		TopicUID: topicUID,
		Label:    label,
		Type:     fieldType,
		Required: required,
		Position: position,
		Options:  normalizedOptions,
	}, nil
}

func validateTopicField(
	topicUID uuid.UUID,
	label string,
	fieldType FieldType,
	position int,
) error {
	if topicUID == uuid.Nil {
		return ErrTopicFieldInvalidTopicUID
	}

	if strings.TrimSpace(label) == "" {
		return ErrTopicFieldLabelRequired
	}

	if !fieldType.IsValid() {
		return ErrTopicFieldInvalidType
	}

	if position < 0 {
		return ErrTopicFieldInvalidPosition
	}

	return nil
}

func (f *TopicField) Update(
	label string,
	fieldType FieldType,
	required bool,
	position int,
	options []SelectOption,
) error {
	if err := validateTopicField(
		f.TopicUID,
		label,
		fieldType,
		position,
	); err != nil {
		return err
	}

	normalizedOptions, err := normalizeSelectOptions(
		fieldType,
		options,
	)
	if err != nil {
		return err
	}

	f.Label = label
	f.Type = fieldType
	f.Required = required
	f.Position = position
	f.Options = normalizedOptions

	return nil
}

func normalizeSelectOptions(
	fieldType FieldType,
	options []SelectOption,
) ([]SelectOption, error) {

	if fieldType != FieldTypeSelect {
		if len(options) > 0 {
			return nil, ErrTopicFieldOptionsOnlyForSelect
		}

		return []SelectOption{}, nil
	}

	if len(options) == 0 {
		return nil, ErrTopicFieldSelectOptionsRequired
	}

	normalized := make([]SelectOption, 0, len(options))
	seenValues := make(map[string]struct{})

	for _, option := range options {
		option.Label = strings.TrimSpace(option.Label)
		option.Value = strings.TrimSpace(option.Value)

		if option.Label == "" {
			return nil, ErrTopicFieldSelectOptionLabelRequired
		}

		if option.Value == "" {
			return nil, ErrTopicFieldSelectOptionValueRequired
		}

		if _, exists := seenValues[option.Value]; exists {
			return nil, ErrTopicFieldSelectOptionDuplicateValue
		}

		seenValues[option.Value] = struct{}{}

		normalized = append(normalized, option)
	}

	return normalized, nil
}

func (f TopicField) HasSelectOption(value string) bool {
	if f.Type != FieldTypeSelect {
		return false
	}

	value = strings.TrimSpace(value)

	for _, option := range f.Options {
		if option.Value == value {
			return true
		}
	}

	return false
}
