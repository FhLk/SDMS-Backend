package server

import (
	"net/http"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/authorization"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type updateTopicRequest struct {
	Title    *string                    `json:"title"`
	DueDate  *string                    `json:"due_date"`
	IsSecret *bool                      `json:"is_secret"`
	Fields   *[]updateTopicFieldRequest `json:"fields"`
}

type updateTopicFieldRequest struct {
	UUID     uuid.UUID `json:"uuid"`
	Position int16     `json:"position"`
	Type     string    `json:"type"`
	Label    string    `json:"label"`
	Required *bool     `json:"required"`
	Options  []string  `json:"options"`
}

// update รับเฉพาะค่าที่ต้องแก้ในระดับหัวข้อ แต่ถ้าส่ง fields มา ต้องส่ง
// แบบฟอร์มฉบับเต็ม เพื่อให้ service รู้ว่าช่องใดถูกเพิ่ม แก้ หรือลบ.
func (h topicHandler) update(c fiber.Ctx) error {
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	if _, err := h.authorizedTopic(c, topicUUID); err != nil {
		return err
	}

	var request updateTopicRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}

	var fields *[]topic.UpdateFieldInput
	if request.Fields != nil {
		converted := make([]topic.UpdateFieldInput, 0, len(*request.Fields))
		for _, field := range *request.Fields {
			converted = append(converted, topic.UpdateFieldInput{
				UUID:     field.UUID,
				Position: field.Position,
				Type:     field.Type,
				Label:    field.Label,
				Required: field.Required,
				Options:  field.Options,
			})
		}
		fields = &converted
	}

	updated, err := h.service.Update(c.Context(), topicUUID, topic.UpdateInput{
		Title:    request.Title,
		DueDate:  request.DueDate,
		IsSecret: request.IsSecret,
		Fields:   fields,
	})
	if err != nil {
		return mapTopicError(err)
	}
	response, err := newTopicResponse(updated)
	if err != nil {
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
	return c.JSON(response)
}

func (h topicHandler) close(c fiber.Ctx) error {
	return h.changeOpenState(c, false)
}

func (h topicHandler) reopen(c fiber.Ctx) error {
	return h.changeOpenState(c, true)
}

func (h topicHandler) changeOpenState(c fiber.Ctx, open bool) error {
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	if _, err := h.authorizedTopic(c, topicUUID); err != nil {
		return err
	}
	updated, err := h.service.SetOpen(c.Context(), topicUUID, open)
	if err != nil {
		return mapTopicError(err)
	}
	response, err := newTopicResponse(updated)
	if err != nil {
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
	return c.JSON(response)
}

func (h topicHandler) delete(c fiber.Ctx) error {
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	if _, err := h.authorizedTopic(c, topicUUID); err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), topicUUID); err != nil {
		return mapTopicError(err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// authorizedTopic โหลดข้อมูลเจ้าของหัวข้อก่อนใช้ policy กลาง เพื่อรองรับสิทธิ์
// ของแอดมิน หัวหน้าฝ่าย และคนสร้างหัวข้อด้วยกฎชุดเดียวกันทุก endpoint.
func (h topicHandler) authorizedTopic(c fiber.Ctx, topicUUID uuid.UUID) (model.Topic, error) {
	if h.service == nil {
		return model.Topic{}, apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Topic service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return model.Topic{}, apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	current, err := h.service.Get(c.Context(), topicUUID)
	if err != nil {
		return model.Topic{}, mapTopicError(err)
	}
	if !authorization.CanEditTopic(profile, current) {
		return model.Topic{}, apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
	}
	return current, nil
}

func parseTopicUUID(c fiber.Ctx) (uuid.UUID, error) {
	topicUUID, err := uuid.Parse(c.Params("uuid"))
	if err != nil {
		return uuid.Nil, mapTopicError(&admin.ValidationError{Field: "topic_uuid", Message: "topic UUID is invalid"})
	}
	return topicUUID, nil
}
