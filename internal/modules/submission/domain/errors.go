package domain

import "errors"

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
)
