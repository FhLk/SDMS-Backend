package middleware

import (
	"context"
	"errors"
	"strings"

	userdomain "sdms/internal/modules/user/domain"
	platformauth "sdms/internal/platform/auth"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const currentUserLocalKey = "sdms.current_user"

type UserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error)
}

type TokenParser interface {
	Parse(token string) (uuid.UUID, error)
}

type Auth struct {
	users  UserRepository
	tokens TokenParser
}

func NewAuth(users UserRepository, tokens TokenParser) *Auth {
	return &Auth{users: users, tokens: tokens}
}

func (m *Auth) RequireAuth(c fiber.Ctx) error {
	authorization := strings.TrimSpace(c.Get("Authorization"))
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return unauthorized(c)
	}

	userID, err := m.tokens.Parse(parts[1])
	if err != nil {
		if errors.Is(err, platformauth.ErrExpiredToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "token expired"})
		}
		return unauthorized(c)
	}

	user, err := m.users.FindByID(c.Context(), userID)
	if err != nil || user.Status != userdomain.StatusActive {
		return unauthorized(c)
	}

	c.Locals(currentUserLocalKey, user)
	return c.Next()
}

func (m *Auth) RequireRoles(roles ...userdomain.Role) fiber.Handler {
	allowed := make(map[userdomain.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c fiber.Ctx) error {
		user, ok := CurrentUser(c)
		if !ok {
			return unauthorized(c)
		}
		if _, ok := allowed[user.Role]; !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "forbidden"})
		}
		return c.Next()
	}
}

func CurrentUser(c fiber.Ctx) (*userdomain.User, bool) {
	value := c.Locals(currentUserLocalKey)
	user, ok := value.(*userdomain.User)
	return user, ok && user != nil
}

func SetCurrentUser(c fiber.Ctx, user *userdomain.User) {
	c.Locals(currentUserLocalKey, user)
}

func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "authentication required"})
}
