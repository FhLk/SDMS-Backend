package server

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/upload"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type uploadService interface {
	Upload(context.Context, upload.UploadInput) (model.File, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type uploadHandler struct {
	service uploadService
}

type uploadResponse struct {
	UUID         uuid.UUID `json:"uuid"`
	TopicUUID    uuid.UUID `json:"topic_uuid"`
	FieldUUID    uuid.UUID `json:"field_uuid"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// create รับไฟล์เดียวจาก multipart field ชื่อ file และคืน UUID สำหรับแนบใน BE-12.
func (h uploadHandler) create(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Upload service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	topicUUID, err := parseUploadUUID(c.FormValue("topic_uuid"), "topic_uuid")
	if err != nil {
		return err
	}
	fieldUUID, err := parseUploadUUID(c.FormValue("field_uuid"), "field_uuid")
	if err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "VALIDATION_ERROR", "file is required", err)
	}
	if fileHeader.Size > upload.MaxFileSize {
		return mapUploadError(upload.ErrFileTooLarge)
	}
	file, err := fileHeader.Open()
	if err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_FILE", "Uploaded file cannot be read", err)
	}
	defer file.Close()

	created, err := h.service.Upload(c.Context(), upload.UploadInput{
		TopicUUID:      topicUUID,
		FieldUUID:      fieldUUID,
		UploadedByUUID: profile.User.UUID,
		OriginalName:   fileHeader.Filename,
		MimeType:       multipartMimeType(fileHeader),
		SizeHint:       fileHeader.Size,
		Content:        file,
	})
	if err != nil {
		return mapUploadError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(newUploadResponse(created))
}

func (h uploadHandler) delete(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Upload service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	fileUUID, err := parseUploadUUID(c.Params("uuid"), "upload_uuid")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), fileUUID, profile.User.UUID); err != nil {
		return mapUploadError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func parseUploadUUID(value, field string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, apperror.WrapWithDetails(
			fiber.StatusBadRequest,
			"VALIDATION_ERROR",
			field+" must be a valid UUID",
			map[string]any{"field": field},
			nil,
		)
	}
	return parsed, nil
}

func multipartMimeType(file *multipart.FileHeader) string {
	if value := strings.TrimSpace(file.Header.Get(fiber.HeaderContentType)); value != "" {
		return value
	}
	return "application/octet-stream"
}

func mapUploadError(err error) error {
	switch {
	case errors.Is(err, upload.ErrFileTooLarge):
		return apperror.New(fiber.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "File must not exceed 50 MiB")
	case errors.Is(err, upload.ErrFileEmpty):
		return apperror.New(fiber.StatusBadRequest, "EMPTY_FILE", "File must not be empty")
	case errors.Is(err, upload.ErrInvalidUpload):
		return apperror.New(fiber.StatusBadRequest, "VALIDATION_ERROR", "Upload data is invalid")
	case errors.Is(err, upload.ErrTopicNotFound):
		return apperror.New(fiber.StatusNotFound, "TOPIC_NOT_FOUND", "Topic not found")
	case errors.Is(err, upload.ErrFieldNotFound):
		return apperror.New(fiber.StatusNotFound, "TOPIC_FIELD_NOT_FOUND", "Topic field not found")
	case errors.Is(err, upload.ErrFieldNotFile):
		return apperror.New(fiber.StatusBadRequest, "FIELD_NOT_FILE", "Topic field does not accept files")
	case errors.Is(err, upload.ErrTopicClosed):
		return apperror.New(fiber.StatusConflict, "TOPIC_CLOSED", "Topic is closed")
	case errors.Is(err, upload.ErrNotAssignee), errors.Is(err, upload.ErrFileForbidden):
		return apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
	case errors.Is(err, upload.ErrFileNotFound):
		return apperror.New(fiber.StatusNotFound, "UPLOAD_NOT_FOUND", "Upload not found")
	case errors.Is(err, upload.ErrFileAttached):
		return apperror.New(fiber.StatusConflict, "FILE_ALREADY_ATTACHED", "Attached file must be removed through submission editing")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newUploadResponse(file model.File) uploadResponse {
	return uploadResponse{
		UUID:         file.UUID,
		TopicUUID:    file.TopicUUID,
		FieldUUID:    file.FieldUUID,
		OriginalName: file.OriginalName,
		MimeType:     file.MimeType,
		SizeBytes:    file.SizeBytes,
		Status:       file.Status,
		CreatedAt:    file.CreatedAt,
	}
}
