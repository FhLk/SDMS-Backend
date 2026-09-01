package domain

import (
	"context"

	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error

	FindByID(ctx context.Context, id uuid.UUID) (*User, error)

	FindByUsername(ctx context.Context, username string) (*User, error)

	FindByEmployeeCode(ctx context.Context, employeeCode string) (*User, error)

	List(ctx context.Context) ([]User, error)

	Update(ctx context.Context, user *User) error

	Delete(ctx context.Context, id uuid.UUID) error
}
