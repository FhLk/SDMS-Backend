package usecase

import (
	"context"
	"reflect"
	"strings"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type SubmissionLookupRepository interface {
	HasAnyByTopicID(ctx context.Context, topicUID uuid.UUID) (bool, error)
}

type TopicService struct {
	topicRepo      domain.TopicRepository
	fieldRepo      domain.FieldRepository
	submissionRepo SubmissionLookupRepository
}

func NewTopicService(
	topicRepo domain.TopicRepository,
	fieldRepo domain.FieldRepository,
	submissionRepos ...SubmissionLookupRepository,
) *TopicService {
	var submissionRepo SubmissionLookupRepository
	if len(submissionRepos) > 0 {
		submissionRepo = submissionRepos[0]
	}
	return &TopicService{topicRepo: topicRepo, fieldRepo: fieldRepo, submissionRepo: submissionRepo}
}

func (s *TopicService) CreateTopic(ctx context.Context, academicYear, name, description string) (*domain.Topic, error) {
	topic := &domain.Topic{
		UID:          uuid.New(),
		AcademicYear: strings.TrimSpace(academicYear),
		Name:         strings.TrimSpace(name),
		Description:  strings.TrimSpace(description),
		IsActive:     true,
		FormVersion:  1,
	}
	if err := topic.Validate(); err != nil {
		return nil, err
	}
	if err := s.topicRepo.Create(ctx, topic); err != nil {
		return nil, err
	}
	return topic, nil
}

func (s *TopicService) CreateField(ctx context.Context, topicUID uuid.UUID, input CreateFieldInput) (*domain.TopicField, error) {
	topic, err := s.topicRepo.FindByID(ctx, topicUID)
	if err != nil {
		return nil, err
	}

	hasSubmissions, err := s.topicHasSubmissions(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	if hasSubmissions && input.Required {
		return nil, domain.ErrTopicRequiredFieldAddLocked
	}

	input.Label = strings.TrimSpace(input.Label)
	field, err := domain.NewTopicFieldWithOptions(topicUID, input.Label, input.Type, input.Required, input.Position, input.Options)
	if err != nil {
		return nil, err
	}
	field.IsPreview = input.IsPreview
	if err := s.fieldRepo.Create(ctx, field); err != nil {
		return nil, err
	}
	if hasSubmissions {
		if err := s.bumpFormVersion(ctx, topic); err != nil {
			return nil, err
		}
	}
	return field, nil
}

func (s *TopicService) FindAll(ctx context.Context) ([]domain.Topic, error) {
	return s.topicRepo.FindAll(ctx)
}

func (s *TopicService) FindAllByAcademicYear(ctx context.Context, academicYear string) ([]domain.Topic, error) {
	academicYear = strings.TrimSpace(academicYear)
	if academicYear == "" {
		return s.FindAll(ctx)
	}
	return s.topicRepo.FindAllByAcademicYear(ctx, academicYear)
}

func (s *TopicService) FindByID(ctx context.Context, topicID uuid.UUID) (*domain.Topic, error) {
	return s.topicRepo.FindByID(ctx, topicID)
}

func (s *TopicService) Update(ctx context.Context, topicID uuid.UUID, academicYear, name, description string, isActive bool) (*domain.Topic, error) {
	newAcademicYear := strings.TrimSpace(academicYear)
	newName := strings.TrimSpace(name)
	newDescription := strings.TrimSpace(description)
	if newAcademicYear == "" {
		return nil, domain.ErrTopicAcademicYearRequired
	}
	if newName == "" {
		return nil, domain.ErrTopicNameEmpty
	}

	topic, err := s.topicRepo.FindByID(ctx, topicID)
	if err != nil {
		return nil, err
	}
	if newAcademicYear != topic.AcademicYear {
		hasSubmissions, err := s.topicHasSubmissions(ctx, topicID)
		if err != nil {
			return nil, err
		}
		if hasSubmissions {
			return nil, domain.ErrTopicAcademicYearLocked
		}
	}

	topic.AcademicYear = newAcademicYear
	topic.Name = newName
	topic.Description = newDescription
	topic.IsActive = isActive
	if err := topic.Validate(); err != nil {
		return nil, err
	}
	if err := s.topicRepo.Update(ctx, topic); err != nil {
		return nil, err
	}
	return topic, nil
}

func (s *TopicService) Delete(ctx context.Context, topicID uuid.UUID) error {
	if _, err := s.topicRepo.FindByID(ctx, topicID); err != nil {
		return err
	}
	hasSubmissions, err := s.topicHasSubmissions(ctx, topicID)
	if err != nil {
		return err
	}
	if hasSubmissions {
		return domain.ErrTopicDeleteLocked
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

func (s *TopicService) FindFieldByID(ctx context.Context, topicID, fieldID uuid.UUID) (*domain.TopicField, error) {
	return s.findFieldInTopic(ctx, topicID, fieldID)
}

func (s *TopicService) UpdateField(ctx context.Context, topicID, fieldID uuid.UUID, input UpdateFieldInput) (*domain.TopicField, error) {
	field, err := s.findFieldInTopic(ctx, topicID, fieldID)
	if err != nil {
		return nil, err
	}

	input.Label = strings.TrimSpace(input.Label)
	candidate := *field
	if err := candidate.Update(input.Label, input.Type, input.Required, input.Position, input.Options); err != nil {
		return nil, err
	}
	candidate.IsPreview = input.IsPreview

	hasSubmissions, err := s.topicHasSubmissions(ctx, topicID)
	if err != nil {
		return nil, err
	}
	if hasSubmissions && validationSchemaChanged(*field, candidate) {
		if candidate.Type != field.Type {
			return nil, domain.ErrTopicFieldTypeLocked
		}
		return nil, domain.ErrTopicFieldSchemaLocked
	}

	*field = candidate
	if err := s.fieldRepo.Update(ctx, field); err != nil {
		return nil, err
	}
	if hasSubmissions {
		topic, err := s.topicRepo.FindByID(ctx, topicID)
		if err != nil {
			return nil, err
		}
		if err := s.bumpFormVersion(ctx, topic); err != nil {
			return nil, err
		}
	}
	return field, nil
}

func (s *TopicService) DeleteField(ctx context.Context, topicID, fieldID uuid.UUID) error {
	if _, err := s.findFieldInTopic(ctx, topicID, fieldID); err != nil {
		return err
	}
	hasSubmissions, err := s.topicHasSubmissions(ctx, topicID)
	if err != nil {
		return err
	}
	if hasSubmissions {
		return domain.ErrTopicFieldDeleteLocked
	}
	return s.fieldRepo.Delete(ctx, fieldID)
}

func validationSchemaChanged(before, after domain.TopicField) bool {
	return before.Type != after.Type || before.Required != after.Required || !reflect.DeepEqual(before.Options, after.Options)
}

func (s *TopicService) bumpFormVersion(ctx context.Context, topic *domain.Topic) error {
	topic.FormVersion++
	return s.topicRepo.Update(ctx, topic)
}

func (s *TopicService) topicHasSubmissions(ctx context.Context, topicID uuid.UUID) (bool, error) {
	if s.submissionRepo == nil {
		return false, nil
	}
	return s.submissionRepo.HasAnyByTopicID(ctx, topicID)
}

func (s *TopicService) findFieldInTopic(ctx context.Context, topicID, fieldID uuid.UUID) (*domain.TopicField, error) {
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
