package http

import (
	"encoding/json"

	"github.com/google/uuid"
)

type CreateSubmissionRequest struct {
	Values []SubmissionValueRequest `json:"values"`
}

type UpdateSubmissionRequest = CreateSubmissionRequest

type SubmissionValueRequest struct {
	FieldUID uuid.UUID       `json:"field_uid"`
	Value    json.RawMessage `json:"value"`
}
