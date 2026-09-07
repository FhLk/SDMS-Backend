package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"
	userdomain "sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type UserLookupRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error)
}

type userListRepository interface {
	List(ctx context.Context) ([]userdomain.User, error)
}

type SubmissionService struct {
	submissionRepo submissiondomain.SubmissionRepository
	topicRepo      topicdomain.TopicRepository
	fieldRepo      topicdomain.FieldRepository
	userRepo       UserLookupRepository
	userListRepo   userListRepository
}

func NewSubmissionService(
	submissionRepo submissiondomain.SubmissionRepository,
	topicRepo topicdomain.TopicRepository,
	fieldRepo topicdomain.FieldRepository,
	userRepos ...UserLookupRepository,
) *SubmissionService {
	var userRepo UserLookupRepository
	var userListRepo userListRepository
	if len(userRepos) > 0 {
		userRepo = userRepos[0]
		if listRepo, ok := userRepos[0].(userListRepository); ok {
			userListRepo = listRepo
		}
	}
	return &SubmissionService{
		submissionRepo: submissionRepo,
		topicRepo:      topicRepo,
		fieldRepo:      fieldRepo,
		userRepo:       userRepo,
		userListRepo:   userListRepo,
	}
}

func (s *SubmissionService) Create(ctx context.Context, topicUID uuid.UUID, input CreateSubmissionInput) (*submissiondomain.Submission, error) {
	topic, err := s.requireActiveTopic(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSubmitter(ctx, input.SubmittedBy); err != nil {
		return nil, err
	}

	fields, values, err := s.validateAndBuildValues(ctx, topicUID, input.Values)
	if err != nil {
		return nil, err
	}
	submission, err := submissiondomain.NewSubmission(topicUID, input.SubmittedBy, topic.FormVersion, values)
	if err != nil {
		return nil, err
	}
	submission.FormSnapshot = buildFormSnapshot(fields)
	if err := s.submissionRepo.Create(ctx, submission); err != nil {
		return nil, err
	}
	s.decorateCompletion(submission, fields)
	return submission, nil
}

func (s *SubmissionService) UpdateForSubmitter(
	ctx context.Context,
	topicUID, submissionUID, submittedBy uuid.UUID,
	input UpdateSubmissionInput,
) (*submissiondomain.Submission, error) {
	topic, err := s.requireActiveTopic(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	existing, err := s.submissionRepo.FindByIDAndTopicIDAndSubmittedBy(ctx, submissionUID, topicUID, submittedBy)
	if err != nil {
		return nil, err
	}
	fields, values, err := s.validateAndBuildValues(ctx, topicUID, input.Values)
	if err != nil {
		return nil, err
	}
	existing.FormVersion = topic.FormVersion
	existing.FormSnapshot = buildFormSnapshot(fields)
	existing.Values = values
	for i := range existing.Values {
		existing.Values[i].SubmissionUID = existing.UID
	}
	if err := s.submissionRepo.UpdateValues(ctx, existing); err != nil {
		return nil, err
	}
	fresh, err := s.submissionRepo.FindByIDAndTopicIDAndSubmittedBy(ctx, submissionUID, topicUID, submittedBy)
	if err != nil {
		return nil, err
	}
	s.decorateCompletion(fresh, fields)
	return fresh, nil
}

func (s *SubmissionService) DeleteForSubmitter(ctx context.Context, topicUID, submissionUID, submittedBy uuid.UUID) error {
	if _, err := s.submissionRepo.FindByIDAndTopicIDAndSubmittedBy(ctx, submissionUID, topicUID, submittedBy); err != nil {
		return err
	}
	return s.submissionRepo.Delete(ctx, submissionUID)
}

func (s *SubmissionService) FindAllByTopicID(ctx context.Context, topicUID uuid.UUID) ([]submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	submissions, err := s.submissionRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	s.decorateAll(submissions, fields)
	return submissions, nil
}

func (s *SubmissionService) FindAllByTopicIDAndSubmittedBy(ctx context.Context, topicUID, submittedBy uuid.UUID) ([]submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	submissions, err := s.submissionRepo.FindAllByTopicIDAndSubmittedBy(ctx, topicUID, submittedBy)
	if err != nil {
		return nil, err
	}
	s.decorateAll(submissions, fields)
	return submissions, nil
}

func (s *SubmissionService) FindByID(ctx context.Context, topicUID, submissionUID uuid.UUID) (*submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	submission, err := s.submissionRepo.FindByIDAndTopicID(ctx, submissionUID, topicUID)
	if err != nil {
		return nil, err
	}
	s.decorateCompletion(submission, fields)
	return submission, nil
}

func (s *SubmissionService) FindByIDForSubmitter(ctx context.Context, topicUID, submissionUID, submittedBy uuid.UUID) (*submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	submission, err := s.submissionRepo.FindByIDAndTopicIDAndSubmittedBy(ctx, submissionUID, topicUID, submittedBy)
	if err != nil {
		return nil, err
	}
	s.decorateCompletion(submission, fields)
	return submission, nil
}

// GetTopicSubmissionStatus gives the Phase-1 management view requested by SDMS:
// every active teacher is represented, including teachers who have not submitted anything.
func (s *SubmissionService) GetTopicSubmissionStatus(ctx context.Context, topicUID uuid.UUID) ([]TopicSubmissionStatus, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}
	if s.userListRepo == nil {
		return nil, errors.New("user listing repository is not configured")
	}
	users, err := s.userListRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	submissions, err := s.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	latestByTeacher := make(map[uuid.UUID]submissiondomain.Submission)
	for _, submission := range submissions {
		if _, exists := latestByTeacher[submission.SubmittedBy]; !exists {
			latestByTeacher[submission.SubmittedBy] = submission
		}
	}

	result := make([]TopicSubmissionStatus, 0)
	for _, user := range users {
		if user.Role != userdomain.RoleTeacher || user.Status != userdomain.StatusActive {
			continue
		}
		item := TopicSubmissionStatus{Teacher: user, Status: "NOT_SUBMITTED"}
		if submission, ok := latestByTeacher[user.UID]; ok {
			uid := submission.UID
			updatedAt := submission.UpdatedAt
			item.LatestSubmissionUID = &uid
			item.LatestSubmissionAt = &updatedAt
			item.MissingRequiredFields = submission.MissingRequiredFields
			item.Status = string(submission.CompletionStatus)
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *SubmissionService) requireActiveTopic(ctx context.Context, topicUID uuid.UUID) (*topicdomain.Topic, error) {
	topic, err := s.topicRepo.FindByID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	if !topic.IsActive {
		return nil, submissiondomain.ErrSubmissionTopicInactive
	}
	return topic, nil
}

func (s *SubmissionService) validateSubmitter(ctx context.Context, submittedBy uuid.UUID) error {
	if submittedBy == uuid.Nil {
		return submissiondomain.ErrSubmissionSubmittedByRequired
	}
	if s.userRepo == nil {
		return nil
	}
	user, err := s.userRepo.FindByID(ctx, submittedBy)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			return submissiondomain.ErrSubmissionSubmitterNotFound
		}
		return err
	}
	if user.Role != userdomain.RoleTeacher {
		return submissiondomain.ErrSubmissionSubmitterMustBeTeacher
	}
	if user.Status != userdomain.StatusActive {
		return submissiondomain.ErrSubmissionSubmitterInactive
	}
	return nil
}

func (s *SubmissionService) validateAndBuildValues(
	ctx context.Context,
	topicUID uuid.UUID,
	inputs []SubmissionValueInput,
) ([]topicdomain.TopicField, []submissiondomain.SubmissionValue, error) {
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, nil, err
	}
	fieldMap := make(map[uuid.UUID]topicdomain.TopicField, len(fields))
	for _, field := range fields {
		fieldMap[field.UID] = field
	}
	inputMap := make(map[uuid.UUID]json.RawMessage, len(inputs))
	for _, inputValue := range inputs {
		if inputValue.FieldUID == uuid.Nil {
			return nil, nil, submissiondomain.NewFieldError(submissiondomain.ErrSubmissionInvalidField, inputValue.FieldUID, "")
		}
		if _, exists := inputMap[inputValue.FieldUID]; exists {
			label := ""
			if field, ok := fieldMap[inputValue.FieldUID]; ok {
				label = field.Label
			}
			return nil, nil, submissiondomain.NewFieldError(submissiondomain.ErrSubmissionDuplicateField, inputValue.FieldUID, label)
		}
		if _, exists := fieldMap[inputValue.FieldUID]; !exists {
			return nil, nil, submissiondomain.NewFieldError(submissiondomain.ErrSubmissionInvalidField, inputValue.FieldUID, "")
		}
		inputMap[inputValue.FieldUID] = inputValue.Value
	}

	values := make([]submissiondomain.SubmissionValue, 0, len(inputs))
	for _, field := range fields {
		rawValue, exists := inputMap[field.UID]
		if field.Type == topicdomain.FieldTypeFile {
			if exists && !isEmptySubmissionInput(rawValue) {
				return nil, nil, submissiondomain.NewFieldError(submissiondomain.ErrSubmissionFileFieldUnsupported, field.UID, field.Label)
			}
			continue
		}
		if !exists || isEmptySubmissionInput(rawValue) {
			if field.Required {
				return nil, nil, submissiondomain.NewFieldError(submissiondomain.ErrSubmissionRequiredFieldMissing, field.UID, field.Label)
			}
			continue
		}
		value, err := parseSubmissionValue(field, rawValue)
		if err != nil {
			return nil, nil, err
		}
		values = append(values, value)
	}
	return fields, values, nil
}

