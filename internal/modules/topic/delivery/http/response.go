package http

import (
	"sdms/internal/modules/topic/domain"
	"time"

	"github.com/google/uuid"
)

type TopicResponse struct {
	UID         uuid.UUID `json:"uid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newTopicResponse(topic domain.Topic) TopicResponse {
	return TopicResponse{
		UID:         topic.UID,
		Name:        topic.Name,
		Description: topic.Description,
		IsActive:    topic.IsActive,
		CreatedAt:   topic.CreatedAt,
		UpdatedAt:   topic.UpdatedAt,
	}
}

type TopicFieldResponse struct {
	UID       uuid.UUID `json:"uid"`
	TopicUID  uuid.UUID `json:"topic_uid"`
	Label     string    `json:"label"`
	Type      string    `json:"type"`
	Required  bool      `json:"required"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newTopicFieldResponse(field *domain.TopicField) TopicFieldResponse {
	return TopicFieldResponse{
		UID:       field.UID,
		TopicUID:  field.TopicUID,
		Label:     field.Label,
		Type:      string(field.Type),
		Required:  field.Required,
		Position:  field.Position,
		CreatedAt: field.CreatedAt,
		UpdatedAt: field.UpdatedAt,
	}
}
