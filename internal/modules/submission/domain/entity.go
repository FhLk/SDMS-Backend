package domain

import (
	"time"

	"github.com/google/uuid"
)

type Submission struct {
	UID         uuid.UUID
	TopicUID    uuid.UUID
	SubmittedBy uuid.UUID
	Values      []SubmissionValue
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SubmissionValue struct {
	UID           uuid.UUID
	SubmissionUID uuid.UUID
	FieldUID      uuid.UUID
	TextValue     *string
	NumberValue   *float64
	DateValue     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewSubmission(
	topicUID uuid.UUID,
	submittedBy uuid.UUID,
	values []SubmissionValue,
) (*Submission, error) {
	if topicUID == uuid.Nil {
		return nil, ErrSubmissionTopicUIDRequired
	}

	if submittedBy == uuid.Nil {
		return nil, ErrSubmissionSubmittedByRequired
	}

	submissionUID := uuid.New()

	for i := range values {
		values[i].UID = uuid.New()
		values[i].SubmissionUID = submissionUID
	}

	return &Submission{
		UID:         submissionUID,
		TopicUID:    topicUID,
		SubmittedBy: submittedBy,
		Values:      values,
	}, nil
}
