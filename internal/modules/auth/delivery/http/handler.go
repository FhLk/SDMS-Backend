package http

import (
	"context"
	"errors"

	authdomain "sdms/internal/modules/auth/domain"
	userdomain "sdms/internal/modules/user/domain"
	platformmiddleware "sdms/internal/platform/http/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AuthUsecase interface {
	Login(ctx context.Context, username, password string) (*userdomain.User, string, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
}

type Handler struct {
	service AuthUsecase
}

func NewHandler(service AuthUsecase) *Handler {
	return &Handler{service: service}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type authUserResponse struct {
	UID          string `json:"uid"`
	Username     string `json:"username"`
	EmployeeCode string `json:"employee_code"`
	Prefix       string `json:"prefix"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Role         string `json:"role"`
	Status       string `json:"status"`
}

type loginResponse struct {
	AccessToken string           `json:"access_token"`
	TokenType   string           `json:"token_type"`
	User        authUserResponse `json:"user"`
}

func (h *Handler) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid request body"})
	}

	user, token, err := h.service.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, authdomain.ErrInvalidCredentials):
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": err.Error()})
		case errors.Is(err, authdomain.ErrInactiveUser):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "internal server error"})
		}
	}

	return c.JSON(loginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		User:        newAuthUserResponse(user),
	})
}

func (h *Handler) ChangePassword(c fiber.Ctx) error {
	user, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}

	var req changePasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid request body"})
	}

	if err := h.service.ChangePassword(c.Context(), user.UID, req.CurrentPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, authdomain.ErrInvalidCredentials):
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "current password is invalid"})
		case errors.Is(err, authdomain.ErrPasswordRequired), errors.Is(err, authdomain.ErrPasswordTooShort):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "internal server error"})
		}
	}

	return c.JSON(fiber.Map{"message": "password changed successfully"})
}

func (h *Handler) Me(c fiber.Ctx) error {
	user, ok := platformmiddleware.CurrentUser(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
	}
	return c.JSON(newAuthUserResponse(user))
}

func newAuthUserResponse(user *userdomain.User) authUserResponse {
	return authUserResponse{
		UID:          user.UID.String(),
		Username:     user.Username,
		EmployeeCode: user.EmployeeCode,
		Prefix:       user.Prefix,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Role:         string(user.Role),
		Status:       string(user.Status),
	}
}
