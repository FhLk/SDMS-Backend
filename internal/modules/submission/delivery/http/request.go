package http

import (
	"encoding/json"

	"github.com/google/uuid"
)

type CreateSubmissionRequest struct {
	SubmittedBy uuid.UUID                      `json:"submitted_by"`
	Values      []CreateSubmissionValueRequest `json:"values"`
}

type CreateSubmissionValueRequest struct {
	FieldUID uuid.UUID       `json:"field_uid"`
	Value    json.RawMessage `json:"value"`
}
