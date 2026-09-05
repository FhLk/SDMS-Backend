package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrSubmissionNotFound = errors.New(
		"submission not found",
	)

	ErrSubmissionTopicUIDRequired = errors.New(
		"submission topic uid is required",
	)

	ErrSubmissionSubmittedByRequired = errors.New(
		"submitted by is required",
	)

	ErrSubmissionSubmitterNotFound = errors.New(
		"submission submitter not found",
	)

	ErrSubmissionSubmitterMustBeTeacher = errors.New(
		"submission submitter must be a teacher",
	)

	ErrSubmissionSubmitterInactive = errors.New(
		"submission submitter is inactive",
	)

	ErrSubmissionTopicInactive = errors.New(
		"topic is not accepting submissions",
	)

	ErrSubmissionInvalidField = errors.New(
		"invalid submission field",
	)

	ErrSubmissionDuplicateField = errors.New(
		"duplicate submission field",
	)

	ErrSubmissionRequiredFieldMissing = errors.New(
		"required field is missing",
	)

	ErrSubmissionInvalidValue = errors.New(
		"invalid submission value",
	)

	ErrSubmissionFileFieldUnsupported = errors.New(
		"file field is not supported by normal submission",
	)

	ErrSubmissionFileNotFound = errors.New(
		"submission file not found",
	)

	ErrSubmissionFileSubmissionUIDRequired = errors.New(
		"submission file submission uid is required",
	)

	ErrSubmissionFileFieldUIDRequired = errors.New(
		"submission file field uid is required",
	)

	ErrSubmissionFileNameRequired = errors.New(
		"submission file name is required",
	)

	ErrSubmissionFileStoragePathRequired = errors.New(
		"submission file storage path is required",
	)

	ErrSubmissionFileEmpty = errors.New(
		"uploaded file is empty",
	)

	ErrSubmissionFileTooLarge = errors.New(
		"uploaded file is too large",
	)

	ErrSubmissionFileTypeNotAllowed = errors.New(
		"uploaded file type is not allowed",
	)

	ErrSubmissionFileFieldNotFile = errors.New(
		"submission field is not a file field",
	)

	ErrSubmissionFileFieldTopicMismatch = errors.New(
		"submission file field does not belong to the submission topic",
	)

	ErrSubmissionFileAlreadyExists = errors.New(
		"a file already exists for this submission field",
	)
)

// FieldError keeps the original sentinel error while attaching enough field
// metadata for an HTTP client to highlight the exact dynamic-form control.
type FieldError struct {
	Err        error
	FieldUID   uuid.UUID
	FieldLabel string
}

func (e *FieldError) Error() string {
	if e == nil {
		return ""
	}

	if e.FieldLabel != "" {
		return fmt.Sprintf("%s: %s", e.Err.Error(), e.FieldLabel)
	}

	if e.FieldUID != uuid.Nil {
		return fmt.Sprintf("%s: %s", e.Err.Error(), e.FieldUID)
	}

	return e.Err.Error()
}

func (e *FieldError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func NewFieldError(err error, fieldUID uuid.UUID, fieldLabel string) error {
	return &FieldError{
		Err:        err,
		FieldUID:   fieldUID,
		FieldLabel: fieldLabel,
	}
}
