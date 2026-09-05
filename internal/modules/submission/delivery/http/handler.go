package http

import (
	"context"
	"errors"

	submissiondomain "sdms/internal/modules/submission/domain"
	"sdms/internal/modules/submission/usecase"
	topicdomain "sdms/internal/modules/topic/domain"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type SubmissionService interface {
	Create(
		ctx context.Context,
		topicUID uuid.UUID,
		input usecase.CreateSubmissionInput,
	) (*submissiondomain.Submission, error)

	FindAllByTopicID(
		ctx context.Context,
		topicUID uuid.UUID,
	) ([]submissiondomain.Submission, error)

	FindAllByTopicIDAndSubmittedBy(
		ctx context.Context,
		topicUID uuid.UUID,
		submittedBy uuid.UUID,
	) ([]submissiondomain.Submission, error)

	FindByID(
		ctx context.Context,
		topicUID uuid.UUID,
		submissionUID uuid.UUID,
	) (*submissiondomain.Submission, error)

	FindByIDForSubmitter(
		ctx context.Context,
		topicUID uuid.UUID,
		submissionUID uuid.UUID,
		submittedBy uuid.UUID,
	) (*submissiondomain.Submission, error)
}

type SubmissionHandler struct {
	service SubmissionService
}

func NewSubmissionHandler(
	service SubmissionService,
) *SubmissionHandler {
	return &SubmissionHandler{
		service: service,
	}
}

func (h *SubmissionHandler) Create(c fiber.Ctx) error {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"message": "invalid topic id",
			},
		)
	}

	var req CreateSubmissionRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"message": "invalid request body",
			},
		)
	}

	values := make(
		[]usecase.SubmissionValueInput,
		0,
		len(req.Values),
	)

	for _, value := range req.Values {
		values = append(
			values,
			usecase.SubmissionValueInput{
				FieldUID: value.FieldUID,
				Value:    value.Value,
			},
		)
	}

	submission, err := h.service.Create(
		c.Context(),
		topicUID,
		usecase.CreateSubmissionInput{
			SubmittedBy: req.SubmittedBy,
			Values:      values,
		},
	)

	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(
		newSubmissionResponse(*submission),
	)
}

func (h *SubmissionHandler) FindAll(c fiber.Ctx) error {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"message": "invalid topic id",
			},
		)
	}

	var submissions []submissiondomain.Submission

	// Temporary ownership filter until authentication can provide the current user.
	submittedByParam := c.Query("submitted_by")
	if submittedByParam == "" {
		submissions, err = h.service.FindAllByTopicID(
			c.Context(),
			topicUID,
		)
	} else {
		submittedBy, parseErr := uuid.Parse(submittedByParam)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				fiber.Map{
					"message": "invalid submitted_by",
				},
			)
		}

		submissions, err = h.service.FindAllByTopicIDAndSubmittedBy(
			c.Context(),
			topicUID,
			submittedBy,
		)
	}

	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newSubmissionListResponse(submissions),
	)
}

func (h *SubmissionHandler) FindByID(c fiber.Ctx) error {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"message": "invalid topic id",
			},
		)
	}

	submissionUID, err := uuid.Parse(
		c.Params("submissionID"),
	)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"message": "invalid submission id",
			},
		)
	}

	var submission *submissiondomain.Submission

	// Temporary ownership check until authentication can provide the current user.
	submittedByParam := c.Query("submitted_by")
	if submittedByParam == "" {
		submission, err = h.service.FindByID(
			c.Context(),
			topicUID,
			submissionUID,
		)
	} else {
		submittedBy, parseErr := uuid.Parse(submittedByParam)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				fiber.Map{
					"message": "invalid submitted_by",
				},
			)
		}

		submission, err = h.service.FindByIDForSubmitter(
			c.Context(),
			topicUID,
			submissionUID,
			submittedBy,
		)
	}
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newSubmissionResponse(*submission),
	)
}

