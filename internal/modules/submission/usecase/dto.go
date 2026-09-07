package usecase

import (
	"encoding/json"
	"time"

	submissiondomain "sdms/internal/modules/submission/domain"
	userdomain "sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type CreateSubmissionInput struct {
	SubmittedBy uuid.UUID
	Values      []SubmissionValueInput
}

type UpdateSubmissionInput struct {
	Values []SubmissionValueInput
}

type SubmissionValueInput struct {
	FieldUID uuid.UUID
	Value    json.RawMessage
}

type TopicSubmissionStatus struct {
	Teacher               userdomain.User
	Status                string
	LatestSubmissionUID   *uuid.UUID
	LatestSubmissionAt    *time.Time
	MissingRequiredFields []submissiondomain.MissingRequiredField
}
