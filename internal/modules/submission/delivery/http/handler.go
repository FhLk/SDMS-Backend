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

	FindByID(
		ctx context.Context,
		topicUID uuid.UUID,
		submissionUID uuid.UUID,
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

	submissions, err := h.service.FindAllByTopicID(
		c.Context(),
		topicUID,
	)

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

	submission, err := h.service.FindByID(
		c.Context(),
		topicUID,
		submissionUID,
	)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newSubmissionResponse(*submission),
	)
}

func handleError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, topicdomain.ErrTopicNotFound),
		errors.Is(err, submissiondomain.ErrSubmissionNotFound):

		return c.Status(fiber.StatusNotFound).JSON(
			fiber.Map{
				"message": err.Error(),
			},
		)

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
		):

		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
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
