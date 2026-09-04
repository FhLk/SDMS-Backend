package usecase

import (
	"encoding/json"

	"github.com/google/uuid"
)

type CreateSubmissionInput struct {
	SubmittedBy uuid.UUID
	Values      []SubmissionValueInput
}

type SubmissionValueInput struct {
	FieldUID uuid.UUID
	Value    json.RawMessage
}
