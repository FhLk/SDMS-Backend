package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	userdomain "sdms/internal/modules/user/domain"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type authUserRepositoryStub struct {
	user *userdomain.User
	err  error
}

func (s authUserRepositoryStub) FindByID(context.Context, uuid.UUID) (*userdomain.User, error) {
	return s.user, s.err
}

type tokenParserStub struct {
	userID uuid.UUID
	err    error
}

func (s tokenParserStub) Parse(string) (uuid.UUID, error) {
	return s.userID, s.err
}

func TestRequireAuthStoresCurrentUser(t *testing.T) {
	user := &userdomain.User{UID: uuid.New(), Role: userdomain.RoleTeacher, Status: userdomain.StatusActive}
	middleware := NewAuth(authUserRepositoryStub{user: user}, tokenParserStub{userID: user.UID})
	app := fiber.New()
	app.Get("/protected", middleware.RequireAuth, func(c fiber.Ctx) error {
		current, ok := CurrentUser(c)
		if !ok || current.UID != user.UID {
			t.Fatal("current user was not stored in locals")
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusNoContent)
	}
}

func TestRequireAuthRejectsMissingBearerToken(t *testing.T) {
	middleware := NewAuth(authUserRepositoryStub{}, tokenParserStub{})
	app := fiber.New()
	app.Get("/protected", middleware.RequireAuth, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestRequireRolesRejectsTeacherFromDirectorRoute(t *testing.T) {
	user := &userdomain.User{UID: uuid.New(), Role: userdomain.RoleTeacher, Status: userdomain.StatusActive}
	middleware := NewAuth(authUserRepositoryStub{}, tokenParserStub{})
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		SetCurrentUser(c, user)
		return c.Next()
	})
	app.Get("/director", middleware.RequireRoles(userdomain.RoleDirector), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/director", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusForbidden)
	}
}
