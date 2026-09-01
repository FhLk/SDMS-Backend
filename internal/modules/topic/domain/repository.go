package domain

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, topic *Topic) error

	FindAll(ctx context.Context) ([]Topic, error)

	FindByID(ctx context.Context, id uuid.UUID) (*Topic, error)

	Update(ctx context.Context, topic *Topic) error

	Delete(ctx context.Context, id uuid.UUID) error
}
