package http

import (
	"errors"
	"strconv"

	"github.com/FhLk/SDMS-Backend/internal/domain"
	"github.com/FhLk/SDMS-Backend/internal/usecase"
	"github.com/gofiber/fiber/v3"
)

type UserHandler struct {
	usecase *usecase.UserUsecase
}

func NewUserHandler(userUsecase *usecase.UserUsecase) *UserHandler {
	return &UserHandler{usecase: userUsecase}
}

func (h *UserHandler) Create(c fiber.Ctx) error {
	var input usecase.CreateUserInput
	if err := c.Bind().Body(&input); err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_BODY", "request body is invalid")
	}

	user, err := h.usecase.Create(c.Context(), input)
	if err != nil {
		return handleUsecaseError(c, err)
	}
	return success(c, fiber.StatusCreated, user, nil)
}

func (h *UserHandler) GetByID(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_ID", "id must be a positive integer")
	}

	user, err := h.usecase.GetByID(c.Context(), id)
	if err != nil {
		return handleUsecaseError(c, err)
	}
	return success(c, fiber.StatusOK, user, nil)
}

func (h *UserHandler) List(c fiber.Ctx) error {
	page, err := parseOptionalPositiveInt(c.Query("page"), 1)
	if err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_PAGE", "page must be a positive integer")
	}
	pageSize, err := parseOptionalPositiveInt(c.Query("page_size"), 20)
	if err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_PAGE_SIZE", "page_size must be a positive integer")
	}

	users, total, page, pageSize, err := h.usecase.List(c.Context(), page, pageSize)
	if err != nil {
		return handleUsecaseError(c, err)
	}
	return success(c, fiber.StatusOK, users, fiber.Map{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

func (h *UserHandler) Update(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_ID", "id must be a positive integer")
	}

	var input usecase.UpdateUserInput
	if err := c.Bind().Body(&input); err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_BODY", "request body is invalid")
	}

	user, err := h.usecase.Update(c.Context(), id, input)
	if err != nil {
		return handleUsecaseError(c, err)
	}
	return success(c, fiber.StatusOK, user, nil)
}

func (h *UserHandler) Delete(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return failure(c, fiber.StatusBadRequest, "INVALID_ID", "id must be a positive integer")
	}

	if err := h.usecase.Delete(c.Context(), id); err != nil {
		return handleUsecaseError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func handleUsecaseError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return failure(c, fiber.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return failure(c, fiber.StatusNotFound, "NOT_FOUND", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		return failure(c, fiber.StatusConflict, "CONFLICT", err.Error())
	default:
		return failure(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred")
	}
}

func parseID(value string) (uint, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(parsed), nil
}

func parseOptionalPositiveInt(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New("invalid positive integer")
	}
	return parsed, nil
}