func handleError(c fiber.Ctx, err error) error {
	var fieldErr *submissiondomain.FieldError
	if errors.As(err, &fieldErr) {
		status := fiber.StatusBadRequest
		if errors.Is(fieldErr.Err, submissiondomain.ErrSubmissionFileAlreadyExists) {
			status = fiber.StatusConflict
		}
		return c.Status(status).JSON(fiber.Map{
			"code":        submissionErrorCode(fieldErr.Err),
			"message":     fieldErr.Error(),
			"field_uid":   fieldErr.FieldUID,
			"field_label": fieldErr.FieldLabel,
		})
	}

	switch {
	case errors.Is(err, topicdomain.ErrTopicNotFound),
		errors.Is(err, topicdomain.ErrTopicFieldNotFound),
		errors.Is(err, submissiondomain.ErrSubmissionNotFound),
		errors.Is(err, submissiondomain.ErrSubmissionFileNotFound):

		return c.Status(fiber.StatusNotFound).JSON(
			fiber.Map{
				"message": err.Error(),
			},
		)

	case errors.Is(err, submissiondomain.ErrSubmissionFileTooLarge):
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"code":    submissionErrorCode(err),
			"message": err.Error(),
		})

	case errors.Is(err, submissiondomain.ErrSubmissionFileTypeNotAllowed):
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{
			"code":    submissionErrorCode(err),
			"message": err.Error(),
		})

	case errors.Is(
		err,
		submissiondomain.ErrSubmissionTopicUIDRequired,
	),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionSubmittedByRequired,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionSubmitterNotFound,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionSubmitterMustBeTeacher,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionSubmitterInactive,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionTopicInactive,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionInvalidField,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionDuplicateField,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionRequiredFieldMissing,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionInvalidValue,
		),
		errors.Is(
			err,
			submissiondomain.ErrSubmissionFileFieldUnsupported,
		),
		errors.Is(err, submissiondomain.ErrSubmissionFileSubmissionUIDRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldUIDRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileNameRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileStoragePathRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileEmpty),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldNotFile),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldTopicMismatch):

		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"code":    submissionErrorCode(err),
				"message": err.Error(),
			},
		)

	default:
		return c.Status(
			fiber.StatusInternalServerError,
		).JSON(
			fiber.Map{
				"message": "internal server error",
			},
		)
	}
}

func submissionErrorCode(err error) string {
	switch {
	case errors.Is(err, submissiondomain.ErrSubmissionRequiredFieldMissing):
		return "REQUIRED_FIELD_MISSING"
	case errors.Is(err, submissiondomain.ErrSubmissionInvalidValue):
		return "INVALID_SUBMISSION_VALUE"
	case errors.Is(err, submissiondomain.ErrSubmissionInvalidField):
		return "INVALID_SUBMISSION_FIELD"
	case errors.Is(err, submissiondomain.ErrSubmissionDuplicateField):
		return "DUPLICATE_SUBMISSION_FIELD"
	case errors.Is(err, submissiondomain.ErrSubmissionFileFieldUnsupported):
		return "FILE_FIELD_UNSUPPORTED"
	case errors.Is(err, submissiondomain.ErrSubmissionFileTooLarge):
		return "FILE_TOO_LARGE"
	case errors.Is(err, submissiondomain.ErrSubmissionFileTypeNotAllowed):
		return "FILE_TYPE_NOT_ALLOWED"
	case errors.Is(err, submissiondomain.ErrSubmissionFileAlreadyExists):
		return "FILE_ALREADY_EXISTS"
	case errors.Is(err, submissiondomain.ErrSubmissionFileFieldNotFile):
		return "FIELD_NOT_FILE"
	case errors.Is(err, submissiondomain.ErrSubmissionFileFieldTopicMismatch):
		return "FILE_FIELD_TOPIC_MISMATCH"
	case errors.Is(err, submissiondomain.ErrSubmissionFileEmpty):
		return "FILE_EMPTY"
	case errors.Is(err, submissiondomain.ErrSubmissionTopicInactive):
		return "TOPIC_INACTIVE"
	case errors.Is(err, submissiondomain.ErrSubmissionSubmitterNotFound):
		return "SUBMITTER_NOT_FOUND"
	case errors.Is(err, submissiondomain.ErrSubmissionSubmitterMustBeTeacher):
		return "SUBMITTER_MUST_BE_TEACHER"
	case errors.Is(err, submissiondomain.ErrSubmissionSubmitterInactive):
		return "SUBMITTER_INACTIVE"
	case errors.Is(err, submissiondomain.ErrSubmissionSubmittedByRequired):
		return "SUBMITTED_BY_REQUIRED"
	case errors.Is(err, submissiondomain.ErrSubmissionTopicUIDRequired):
		return "TOPIC_UID_REQUIRED"
	default:
		return "INVALID_SUBMISSION"
	}
}
