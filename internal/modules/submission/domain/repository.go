package domain

import (
	"context"

	"github.com/google/uuid"
)

type SubmissionRepository interface {
	Create(ctx context.Context, submission *Submission) error
	UpdateValues(ctx context.Context, submission *Submission) error
	Delete(ctx context.Context, submissionUID uuid.UUID) error

	FindAllByTopicID(ctx context.Context, topicUID uuid.UUID) ([]Submission, error)
	FindAllByTopicIDAndSubmittedBy(ctx context.Context, topicUID, submittedBy uuid.UUID) ([]Submission, error)
	FindByIDAndTopicID(ctx context.Context, submissionUID, topicUID uuid.UUID) (*Submission, error)
	FindByIDAndTopicIDAndSubmittedBy(ctx context.Context, submissionUID, topicUID, submittedBy uuid.UUID) (*Submission, error)
	HasAnyByTopicID(ctx context.Context, topicUID uuid.UUID) (bool, error)
}
