package domain

import "errors"

var (
	ErrTopicNotFound             = errors.New("topic not found")
	ErrTopicNameEmpty            = errors.New("topic name is required")
	ErrTopicAcademicYearRequired = errors.New("academic year is required")
	ErrTopicAcademicYearLocked   = errors.New("academic year cannot be changed after submissions exist")
	ErrTopicDeleteLocked         = errors.New("topic cannot be deleted after submissions exist")
)

var (
	ErrTopicFieldInvalidTopicUID   = errors.New("topic field topic uid is required")
	ErrTopicFieldLabelRequired     = errors.New("topic field label is required")
	ErrTopicFieldInvalidType       = errors.New("invalid topic field type")
	ErrTopicFieldInvalidPosition   = errors.New("topic field position must be greater than or equal to zero")
	ErrTopicFieldNotFound          = errors.New("topic field not found")
	ErrTopicFieldTypeLocked        = errors.New("topic field type cannot be changed after submissions exist")
	ErrTopicFieldDeleteLocked      = errors.New("topic field cannot be deleted after submissions exist")
	ErrTopicFieldSchemaLocked      = errors.New("validation-affecting field settings cannot be changed after submissions exist")
	ErrTopicRequiredFieldAddLocked = errors.New("a required field cannot be added after submissions exist")
)

var (
	ErrTopicFieldSelectOptionsRequired      = errors.New("select field requires at least one option")
	ErrTopicFieldSelectOptionLabelRequired  = errors.New("select option label is required")
	ErrTopicFieldSelectOptionValueRequired  = errors.New("select option value is required")
	ErrTopicFieldSelectOptionDuplicateValue = errors.New("select option value must be unique")
	ErrTopicFieldOptionsOnlyForSelect       = errors.New("options are only allowed for select fields")
)
