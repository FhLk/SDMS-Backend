package http

import (
	"errors"
	"sdms/internal/modules/topic/domain"
	"sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type Handler struct {
	service *usecase.Service
}

func NewHandler(service *usecase.Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Create(c fiber.Ctx) error {
	var req CreateTopicRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid request body",
		})
	}

	topic, err := h.service.Create(c.Context(), req.Name, req.Description)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(newTopicResponse(*topic))
}

func (h *Handler) FindAll(c fiber.Ctx) error {
	topics, err := h.service.FindAll(c.Context())
	if err != nil {
		return handleError(c, err)
	}

	response := make([]TopicResponse, len(topics))

	for _, topic := range topics {
		response = append(response, newTopicResponse(topic))
	}
	return c.JSON(response)
}

func (h *Handler) FindTopic(c fiber.Ctx) error {
	topicID, err := uuid.Parse(c.Params("id"))

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid topic id",
		})
	}

	topic, err := h.service.FindByID(c.Context(), topicID)
	if err != nil {
		return handleError(c, err)
	}

	response := newTopicResponse(*topic)
	return c.JSON(response)
}

func (h *Handler) Update(c fiber.Ctx) error {
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

func (h *Handler) Delete(c fiber.Ctx) error {
	topicID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid topic id",
		})
	}

	if err := h.service.Delete(c.Context(), topicID); err != nil {
		return handleError(c, err)
	}

	return nil
}

func handleError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrTopicNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": err.Error(),
		})

	case errors.Is(err, domain.ErrTopicNameEmpty):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err.Error(),
		})

	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "internal server error",
		})
	}
}
