package domain

import "errors"

var (
	ErrTopicNotFound  = errors.New("topic not found")
	ErrTopicNameEmpty = errors.New("topic name is required")
)
