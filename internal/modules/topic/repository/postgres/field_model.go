package postgres

import (
	"time"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type TopicFieldModel struct {
	UID       uuid.UUID             `gorm:"type:uuid;primaryKey"`
	TopicUID  uuid.UUID             `gorm:"type:uuid;not null;index"`
	Label     string                `gorm:"type:varchar(255);not null"`
	Type      string                `gorm:"type:varchar(50);not null"`
	Required  bool                  `gorm:"not null;default:false"`
	Position  int                   `gorm:"not null"`
	Options   []domain.SelectOption `gorm:"serializer:json;type:jsonb"`
	CreatedAt time.Time             `gorm:"not null"`
	UpdatedAt time.Time             `gorm:"not null"`
	Topic     TopicModel            `gorm:"foreignKey:TopicUID;references:UID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (TopicFieldModel) TableName() string {
	return "topic_fields"
}

func toTopicFieldDomain(model TopicFieldModel) domain.TopicField {
	return domain.TopicField{
		UID:       model.UID,
		TopicUID:  model.TopicUID,
		Label:     model.Label,
		Type:      domain.FieldType(model.Type),
		Required:  model.Required,
		Position:  model.Position,
		Options:   model.Options,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func toTopicFieldModel(field domain.TopicField) TopicFieldModel {
	return TopicFieldModel{
		UID:       field.UID,
		TopicUID:  field.TopicUID,
		Label:     field.Label,
		Type:      string(field.Type),
		Required:  field.Required,
		Position:  field.Position,
		Options:   field.Options,
		CreatedAt: field.CreatedAt,
		UpdatedAt: field.UpdatedAt,
	}
}
