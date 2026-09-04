package http

import (
	"context"
	"errors"
	"sdms/internal/modules/topic/domain"
	"sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type TopicService interface {
	CreateTopic(
		ctx context.Context,
		name string,
		description string,
	) (*domain.Topic, error)

	FindAll(
		ctx context.Context,
	) ([]domain.Topic, error)

	FindTopicWithFields(
		ctx context.Context,
		id uuid.UUID,
	) (*domain.Topic, []domain.TopicField, error)

	Update(
		ctx context.Context,
		id uuid.UUID,
		name string,
		description string,
		isActive bool,
	) (*domain.Topic, error)

	Delete(
		ctx context.Context,
		id uuid.UUID,
	) error

	CreateField(
		ctx context.Context,
		topicUID uuid.UUID,
		input usecase.CreateFieldInput,
	) (*domain.TopicField, error)

	FindFieldsByTopicID(
		ctx context.Context,
		topicUID uuid.UUID,
	) ([]domain.TopicField, error)

	FindFieldByID(
		ctx context.Context,
		topicUID uuid.UUID,
		fieldUID uuid.UUID,
	) (*domain.TopicField, error)

	UpdateField(
		ctx context.Context,
		topicUID uuid.UUID,
		fieldUID uuid.UUID,
		input usecase.UpdateFieldInput,
	) (*domain.TopicField, error)

	DeleteField(
		ctx context.Context,
		topicUID uuid.UUID,
		fieldUID uuid.UUID,
	) error
}

type TopicHandler struct {
	service TopicService
}

func NewTopicHandler(service TopicService) *TopicHandler {
	return &TopicHandler{
		service: service,
	}
}

func (h *TopicHandler) Create(c fiber.Ctx) error {
	var req CreateTopicRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid request body",
		})
	}

	topic, err := h.service.CreateTopic(c.Context(), req.Name, req.Description)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(newTopicResponse(*topic))
}

func (h *TopicHandler) FindAll(c fiber.Ctx) error {
	topics, err := h.service.FindAll(c.Context())
	if err != nil {
		return handleError(c, err)
	}

	response := make([]TopicResponse, len(topics))

	for i, topic := range topics {
		response[i] = newTopicResponse(topic)
	}
	return c.JSON(response)
}

func (h *TopicHandler) FindTopic(c fiber.Ctx) error {
	topicID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid topic id",
		})
	}

	topic, fields, err := h.service.FindTopicWithFields(
		c.Context(),
		topicID,
	)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newTopicDetailResponse(*topic, fields),
	)
}

func (h *TopicHandler) Update(c fiber.Ctx) error {
	topicID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid topic id",
		})
	}

	var req UpdateTopicRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid request body",
		})
	}

	topic, err := h.service.Update(c.Context(), topicID, req.Name, req.Description, req.IsActive)

	if err != nil {
		return handleError(c, err)
	}

	response := newTopicResponse(*topic)

	return c.JSON(response)
}

func (h *TopicHandler) Delete(c fiber.Ctx) error {
	topicID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid topic id",
		})
	}

	if err := h.service.Delete(c.Context(), topicID); err != nil {
		return handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *TopicHandler) CreateField(c fiber.Ctx) error {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid topic id",
		})
	}

	var req CreateFieldRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	field, err := h.service.CreateField(
		c,
		topicUID,
		usecase.CreateFieldInput{
			Label:    req.Label,
			Type:     domain.FieldType(req.Type),
			Required: req.Required,
			Position: req.Position,
		},
	)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(
		newTopicFieldResponse(field),
	)
}

func (h *TopicHandler) FindFields(c fiber.Ctx) error {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid topic id",
		})
	}

	fields, err := h.service.FindFieldsByTopicID(
		c.Context(),
		topicUID,
	)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newTopicFieldListResponse(fields),
	)
}

func (h *TopicHandler) FindField(c fiber.Ctx) error {
	topicUID, fieldUID, err := parseTopicAndFieldIDs(c)
	if err != nil {
		return handleTopicFieldParamError(c, err)
	}

	field, err := h.service.FindFieldByID(c.Context(), topicUID, fieldUID)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newTopicFieldResponse(field),
	)
}

func (h *TopicHandler) UpdateField(c fiber.Ctx) error {
	topicUID, fieldUID, err := parseTopicAndFieldIDs(c)
	if err != nil {
		return handleTopicFieldParamError(c, err)
	}

	var req UpdateFieldRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	field, err := h.service.UpdateField(
		c.Context(),
		topicUID,
		fieldUID,
		usecase.UpdateFieldInput{
			Label:    req.Label,
			Type:     domain.FieldType(req.Type),
			Required: req.Required,
			Position: req.Position,
		},
	)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(
		newTopicFieldResponse(field),
	)
}

func (h *TopicHandler) DeleteField(c fiber.Ctx) error {
	topicUID, fieldUID, err := parseTopicAndFieldIDs(c)
	if err != nil {
		return handleTopicFieldParamError(c, err)
	}

	if err := h.service.DeleteField(
		c.Context(),
		topicUID,
		fieldUID,
	); err != nil {
		return handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

var (
	errInvalidTopicID = errors.New("invalid topic id")
	errInvalidFieldID = errors.New("invalid field id")
)

func parseTopicAndFieldIDs(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, errInvalidTopicID
	}

	fieldUID, err := uuid.Parse(c.Params("fieldID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, errInvalidFieldID
	}

	return topicUID, fieldUID, nil
}

func handleTopicFieldParamError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, errInvalidTopicID):
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "invalid topic id"},
		)

	case errors.Is(err, errInvalidFieldID):
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "invalid field id"},
		)

	default:
		return c.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "invalid request"},
		)
	}
}

func handleError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrTopicNotFound),
		errors.Is(err, domain.ErrTopicFieldNotFound):

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": err.Error(),
		})

	case errors.Is(err, domain.ErrTopicNameEmpty):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err.Error(),
		})
	case errors.Is(err, domain.ErrTopicFieldInvalidTopicUID),
		errors.Is(err, domain.ErrTopicFieldLabelRequired),
		errors.Is(err, domain.ErrTopicFieldInvalidType),
		errors.Is(err, domain.ErrTopicFieldInvalidPosition):

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})

	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "internal server error",
		})
	}
}
