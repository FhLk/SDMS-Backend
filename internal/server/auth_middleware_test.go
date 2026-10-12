package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/authorization"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func TestAuthenticationMiddlewareRejectsMissingSession(t *testing.T) {
	service := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "" {
				t.Fatalf("session token = %q, want empty", token)
			}
			return auth.Profile{}, auth.ErrUnauthenticated
		},
	}
	app := protectedTestApp(service, func(auth.Profile) bool { return true })

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "UNAUTHORIZED")
}

func TestAuthenticationMiddlewareRejectsEmptyProfile(t *testing.T) {
	service := fakeAuthService{
		currentUser: func(_ context.Context, _ string) (auth.Profile, error) {
			return auth.Profile{}, nil
		},
	}
	app := protectedTestApp(service, func(auth.Profile) bool { return true })

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "invalid-profile-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "UNAUTHORIZED")
}

func TestAuthorizationMiddlewareRejectsInsufficientPermission(t *testing.T) {
	divisionUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f820001")
	service := authenticatedFakeService(model.User{
		UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f820002")},
	})
	app := protectedTestApp(service, func(profile auth.Profile) bool {
		return authorization.CanManageTopic(profile, divisionUUID)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "valid-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "FORBIDDEN")
}

func TestAuthorizationMiddlewareAllowsSufficientPermission(t *testing.T) {
	divisionUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f830001")
	service := authenticatedFakeService(model.User{
		UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f830002")},
		IsAdmin:   true,
	})
	app := protectedTestApp(service, func(profile auth.Profile) bool {
		return authorization.CanManageTopic(profile, divisionUUID)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "valid-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func protectedTestApp(service authService, check permissionCheck) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: handleError})
	handler := authHandler{service: service, config: testAuthConfig()}
	app.Get("/protected", handler.authenticate, requirePermission(check), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	return app
}

func authenticatedFakeService(user model.User) fakeAuthService {
	return fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "valid-session" {
				return auth.Profile{}, auth.ErrUnauthenticated
			}
			return auth.Profile{User: user}, nil
		},
	}
}
