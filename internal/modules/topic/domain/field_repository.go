package domain

import (
	"context"

	"github.com/google/uuid"
)

type FieldRepository interface {
	Create(ctx context.Context, field *TopicField) error

	FindAllByTopicID(
		ctx context.Context,
		topicUID uuid.UUID,
	) ([]TopicField, error)

	FindByID(
		ctx context.Context,
		uid uuid.UUID,
	) (*TopicField, error)

	Update(
		ctx context.Context,
		field *TopicField,
	) error

	Delete(
		ctx context.Context,
		uid uuid.UUID,
	) error
}
