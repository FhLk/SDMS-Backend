package domain

import (
	"context"

	"github.com/google/uuid"
)

type SubmissionRepository interface {
	Create(
		ctx context.Context,
		submission *Submission,
	) error

	FindAllByTopicID(
		ctx context.Context,
		topicUID uuid.UUID,
	) ([]Submission, error)

	FindByIDAndTopicID(
		ctx context.Context,
		submissionUID uuid.UUID,
		topicUID uuid.UUID,
	) (*Submission, error)
}
