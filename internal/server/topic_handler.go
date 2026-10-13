package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/authorization"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type topicService interface {
	Create(context.Context, topic.CreateInput) (model.Topic, error)
	Get(context.Context, uuid.UUID) (model.Topic, error)
	Update(context.Context, uuid.UUID, topic.UpdateInput) (model.Topic, error)
	SetOpen(context.Context, uuid.UUID, bool) (model.Topic, error)
	Delete(context.Context, uuid.UUID) error
	ListDivisions(context.Context) ([]model.Division, error)
	ListDivisionTopics(context.Context, string, *int16, uuid.UUID) (topic.DivisionTopics, error)
}

type topicHandler struct {
	service topicService
}

type createTopicRequest struct {
	DivisionUUID uuid.UUID                 `json:"division_uuid"`
	Title        string                    `json:"title"`
	DueDate      string                    `json:"due_date"`
	IsSecret     bool                      `json:"is_secret"`
	Fields       []createTopicFieldRequest `json:"fields"`
}

type createTopicFieldRequest struct {
	Position int16    `json:"position"`
	Type     string   `json:"type"`
	Label    string   `json:"label"`
	Required *bool    `json:"required"`
	Options  []string `json:"options"`
}

type topicAcademicYearResponse struct {
	UUID uuid.UUID `json:"uuid"`
	Year int16     `json:"year"`
}

type topicFieldResponse struct {
	UUID     uuid.UUID `json:"uuid"`
	Position int16     `json:"position"`
	Type     string    `json:"type"`
	Label    string    `json:"label"`
	Required bool      `json:"required"`
	Options  []string  `json:"options"`
}

type topicResponse struct {
	UUID          uuid.UUID                 `json:"uuid"`
	Division      divisionResponse          `json:"division"`
	AcademicYear  topicAcademicYearResponse `json:"academic_year"`
	Semester      int16                     `json:"semester"`
	Title         string                    `json:"title"`
	DueDate       string                    `json:"due_date"`
	IsOpen        bool                      `json:"is_open"`
	IsSecret      bool                      `json:"is_secret"`
	CreatedByUUID uuid.UUID                 `json:"created_by_uuid"`
	Fields        []topicFieldResponse      `json:"fields"`
	AssigneeCount int                       `json:"assignee_count"`
	ClosedAt      *time.Time                `json:"closed_at"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

// create ตรวจสิทธิ์จาก profile ที่ middleware ยืนยันแล้ว โดยใช้ division_uuid
// จาก request กับ policy กลาง ก่อนส่งข้อมูลที่ผ่าน boundary ไปยัง service.
func (h topicHandler) create(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Topic service unavailable")
	}

	var request createTopicRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	if request.DivisionUUID == uuid.Nil {
		return mapTopicError(&admin.ValidationError{Field: "division_uuid", Message: "division UUID is required"})
	}

	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	if !authorization.CanManageTopic(profile, request.DivisionUUID) {
		return apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
	}

	fields := make([]topic.FieldInput, 0, len(request.Fields))
	for _, requestField := range request.Fields {
		required := true
		if requestField.Required != nil {
			required = *requestField.Required
		}
		fields = append(fields, topic.FieldInput{
			Position: requestField.Position,
			Type:     requestField.Type,
			Label:    requestField.Label,
			Required: required,
			Options:  requestField.Options,
		})
	}

	created, err := h.service.Create(c.Context(), topic.CreateInput{
		DivisionUUID:  request.DivisionUUID,
		CreatedByUUID: profile.User.UUID,
		Title:         request.Title,
		DueDate:       request.DueDate,
		IsSecret:      request.IsSecret,
		Fields:        fields,
	})
	if err != nil {
		return mapTopicError(err)
	}
	response, err := newTopicResponse(created)
	if err != nil {
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
	return c.Status(fiber.StatusCreated).JSON(response)
}

func mapTopicError(err error) error {
	var validationErr *admin.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return apperror.WrapWithDetails(
			fiber.StatusBadRequest,
			"VALIDATION_ERROR",
			validationErr.Message,
			map[string]any{"field": validationErr.Field},
			err,
		)
	case errors.Is(err, admin.ErrDivisionNotFound):
		return apperror.New(fiber.StatusNotFound, "DIVISION_NOT_FOUND", "Division not found")
	case errors.Is(err, topic.ErrCurrentAcademicYearNotSet):
		return apperror.New(fiber.StatusConflict, "CURRENT_ACADEMIC_YEAR_NOT_SET", "Current academic year is not set")
	case errors.Is(err, topic.ErrCreatorNotFound):
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	case errors.Is(err, topic.ErrTopicNotFound):
		return apperror.New(fiber.StatusNotFound, "TOPIC_NOT_FOUND", "Topic not found")
	case errors.Is(err, topic.ErrFieldTypeLocked):
		return apperror.New(fiber.StatusConflict, "TOPIC_FIELD_TYPE_LOCKED", "Field type cannot be changed after a submission")
	case errors.Is(err, topic.ErrTopicHasSubmissions):
		return apperror.New(fiber.StatusConflict, "TOPIC_HAS_SUBMISSIONS", "Topic with submissions cannot be deleted")
	case errors.Is(err, topic.ErrTopicAcademicYearNotFound):
		return apperror.New(fiber.StatusNotFound, "ACADEMIC_YEAR_NOT_FOUND", "Academic year not found")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newTopicResponse(value model.Topic) (topicResponse, error) {
	fields := make([]topicFieldResponse, 0, len(value.Fields))
	for _, field := range value.Fields {
		options := make([]string, 0)
		if len(field.Options) > 0 {
			if err := json.Unmarshal(field.Options, &options); err != nil {
				return topicResponse{}, err
			}
		}
		fields = append(fields, topicFieldResponse{
			UUID:     field.UUID,
			Position: field.Position,
			Type:     field.Type,
			Label:    field.Label,
			Required: field.Required,
			Options:  options,
		})
	}

	return topicResponse{
		UUID: value.UUID,
		Division: divisionResponse{
			UUID:      value.Division.UUID,
			Code:      value.Division.Code,
			Name:      value.Division.Name,
			ShortName: value.Division.ShortName,
		},
		AcademicYear: topicAcademicYearResponse{
			UUID: value.AcademicYear.UUID,
			Year: value.AcademicYear.Year,
		},
		Semester:      value.Semester,
		Title:         value.Title,
		DueDate:       value.DueDate.Format("2006-01-02"),
		IsOpen:        value.IsOpen,
		IsSecret:      value.IsSecret,
		CreatedByUUID: value.CreatedByUUID,
		Fields:        fields,
		AssigneeCount: len(value.Assignees),
		ClosedAt:      value.ClosedAt,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
	}, nil
}
