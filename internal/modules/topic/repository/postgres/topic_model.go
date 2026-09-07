package postgres

import (
	"time"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type TopicModel struct {
	UID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	AcademicYear string    `gorm:"type:varchar(20);not null;default:'UNSPECIFIED';index"`
	Name         string    `gorm:"type:varchar(255);not null;index"`
	Description  string    `gorm:"type:text"`
	IsActive     bool      `gorm:"not null;default:true;index"`
	FormVersion  int       `gorm:"not null;default:1"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (TopicModel) TableName() string { return "topics" }

func toDomain(model TopicModel) domain.Topic {
	return domain.Topic{
		UID:          model.UID,
		AcademicYear: model.AcademicYear,
		Name:         model.Name,
		Description:  model.Description,
		IsActive:     model.IsActive,
		FormVersion:  model.FormVersion,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func toModel(topic domain.Topic) TopicModel {
	return TopicModel{
		UID:          topic.UID,
		AcademicYear: topic.AcademicYear,
		Name:         topic.Name,
		Description:  topic.Description,
		IsActive:     topic.IsActive,
		FormVersion:  topic.FormVersion,
		CreatedAt:    topic.CreatedAt,
		UpdatedAt:    topic.UpdatedAt,
	}
}
