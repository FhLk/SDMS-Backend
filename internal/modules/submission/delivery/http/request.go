package http

import (
	"encoding/json"

	"github.com/google/uuid"
)

type CreateSubmissionRequest struct {
	Values []CreateSubmissionValueRequest `json:"values"`
}

type CreateSubmissionValueRequest struct {
	FieldUID uuid.UUID       `json:"field_uid"`
	Value    json.RawMessage `json:"value"`
}
