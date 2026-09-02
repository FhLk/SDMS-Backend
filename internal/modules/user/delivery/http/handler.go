package http

import (
	"context"
	"errors"

	"sdms/internal/modules/user/domain"
	"sdms/internal/modules/user/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserUsecase interface {
	Create(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status usecase.UpdateUserStatusInput) (*domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type userHandler struct {
	userUsecase UserUsecase
}

func NewUserHandler(userUsecase UserUsecase) *userHandler {
	return &userHandler{
		userUsecase: userUsecase,
	}
}

func (h *userHandler) Create(c fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			ErrorResponse{
				Message: "invalid request body",
			},
		)
	}

	input := usecase.CreateUserInput{
		Username:     req.Username,
		Prefix:       req.Prefix,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		EmployeeCode: req.EmployeeCode,
		Role:         domain.Role(req.Role),
	}

	user, err := h.userUsecase.Create(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(newUserResponse(user))
}

func (h *userHandler) List(c fiber.Ctx) error {
	users, err := h.userUsecase.List(c.Context())
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(newUserListResponse(users))
}

func (h *userHandler) GetByID(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "invalid user id"})
	}

	user, err := h.userUsecase.GetByID(c.Context(), id)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(newUserResponse(user))
}

func (h *userHandler) GetByUsername(c fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "invalid username"})
	}

	user, err := h.userUsecase.GetByUsername(c.Context(), username)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(newUserResponse(user))
}

func (h *userHandler) Update(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "invalid user id"})
	}

	var req UpdateUserRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			ErrorResponse{
				Message: "invalid request body",
			},
		)
	}

	input := usecase.UpdateUserInput{
		Username:     req.Username,
		EmployeeCode: req.EmployeeCode,
		Prefix:       req.Prefix,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         domain.Role(req.Role),
	}

	user, err := h.userUsecase.Update(c.Context(), id, input)

	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(newUserResponse(user))
}

func (h *userHandler) UpdateStatus(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "invalid user id"})
	}

	var req UpdateUserStatusRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			ErrorResponse{
				Message: "invalid request body",
			},
		)
	}

	input := usecase.UpdateUserStatusInput{
		Status: domain.Status(req.Status),
	}

	user, err := h.userUsecase.UpdateStatus(c.Context(), id, input)

	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(newUserResponse(user))
}

func (h *userHandler) Delete(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "invalid user id"})
	}

	if err := h.userUsecase.Delete(c.Context(), id); err != nil {
		return handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func handleError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return c.Status(fiber.StatusNotFound).JSON(
			ErrorResponse{
				Message: "user not found",
			},
		)

	case errors.Is(err, domain.ErrUsernameAlreadyExists):
		return c.Status(fiber.StatusConflict).JSON(
			ErrorResponse{
				Message: "username already exists",
			},
		)

	case errors.Is(err, domain.ErrEmployeeCodeAlreadyExists):
		return c.Status(fiber.StatusConflict).JSON(
			ErrorResponse{
				Message: "employee code already exists",
			},
		)

	case errors.Is(err, domain.ErrInvalidRole):
		return c.Status(fiber.StatusBadRequest).JSON(
			ErrorResponse{
				Message: "invalid role",
			},
		)

	case errors.Is(err, domain.ErrInvalidStatus):
		return c.Status(fiber.StatusBadRequest).JSON(
			ErrorResponse{
				Message: "invalid status",
			},
		)

	case errors.Is(err, domain.ErrInvalidUserID),
		errors.Is(err, domain.ErrUsernameRequired),
		errors.Is(err, domain.ErrEmployeeCodeRequired),
		errors.Is(err, domain.ErrPrefixRequired),
		errors.Is(err, domain.ErrFirstNameRequired),
		errors.Is(err, domain.ErrLastNameRequired):
		return c.Status(fiber.StatusBadRequest).JSON(
			ErrorResponse{
				Message: err.Error(),
			},
		)

	default:
		return c.Status(fiber.StatusInternalServerError).JSON(
			ErrorResponse{
				Message: "internal server error",
			},
		)
	}
}
