package usecase

import (
	"context"
	"sdms/internal/modules/topic/domain"
	"strings"

	"github.com/google/uuid"
)

type TopicService struct {
	topicRepo domain.TopicRepository
	fieldRepo domain.FieldRepository
}

func NewTopicService(
	topicRepo domain.TopicRepository,
	fieldRepo domain.FieldRepository,
) *TopicService {
	return &TopicService{
		topicRepo: topicRepo,
		fieldRepo: fieldRepo,
	}
}

func (s *TopicService) CreateTopic(ctx context.Context, name string, description string) (*domain.Topic, error) {
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

	if err := s.topicRepo.Create(ctx, topic); err != nil {
		return nil, err
	}

	return topic, nil
}

func (s *TopicService) CreateField(
	ctx context.Context,
	topicUID uuid.UUID,
	input CreateFieldInput,
) (*domain.TopicField, error) {

	// 1. ตรวจสอบก่อนว่า Topic มีจริง
	_, err := s.topicRepo.FindByID(ctx, topicUID)
	if err != nil {
		return nil, err
	}

	input.Label = strings.TrimSpace(input.Label)
	// 2. ให้ Domain เป็นคนสร้าง TopicField
	field, err := domain.NewTopicField(
		topicUID,
		input.Label,
		input.Type,
		input.Required,
		input.Position,
	)
	if err != nil {
		return nil, err
	}

	// 3. บันทึกลง Repository
	if err := s.fieldRepo.Create(ctx, field); err != nil {
		return nil, err
	}

	return field, nil
}

func (s *TopicService) FindAll(ctx context.Context) ([]domain.Topic, error) {
	return s.topicRepo.FindAll(ctx)
}

func (s *TopicService) FindByID(ctx context.Context, topicID uuid.UUID) (*domain.Topic, error) {
	return s.topicRepo.FindByID(ctx, topicID)
}

func (s *TopicService) Update(ctx context.Context, topicID uuid.UUID, name string, description string, isActive bool) (*domain.Topic, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, domain.ErrTopicNameEmpty
	}

	topic, err := s.topicRepo.FindByID(ctx, topicID)
	if err != nil {
		return nil, err
	}

	topic.Name = name
	topic.Description = description
	topic.IsActive = isActive
	if err := s.topicRepo.Update(ctx, topic); err != nil {
		return nil, err
	}

	return topic, nil
}

func (s *TopicService) Delete(ctx context.Context, topicID uuid.UUID) error {
	_, err := s.topicRepo.FindByID(ctx, topicID)
	if err != nil {
		return err
	}

	return s.topicRepo.Delete(ctx, topicID)
}
