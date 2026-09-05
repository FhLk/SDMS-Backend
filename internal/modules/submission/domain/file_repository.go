package domain

import (
	"context"

	"github.com/google/uuid"
)

type SubmissionFileRepository interface {
	Create(ctx context.Context, file *SubmissionFile) error

	FindAllBySubmissionID(
		ctx context.Context,
		submissionUID uuid.UUID,
	) ([]SubmissionFile, error)

	FindByID(
		ctx context.Context,
		fileUID uuid.UUID,
	) (*SubmissionFile, error)

	FindBySubmissionIDAndFieldID(
		ctx context.Context,
		submissionUID uuid.UUID,
		fieldUID uuid.UUID,
	) (*SubmissionFile, error)

	Delete(ctx context.Context, fileUID uuid.UUID) error
}
