package domain

import (
	"time"

	"github.com/google/uuid"
)

type Topic struct {
	UID         uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Name        string
	Description string
	IsActive    bool
}
