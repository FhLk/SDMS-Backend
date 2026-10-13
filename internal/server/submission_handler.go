package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/submission"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type submissionService interface {
	ListPending(context.Context, uuid.UUID) ([]submission.PendingItem, error)
	GetMine(context.Context, uuid.UUID, uuid.UUID) (submission.Detail, error)
	Upsert(context.Context, submission.UpsertInput) (submission.Detail, error)
	ListTopicTable(context.Context, uuid.UUID, auth.Profile, string) (submission.TopicTable, error)
	ListPendingNames(context.Context, uuid.UUID, auth.Profile) ([]model.User, error)
}

type submissionHandler struct {
	service submissionService
}

type upsertSubmissionRequest struct {
	Values map[string]json.RawMessage `json:"values"`
}

type pendingSubmissionResponse struct {
	UUID          uuid.UUID                 `json:"uuid"`
	Division      divisionResponse          `json:"division"`
	AcademicYear  topicAcademicYearResponse `json:"academic_year"`
	Semester      int16                     `json:"semester"`
	Title         string                    `json:"title"`
	DueDate       string                    `json:"due_date"`
	IsSecret      bool                      `json:"is_secret"`
	Status        string                    `json:"status"`
	CreatedByUUID uuid.UUID                 `json:"created_by_uuid"`
}

type submissionFileResponse struct {
	UUID         uuid.UUID `json:"uuid"`
	FieldUUID    uuid.UUID `json:"field_uuid"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Position     int16     `json:"position"`
}

type mySubmissionItemResponse struct {
	UUID        uuid.UUID                `json:"uuid"`
	Values      json.RawMessage          `json:"values"`
	Files       []submissionFileResponse `json:"files"`
	Status      string                   `json:"status"`
	SubmittedAt time.Time                `json:"submitted_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type mySubmissionResponse struct {
	Topic      topicResponse             `json:"topic"`
	Submission *mySubmissionItemResponse `json:"submission"`
}

// listPending ใช้ UUID จาก session เท่านั้นเพื่อไม่ให้ผู้ใช้ขอดูงานค้างของคนอื่น.
func (h submissionHandler) listPending(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Submission service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	items, err := h.service.ListPending(c.Context(), profile.User.UUID)
	if err != nil {
		return mapSubmissionError(err)
	}
	response := make([]pendingSubmissionResponse, 0, len(items))
	for _, item := range items {
		response = append(response, newPendingSubmissionResponse(item))
	}
	return c.JSON(response)
}

// getMine คืนแบบฟอร์มพร้อมคำตอบเดิม; submission เป็น null เมื่อผู้ใช้ยังไม่เคยส่ง.
func (h submissionHandler) getMine(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Submission service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	detail, err := h.service.GetMine(c.Context(), topicUUID, profile.User.UUID)
	if err != nil {
		return mapSubmissionError(err)
	}
	response, err := newMySubmissionResponse(detail)
	if err != nil {
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
	return c.JSON(response)
}

// upsert สร้างหรือแก้คำตอบเดิมจาก values object และคืน snapshot หลัง commit.
func (h submissionHandler) upsert(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Submission service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	var request upsertSubmissionRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	detail, err := h.service.Upsert(c.Context(), submission.UpsertInput{
		TopicUUID: topicUUID,
		UserUUID:  profile.User.UUID,
		Values:    request.Values,
	})
	if err != nil {
		return mapSubmissionError(err)
	}
	response, err := newMySubmissionResponse(detail)
	if err != nil {
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
	return c.JSON(response)
}

func mapSubmissionError(err error) error {
	var validationErr *submission.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return apperror.WrapWithDetails(
			fiber.StatusBadRequest,
			"VALIDATION_ERROR",
			"Submission contains invalid fields",
			map[string]any{"fields": validationErr.Fields},
			err,
		)
	case errors.Is(err, submission.ErrTopicNotFound):
		return apperror.New(fiber.StatusNotFound, "TOPIC_NOT_FOUND", "Topic not found")
	case errors.Is(err, submission.ErrTopicClosed):
		return apperror.New(fiber.StatusConflict, "TOPIC_CLOSED", "Topic is closed")
	case errors.Is(err, submission.ErrNotAssignee):
		return apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You are not assigned to this topic")
	case errors.Is(err, submission.ErrForbidden):
		return apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newPendingSubmissionResponse(item submission.PendingItem) pendingSubmissionResponse {
	return pendingSubmissionResponse{
		UUID: item.Topic.UUID,
		Division: divisionResponse{
			UUID:      item.Topic.Division.UUID,
			Code:      item.Topic.Division.Code,
			Name:      item.Topic.Division.Name,
			ShortName: item.Topic.Division.ShortName,
		},
		AcademicYear: topicAcademicYearResponse{
			UUID: item.Topic.AcademicYear.UUID,
			Year: item.Topic.AcademicYear.Year,
		},
		Semester:      item.Topic.Semester,
		Title:         item.Topic.Title,
		DueDate:       item.Topic.DueDate.Format("2006-01-02"),
		IsSecret:      item.Topic.IsSecret,
		Status:        item.Status,
		CreatedByUUID: item.Topic.CreatedByUUID,
	}
}

func newMySubmissionResponse(detail submission.Detail) (mySubmissionResponse, error) {
	topicValue, err := newTopicResponse(detail.Topic)
	if err != nil {
		return mySubmissionResponse{}, err
	}
	response := mySubmissionResponse{Topic: topicValue}
	if detail.Submission == nil {
		return response, nil
	}
	files := make([]submissionFileResponse, 0, len(detail.Files))
	for _, file := range detail.Files {
		files = append(files, newSubmissionFileResponse(file))
	}
	status := ""
	if detail.Status != nil {
		status = *detail.Status
	}
	response.Submission = &mySubmissionItemResponse{
		UUID:        detail.Submission.UUID,
		Values:      json.RawMessage(detail.Submission.Values),
		Files:       files,
		Status:      status,
		SubmittedAt: detail.Submission.SubmittedAt,
		UpdatedAt:   detail.Submission.UpdatedAt,
	}
	return response, nil
}