func (s *SubmissionService) decorateAll(submissions []submissiondomain.Submission, fields []topicdomain.TopicField) {
	for i := range submissions {
		s.decorateCompletion(&submissions[i], fields)
	}
}

func (s *SubmissionService) decorateCompletion(submission *submissiondomain.Submission, fields []topicdomain.TopicField) {
	valueFields := make(map[uuid.UUID]struct{}, len(submission.Values))
	for _, value := range submission.Values {
		valueFields[value.FieldUID] = struct{}{}
	}
	fileFields := make(map[uuid.UUID]struct{}, len(submission.Files))
	for _, file := range submission.Files {
		fileFields[file.FieldUID] = struct{}{}
	}
	missing := make([]submissiondomain.MissingRequiredField, 0)
	for _, field := range fields {
		if !field.Required {
			continue
		}
		var present bool
		if field.Type == topicdomain.FieldTypeFile {
			_, present = fileFields[field.UID]
		} else {
			_, present = valueFields[field.UID]
		}
		if !present {
			missing = append(missing, submissiondomain.MissingRequiredField{
				FieldUID: field.UID, Label: field.Label, Type: string(field.Type),
			})
		}
	}
	submission.MissingRequiredFields = missing
	if len(missing) == 0 {
		submission.CompletionStatus = submissiondomain.CompletionComplete
	} else {
		submission.CompletionStatus = submissiondomain.CompletionIncomplete
	}
}

