package http

import (
	"sdms/internal/modules/topic/domain"
	"time"

	"github.com/google/uuid"
)

type CreateTopicRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateTopicRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

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
