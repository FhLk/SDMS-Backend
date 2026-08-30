package domain

import (
	"time"

	uuid "github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Model struct {
	UID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (m *Model) BeforeCreate(tx *gorm.DB) error {
	if m.UID == uuid.Nil {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}

		m.UID = id
	}

	return nil
}
