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
	UID       uuid.UUID              `json:"uid"`
	TopicUID  uuid.UUID              `json:"topic_uid"`
	Label     string                 `json:"label"`
	Type      string                 `json:"type"`
	Required  bool                   `json:"required"`
	IsPreview bool                   `json:"is_preview"`
	Position  int                    `json:"position"`
	Options   []SelectOptionResponse `json:"options"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type SelectOptionResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
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
		IsPreview: field.IsPreview,
		Position:  field.Position,
		Options:   newSelectOptionResponse(field.Options),
		CreatedAt: field.CreatedAt,
		UpdatedAt: field.UpdatedAt,
	}
}

func newSelectOptionResponse(
	options []domain.SelectOption,
) []SelectOptionResponse {
	result := make([]SelectOptionResponse, 0, len(options))

	for _, option := range options {
		result = append(result, SelectOptionResponse{
			Label: option.Label,
			Value: option.Value,
		})
	}

	return result
}
