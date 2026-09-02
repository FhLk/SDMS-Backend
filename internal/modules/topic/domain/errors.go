package domain

import "errors"

var (
	ErrTopicNotFound  = errors.New("topic not found")
	ErrTopicNameEmpty = errors.New("topic name is required")
)

var (
	ErrTopicFieldInvalidTopicUID = errors.New("topic field topic uid is required")
	ErrTopicFieldLabelRequired   = errors.New("topic field label is required")
	ErrTopicFieldInvalidType     = errors.New("invalid topic field type")
	ErrTopicFieldInvalidPosition = errors.New("topic field position must be greater than or equal to zero")
)
