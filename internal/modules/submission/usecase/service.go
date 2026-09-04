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

type SubmissionService struct {
	submissionRepo submissiondomain.SubmissionRepository
	topicRepo      topicdomain.TopicRepository
	fieldRepo      topicdomain.FieldRepository
	userRepo       UserLookupRepository
}

// userRepo is optional to keep isolated unit tests/backward construction simple.
// Production wiring passes it so submitted_by is always validated.
func NewSubmissionService(
	submissionRepo submissiondomain.SubmissionRepository,
	topicRepo topicdomain.TopicRepository,
	fieldRepo topicdomain.FieldRepository,
	userRepos ...UserLookupRepository,
) *SubmissionService {
	var userRepo UserLookupRepository
	if len(userRepos) > 0 {
		userRepo = userRepos[0]
	}

	return &SubmissionService{
		submissionRepo: submissionRepo,
		topicRepo:      topicRepo,
		fieldRepo:      fieldRepo,
		userRepo:       userRepo,
	}
}

func (s *SubmissionService) Create(
	ctx context.Context,
	topicUID uuid.UUID,
	input CreateSubmissionInput,
) (*submissiondomain.Submission, error) {
	// 1. Topic ต้องมีจริงและเปิดรับข้อมูล
	topic, err := s.topicRepo.FindByID(ctx, topicUID)
	if err != nil {
		return nil, err
	}
	if !topic.IsActive {
		return nil, submissiondomain.ErrSubmissionTopicInactive
	}

	// 2. submitted_by ต้องอ้างถึงครูที่มีอยู่จริงและยัง active
	if input.SubmittedBy == uuid.Nil {
		return nil, submissiondomain.ErrSubmissionSubmittedByRequired
	}
	if s.userRepo != nil {
		user, err := s.userRepo.FindByID(ctx, input.SubmittedBy)
		if err != nil {
			if errors.Is(err, userdomain.ErrUserNotFound) {
				return nil, submissiondomain.ErrSubmissionSubmitterNotFound
			}
			return nil, err
		}

		if user.Role != userdomain.RoleTeacher {
			return nil, submissiondomain.ErrSubmissionSubmitterMustBeTeacher
		}
		if user.Status != userdomain.StatusActive {
			return nil, submissiondomain.ErrSubmissionSubmitterInactive
		}
	}

	// 3. โหลด Fields ของ Topic
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}

	fieldMap := make(map[uuid.UUID]topicdomain.TopicField, len(fields))
	for _, field := range fields {
		fieldMap[field.UID] = field
	}

	// 4. ตรวจ field ที่ client ส่งเข้ามา
	inputMap := make(map[uuid.UUID]json.RawMessage, len(input.Values))
	for _, inputValue := range input.Values {
		if inputValue.FieldUID == uuid.Nil {
			return nil, submissiondomain.NewFieldError(
				submissiondomain.ErrSubmissionInvalidField,
				inputValue.FieldUID,
				"",
			)
		}

		if _, exists := inputMap[inputValue.FieldUID]; exists {
			label := ""
			if field, ok := fieldMap[inputValue.FieldUID]; ok {
				label = field.Label
			}
			return nil, submissiondomain.NewFieldError(
				submissiondomain.ErrSubmissionDuplicateField,
				inputValue.FieldUID,
				label,
			)
		}

		if _, exists := fieldMap[inputValue.FieldUID]; !exists {
			return nil, submissiondomain.NewFieldError(
				submissiondomain.ErrSubmissionInvalidField,
				inputValue.FieldUID,
				"",
			)
		}

		inputMap[inputValue.FieldUID] = inputValue.Value
	}

	values := make([]submissiondomain.SubmissionValue, 0, len(input.Values))

	// 5. Validate ตาม Field Definition
	for _, field := range fields {
		rawValue, exists := inputMap[field.UID]

		// Optional field ที่ไม่ส่ง, null หรือ "" ให้ถือว่าไม่ได้กรอก
		if !exists || isEmptySubmissionInput(rawValue) {
			if field.Required {
				if field.Type == topicdomain.FieldTypeFile {
					return nil, submissiondomain.NewFieldError(
						submissiondomain.ErrSubmissionFileFieldUnsupported,
						field.UID,
						field.Label,
					)
				}

				return nil, submissiondomain.NewFieldError(
					submissiondomain.ErrSubmissionRequiredFieldMissing,
					field.UID,
					field.Label,
				)
			}
			continue
		}

		if field.Type == topicdomain.FieldTypeFile {
			return nil, submissiondomain.NewFieldError(
				submissiondomain.ErrSubmissionFileFieldUnsupported,
				field.UID,
				field.Label,
			)
		}

		value, err := parseSubmissionValue(field, rawValue)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	// 6. สร้าง Domain
	submission, err := submissiondomain.NewSubmission(
		topicUID,
		input.SubmittedBy,
		values,
	)
	if err != nil {
		return nil, err
	}

	// 7. บันทึก Submission + Values
	if err := s.submissionRepo.Create(ctx, submission); err != nil {
		return nil, err
	}

	return submission, nil
}

func (s *SubmissionService) FindAllByTopicID(
	ctx context.Context,
	topicUID uuid.UUID,
) ([]submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}

	return s.submissionRepo.FindAllByTopicID(ctx, topicUID)
}

func (s *SubmissionService) FindAllByTopicIDAndSubmittedBy(
	ctx context.Context,
	topicUID uuid.UUID,
	submittedBy uuid.UUID,
) ([]submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}

	return s.submissionRepo.FindAllByTopicIDAndSubmittedBy(
		ctx,
		topicUID,
		submittedBy,
	)
}

func (s *SubmissionService) FindByID(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
) (*submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}

	return s.submissionRepo.FindByIDAndTopicID(
		ctx,
		submissionUID,
		topicUID,
	)
}

func (s *SubmissionService) FindByIDForSubmitter(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
	submittedBy uuid.UUID,
) (*submissiondomain.Submission, error) {
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}

	return s.submissionRepo.FindByIDAndTopicIDAndSubmittedBy(
		ctx,
		submissionUID,
		topicUID,
		submittedBy,
	)
}

func parseSubmissionValue(
	field topicdomain.TopicField,
	raw json.RawMessage,
) (submissiondomain.SubmissionValue, error) {
	value := submissiondomain.SubmissionValue{
		FieldUID: field.UID,
	}

	switch field.Type {
	case topicdomain.FieldTypeText,
		topicdomain.FieldTypeTextarea:
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
	return submissiondomain.NewFieldError(
		submissiondomain.ErrSubmissionInvalidValue,
		field.UID,
		field.Label,
	)
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
