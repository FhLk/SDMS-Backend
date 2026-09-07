package http

import (
	"context"
	"errors"

	submissiondomain "sdms/internal/modules/submission/domain"
	"sdms/internal/modules/submission/usecase"
	topicdomain "sdms/internal/modules/topic/domain"
	userdomain "sdms/internal/modules/user/domain"
	platformmiddleware "sdms/internal/platform/http/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type SubmissionService interface {
	Create(ctx context.Context, topicUID uuid.UUID, input usecase.CreateSubmissionInput) (*submissiondomain.Submission, error)
	UpdateForSubmitter(ctx context.Context, topicUID, submissionUID, submittedBy uuid.UUID, input usecase.UpdateSubmissionInput) (*submissiondomain.Submission, error)
	DeleteForSubmitter(ctx context.Context, topicUID, submissionUID, submittedBy uuid.UUID) error
	FindAllByTopicID(ctx context.Context, topicUID uuid.UUID) ([]submissiondomain.Submission, error)
	FindAllByTopicIDAndSubmittedBy(ctx context.Context, topicUID, submittedBy uuid.UUID) ([]submissiondomain.Submission, error)
	FindByID(ctx context.Context, topicUID, submissionUID uuid.UUID) (*submissiondomain.Submission, error)
	FindByIDForSubmitter(ctx context.Context, topicUID, submissionUID, submittedBy uuid.UUID) (*submissiondomain.Submission, error)
	GetTopicSubmissionStatus(ctx context.Context, topicUID uuid.UUID) ([]usecase.TopicSubmissionStatus, error)
}

type SubmissionFileCleanupService interface {
	DeleteAll(ctx context.Context, topicUID, submissionUID uuid.UUID) error
}

type SubmissionHandler struct {
	service SubmissionService
	files   SubmissionFileCleanupService
}

func NewSubmissionHandler(service SubmissionService, fileServices ...SubmissionFileCleanupService) *SubmissionHandler {
	var files SubmissionFileCleanupService
	if len(fileServices) > 0 {
		files = fileServices[0]
	}
	return &SubmissionHandler{service: service, files: files}
}

func (h *SubmissionHandler) Create(c fiber.Ctx) error {
	topicUID, err := parseTopicUID(c)
	if err != nil {
		return err
	}
	var req CreateSubmissionRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid request body"})
	}
	currentUser, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}
	if currentUser.Role != userdomain.RoleTeacher {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
	}
	submission, err := h.service.Create(c.Context(), topicUID, usecase.CreateSubmissionInput{
		SubmittedBy: currentUser.UID,
		Values:      requestValues(req.Values),
	})
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(newSubmissionResponse(*submission))
}

func (h *SubmissionHandler) Update(c fiber.Ctx) error {
	topicUID, submissionUID, err := parseTopicAndSubmissionUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	currentUser, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}
	if currentUser.Role != userdomain.RoleTeacher {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
	}
	var req UpdateSubmissionRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid request body"})
	}
	submission, err := h.service.UpdateForSubmitter(c.Context(), topicUID, submissionUID, currentUser.UID, usecase.UpdateSubmissionInput{
		Values: requestValues(req.Values),
	})
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(newSubmissionResponse(*submission))
}

func (h *SubmissionHandler) Delete(c fiber.Ctx) error {
	topicUID, submissionUID, err := parseTopicAndSubmissionUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	currentUser, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}
	if currentUser.Role != userdomain.RoleTeacher {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
	}
	// Ownership is checked before any physical file is removed.
	if _, err := h.service.FindByIDForSubmitter(c.Context(), topicUID, submissionUID, currentUser.UID); err != nil {
		return handleError(c, err)
	}
	if h.files != nil {
		if err := h.files.DeleteAll(c.Context(), topicUID, submissionUID); err != nil {
			return handleError(c, err)
		}
	}
	if err := h.service.DeleteForSubmitter(c.Context(), topicUID, submissionUID, currentUser.UID); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *SubmissionHandler) FindAll(c fiber.Ctx) error {
	topicUID, err := parseTopicUID(c)
	if err != nil {
		return err
	}
	currentUser, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}
	var submissions []submissiondomain.Submission
	switch currentUser.Role {
	case userdomain.RoleTeacher:
		submissions, err = h.service.FindAllByTopicIDAndSubmittedBy(c.Context(), topicUID, currentUser.UID)
	case userdomain.RoleAdmin, userdomain.RoleDirector, userdomain.RoleQA:
		if submittedByParam := c.Query("submitted_by"); submittedByParam != "" {
			submittedBy, parseErr := uuid.Parse(submittedByParam)
			if parseErr != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid submitted_by"})
			}
			submissions, err = h.service.FindAllByTopicIDAndSubmittedBy(c.Context(), topicUID, submittedBy)
		} else {
			submissions, err = h.service.FindAllByTopicID(c.Context(), topicUID)
		}
	default:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
	}
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(newSubmissionListResponse(submissions))
}

