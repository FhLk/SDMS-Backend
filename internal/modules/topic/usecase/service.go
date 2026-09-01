package usecase

import (
	"context"
	"sdms/internal/modules/topic/domain"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repository domain.Repository
}

func NewService(repository domain.Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Create(ctx context.Context, name string, description string) (*domain.Topic, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, domain.ErrTopicNameEmpty
	}

	topic := &domain.Topic{
		UID:         uuid.New(),
		Name:        name,
		Description: description,
		IsActive:    true,
	}

	if err := s.repository.Create(ctx, topic); err != nil {
		return nil, err
	}

	return topic, nil
}

func (s *Service) FindAll(ctx context.Context) ([]domain.Topic, error) {
	return s.repository.FindAll(ctx)
}

func (s *Service) FindByID(ctx context.Context, topicID uuid.UUID) (*domain.Topic, error) {
	return s.repository.FindByID(ctx, topicID)
}

func (s *Service) Update(ctx context.Context, topicID uuid.UUID, name string, description string, isActive bool) (*domain.Topic, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, domain.ErrTopicNameEmpty
	}

	topic, err := s.repository.FindByID(ctx, topicID)
	if err != nil {
		return nil, err
	}

	topic.Name = name
	topic.Description = description
	topic.IsActive = isActive
	if err := s.repository.Update(ctx, topic); err != nil {
		return nil, err
	}

	return topic, nil
}

func (s *Service) Delete(ctx context.Context, topicID uuid.UUID) error {
	_, err := s.repository.FindByID(ctx, topicID)
	if err != nil {
		return err
	}

	return s.repository.Delete(ctx, topicID)
}
