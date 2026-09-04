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

type TopicDetailResponse struct {
	TopicResponse
	Fields []TopicFieldResponse `json:"fields"`
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

func newTopicDetailResponse(
	topic domain.Topic,
	fields []domain.TopicField,
) TopicDetailResponse {
	return TopicDetailResponse{
		TopicResponse: newTopicResponse(topic),
		Fields:        newTopicFieldListResponse(fields),
	}
}

func newTopicFieldListResponse(
	fields []domain.TopicField,
) []TopicFieldResponse {
	response := make(
		[]TopicFieldResponse,
		0,
		len(fields),
	)

	for i := range fields {
		response = append(
			response,
			newTopicFieldResponse(&fields[i]),
		)
	}

	return response
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