func (h *SubmissionHandler) FindByID(c fiber.Ctx) error {
	topicUID, submissionUID, err := parseTopicAndSubmissionUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}
	currentUser, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}
	var submission *submissiondomain.Submission
	switch currentUser.Role {
	case userdomain.RoleTeacher:
		submission, err = h.service.FindByIDForSubmitter(c.Context(), topicUID, submissionUID, currentUser.UID)
	case userdomain.RoleAdmin, userdomain.RoleDirector, userdomain.RoleQA:
		submission, err = h.service.FindByID(c.Context(), topicUID, submissionUID)
	default:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
	}
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(newSubmissionResponse(*submission))
}

func (h *SubmissionHandler) SubmissionStatus(c fiber.Ctx) error {
	topicUID, err := parseTopicUID(c)
	if err != nil {
		return err
	}
	items, err := h.service.GetTopicSubmissionStatus(c.Context(), topicUID)
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(newTopicSubmissionStatusResponse(items))
}

func requestValues(values []SubmissionValueRequest) []usecase.SubmissionValueInput {
	result := make([]usecase.SubmissionValueInput, 0, len(values))
	for _, value := range values {
		result = append(result, usecase.SubmissionValueInput{FieldUID: value.FieldUID, Value: value.Value})
	}
	return result
}

func parseTopicUID(c fiber.Ctx) (uuid.UUID, error) {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid topic id"})
	}
	return topicUID, nil
}

func handleError(c fiber.Ctx, err error) error {
	var fieldErr *submissiondomain.FieldError
	if errors.As(err, &fieldErr) {
		status := fiber.StatusBadRequest
		if errors.Is(fieldErr.Err, submissiondomain.ErrSubmissionFileAlreadyExists) {
			status = fiber.StatusConflict
		}
		return c.Status(status).JSON(fiber.Map{
			"code": submissionErrorCode(fieldErr.Err), "message": fieldErr.Error(),
			"field_uid": fieldErr.FieldUID, "field_label": fieldErr.FieldLabel,
		})
	}

	switch {
	case errors.Is(err, errAuthenticationRequired):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	case errors.Is(err, errForbidden):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
	case errors.Is(err, topicdomain.ErrTopicNotFound), errors.Is(err, topicdomain.ErrTopicFieldNotFound),
		errors.Is(err, submissiondomain.ErrSubmissionNotFound), errors.Is(err, submissiondomain.ErrSubmissionFileNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": err.Error()})
	case errors.Is(err, submissiondomain.ErrSubmissionFileTooLarge):
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"code": submissionErrorCode(err), "message": err.Error()})
	case errors.Is(err, submissiondomain.ErrSubmissionFileTypeNotAllowed):
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"code": submissionErrorCode(err), "message": err.Error()})
	case errors.Is(err, submissiondomain.ErrSubmissionTopicUIDRequired),
		errors.Is(err, submissiondomain.ErrSubmissionSubmittedByRequired),
		errors.Is(err, submissiondomain.ErrSubmissionSubmitterNotFound),
		errors.Is(err, submissiondomain.ErrSubmissionSubmitterMustBeTeacher),
		errors.Is(err, submissiondomain.ErrSubmissionSubmitterInactive),
		errors.Is(err, submissiondomain.ErrSubmissionTopicInactive),
		errors.Is(err, submissiondomain.ErrSubmissionInvalidField),
		errors.Is(err, submissiondomain.ErrSubmissionDuplicateField),
		errors.Is(err, submissiondomain.ErrSubmissionRequiredFieldMissing),
		errors.Is(err, submissiondomain.ErrSubmissionInvalidValue),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldUnsupported),
		errors.Is(err, submissiondomain.ErrSubmissionFileSubmissionUIDRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldUIDRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileNameRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileStoragePathRequired),
		errors.Is(err, submissiondomain.ErrSubmissionFileEmpty),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldNotFile),
		errors.Is(err, submissiondomain.ErrSubmissionFileFieldTopicMismatch):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"code": submissionErrorCode(err), "message": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "internal server error"})
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
	default:
		return "SUBMISSION_ERROR"
	}
}
