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

func (s *TopicService) FindTopicWithFields(ctx context.Context, topicID uuid.UUID) (*domain.Topic, []domain.TopicField, error) {
	topic, err := s.topicRepo.FindByID(ctx, topicID)
	if err != nil {
		return nil, nil, err
	}

	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicID)
	if err != nil {
		return nil, nil, err
	}

	return topic, fields, nil
}

func (s *TopicService) FindFieldsByTopicID(ctx context.Context, topicID uuid.UUID) ([]domain.TopicField, error) {
	if topicID == uuid.Nil {
		return nil, domain.ErrTopicFieldInvalidTopicUID
	}

	if _, err := s.topicRepo.FindByID(ctx, topicID); err != nil {
		return nil, err
	}

	return s.fieldRepo.FindAllByTopicID(ctx, topicID)
}

func (s *TopicService) FindFieldByID(ctx context.Context, topicID uuid.UUID, fieldID uuid.UUID) (*domain.TopicField, error) {
	return s.findFieldInTopic(ctx, topicID, fieldID)
}

func (s *TopicService) UpdateField(ctx context.Context, topicID uuid.UUID, fieldID uuid.UUID, input UpdateFieldInput) (*domain.TopicField, error) {
	field, err := s.findFieldInTopic(ctx, topicID, fieldID)
	if err != nil {
		return nil, err
	}

	input.Label = strings.TrimSpace(input.Label)

	if err := field.Update(
		input.Label,
		input.Type,
		input.Required,
		input.Position,
	); err != nil {
		return nil, err
	}

	if err := s.fieldRepo.Update(ctx, field); err != nil {
		return nil, err
	}

	return field, nil
}

func (s *TopicService) DeleteField(ctx context.Context, topicID uuid.UUID, fieldID uuid.UUID) error {
	if _, err := s.findFieldInTopic(ctx, topicID, fieldID); err != nil {
		return err
	}

	return s.fieldRepo.Delete(ctx, fieldID)
}

func (s *TopicService) findFieldInTopic(ctx context.Context, topicID uuid.UUID, fieldID uuid.UUID) (*domain.TopicField, error) {
	if topicID == uuid.Nil {
		return nil, domain.ErrTopicFieldInvalidTopicUID
	}

	if _, err := s.topicRepo.FindByID(ctx, topicID); err != nil {
		return nil, err
	}

	field, err := s.fieldRepo.FindByID(ctx, fieldID)
	if err != nil {
		return nil, err
	}

	if field.TopicUID != topicID {
		return nil, domain.ErrTopicFieldNotFound
	}

	return field, nil
}
