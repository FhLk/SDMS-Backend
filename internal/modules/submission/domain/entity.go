package domain

import (
	"time"

	"github.com/google/uuid"
)

type CompletionStatus string

const (
	CompletionIncomplete CompletionStatus = "INCOMPLETE"
	CompletionComplete   CompletionStatus = "COMPLETE"
)

type MissingRequiredField struct {
	FieldUID uuid.UUID
	Label    string
	Type     string
}

type FormSnapshotOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FormSnapshotField struct {
	UID       uuid.UUID            `json:"uid"`
	Label     string               `json:"label"`
	Type      string               `json:"type"`
	Required  bool                 `json:"required"`
	IsPreview bool                 `json:"is_preview"`
	Position  int                  `json:"position"`
	Options   []FormSnapshotOption `json:"options"`
}

type Submission struct {
	UID                   uuid.UUID
	TopicUID              uuid.UUID
	SubmittedBy           uuid.UUID
	FormVersion           int
	FormSnapshot          []FormSnapshotField
	Values                []SubmissionValue
	Files                 []SubmissionFile
	CompletionStatus      CompletionStatus
	MissingRequiredFields []MissingRequiredField
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type SubmissionValue struct {
	UID            uuid.UUID
	SubmissionUID  uuid.UUID
	FieldUID       uuid.UUID
	FieldLabel     string
	FieldType      string
	FieldIsPreview bool
	FieldPosition  int
	TextValue      *string
	NumberValue    *float64
	DateValue      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewSubmission(topicUID, submittedBy uuid.UUID, formVersion int, values []SubmissionValue) (*Submission, error) {
	if topicUID == uuid.Nil {
		return nil, ErrSubmissionTopicUIDRequired
	}
	if submittedBy == uuid.Nil {
		return nil, ErrSubmissionSubmittedByRequired
	}
	if formVersion <= 0 {
		formVersion = 1
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
		FormVersion: formVersion,
		Values:      values,
	}, nil
}
