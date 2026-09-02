package postgres

import (
	"time"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type TopicModel struct {
	UID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	IsActive    bool      `gorm:"not null;default:true"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (TopicModel) TableName() string {
	return "topics"
}

func toDomain(model TopicModel) domain.Topic {
	return domain.Topic{
		UID:         model.UID,
		Name:        model.Name,
		Description: model.Description,
		IsActive:    model.IsActive,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func toModel(topic domain.Topic) TopicModel {
	return TopicModel{
		UID:         topic.UID,
		Name:        topic.Name,
		Description: topic.Description,
		IsActive:    topic.IsActive,
		CreatedAt:   topic.CreatedAt,
		UpdatedAt:   topic.UpdatedAt,
	}
}
