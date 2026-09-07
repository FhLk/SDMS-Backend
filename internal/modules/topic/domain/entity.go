package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Topic struct {
	UID          uuid.UUID
	AcademicYear string
	Name         string
	Description  string
	IsActive     bool
	FormVersion  int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (t *Topic) Validate() error {
	if strings.TrimSpace(t.AcademicYear) == "" {
		return ErrTopicAcademicYearRequired
	}
	if strings.TrimSpace(t.Name) == "" {
		return ErrTopicNameEmpty
	}
	if t.FormVersion <= 0 {
		t.FormVersion = 1
	}
	return nil
}