func buildFormSnapshot(fields []topicdomain.TopicField) []submissiondomain.FormSnapshotField {
	result := make([]submissiondomain.FormSnapshotField, 0, len(fields))
	for _, field := range fields {
		options := make([]submissiondomain.FormSnapshotOption, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, submissiondomain.FormSnapshotOption{Label: option.Label, Value: option.Value})
		}
		result = append(result, submissiondomain.FormSnapshotField{
			UID: field.UID, Label: field.Label, Type: string(field.Type), Required: field.Required,
			IsPreview: field.IsPreview, Position: field.Position, Options: options,
		})
	}
	return result
}

func parseSubmissionValue(field topicdomain.TopicField, raw json.RawMessage) (submissiondomain.SubmissionValue, error) {
	value := submissiondomain.SubmissionValue{
		FieldUID: field.UID, FieldLabel: field.Label, FieldType: string(field.Type),
		FieldIsPreview: field.IsPreview, FieldPosition: field.Position,
	}
	switch field.Type {
	case topicdomain.FieldTypeText, topicdomain.FieldTypeTextarea:
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return value, invalidValueError(field)
		}
		text = strings.TrimSpace(text)
		if field.Required && text == "" {
			return value, invalidValueError(field)
		}
		value.TextValue = &text
	case topicdomain.FieldTypeSelect:
		var selectedValue string
		if err := json.Unmarshal(raw, &selectedValue); err != nil {
			return value, invalidValueError(field)
		}
		selectedValue = strings.TrimSpace(selectedValue)
		if selectedValue == "" || !field.HasSelectOption(selectedValue) {
			return value, invalidValueError(field)
		}
		value.TextValue = &selectedValue
	case topicdomain.FieldTypeNumber:
		var number float64
		if err := json.Unmarshal(raw, &number); err != nil {
			return value, invalidValueError(field)
		}
		value.NumberValue = &number
	case topicdomain.FieldTypeDate:
		var dateString string
		if err := json.Unmarshal(raw, &dateString); err != nil {
			return value, invalidValueError(field)
		}
		date, err := time.Parse("2006-01-02", strings.TrimSpace(dateString))
		if err != nil {
			return value, invalidValueError(field)
		}
		value.DateValue = &date
	default:
		return value, invalidValueError(field)
	}
	return value, nil
}

func invalidValueError(field topicdomain.TopicField) error {
	return submissiondomain.NewFieldError(submissiondomain.ErrSubmissionInvalidValue, field.UID, field.Label)
}

func isNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func isEmptySubmissionInput(raw json.RawMessage) bool {
	if isNull(raw) {
		return true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text) == ""
	}
	return false
}
