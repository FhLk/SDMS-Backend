package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type SubmissionService struct {
	submissionRepo submissiondomain.SubmissionRepository
	topicRepo      topicdomain.TopicRepository
	fieldRepo      topicdomain.FieldRepository
}

func NewSubmissionService(
	submissionRepo submissiondomain.SubmissionRepository,
	topicRepo topicdomain.TopicRepository,
	fieldRepo topicdomain.FieldRepository,
) *SubmissionService {
	return &SubmissionService{
		submissionRepo: submissionRepo,
		topicRepo:      topicRepo,
		fieldRepo:      fieldRepo,
	}
}

func (s *SubmissionService) Create(
	ctx context.Context,
	topicUID uuid.UUID,
	input CreateSubmissionInput,
) (*submissiondomain.Submission, error) {
	// 1. Topic ต้องมีจริง
	if _, err := s.topicRepo.FindByID(ctx, topicUID); err != nil {
		return nil, err
	}

	// 2. โหลด Fields ของ Topic
	fields, err := s.fieldRepo.FindAllByTopicID(ctx, topicUID)
	if err != nil {
		return nil, err
	}

	fieldMap := make(map[uuid.UUID]topicdomain.TopicField, len(fields))

	for _, field := range fields {
		fieldMap[field.UID] = field
	}

	// 3. ตรวจ field ที่ client ส่งเข้ามา
	inputMap := make(map[uuid.UUID]json.RawMessage, len(input.Values))

	for _, inputValue := range input.Values {
		if inputValue.FieldUID == uuid.Nil {
			return nil, submissiondomain.ErrSubmissionInvalidField
		}

		if _, exists := inputMap[inputValue.FieldUID]; exists {
			return nil, fmt.Errorf(
				"%w: %s",
				submissiondomain.ErrSubmissionDuplicateField,
				inputValue.FieldUID,
			)
		}

		if _, exists := fieldMap[inputValue.FieldUID]; !exists {
			return nil, fmt.Errorf(
				"%w: %s",
				submissiondomain.ErrSubmissionInvalidField,
				inputValue.FieldUID,
			)
		}

		inputMap[inputValue.FieldUID] = inputValue.Value
	}

	values := make([]submissiondomain.SubmissionValue, 0, len(input.Values))

	// 4. Validate ตาม Field Definition
	for _, field := range fields {
		rawValue, exists := inputMap[field.UID]

		if !exists || isNull(rawValue) {
			if field.Required {
				if field.Type == topicdomain.FieldTypeFile {
					return nil, fmt.Errorf(
						"%w: %s",
						submissiondomain.ErrSubmissionFileFieldUnsupported,
						field.Label,
					)
				}

				return nil, fmt.Errorf(
					"%w: %s",
					submissiondomain.ErrSubmissionRequiredFieldMissing,
					field.Label,
				)
			}

			continue
		}

		if field.Type == topicdomain.FieldTypeFile {
			return nil, fmt.Errorf(
				"%w: %s",
				submissiondomain.ErrSubmissionFileFieldUnsupported,
				field.Label,
			)
		}

		value, err := parseSubmissionValue(field, rawValue)
		if err != nil {
			return nil, err
		}

		values = append(values, value)
	}

	// 5. สร้าง Domain
	submission, err := submissiondomain.NewSubmission(
		topicUID,
		input.SubmittedBy,
		values,
	)
	if err != nil {
		return nil, err
	}

	// 6. บันทึก Submission + Values
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

func parseSubmissionValue(
	field topicdomain.TopicField,
	raw json.RawMessage,
) (submissiondomain.SubmissionValue, error) {
	value := submissiondomain.SubmissionValue{
		FieldUID: field.UID,
	}

	switch field.Type {

	case topicdomain.FieldTypeText,
		topicdomain.FieldTypeTextarea,
		topicdomain.FieldTypeSelect:

		var text string

		if err := json.Unmarshal(raw, &text); err != nil {
			return value, invalidValueError(field)
		}

		text = strings.TrimSpace(text)

		if field.Required && text == "" {
			return value, invalidValueError(field)
		}

		value.TextValue = &text

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

		date, err := time.Parse("2006-01-02", dateString)
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
	return fmt.Errorf(
		"%w: %s",
		submissiondomain.ErrSubmissionInvalidValue,
		field.Label,
	)
}

func isNull(raw json.RawMessage) bool {
	return len(raw) == 0 ||
		bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
