package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sdms/internal/modules/user/domain"
	"sdms/internal/modules/user/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type userUsecaseStub struct {
	createFn        func(context.Context, usecase.CreateUserInput) (*domain.User, error)
	listFn          func(context.Context) ([]domain.User, error)
	getByIDFn       func(context.Context, uuid.UUID) (*domain.User, error)
	getByUsernameFn func(context.Context, string) (*domain.User, error)
	updateFn        func(context.Context, uuid.UUID, usecase.UpdateUserInput) (*domain.User, error)
	updateStatusFn  func(context.Context, uuid.UUID, usecase.UpdateUserStatusInput) (*domain.User, error)
	deleteFn        func(context.Context, uuid.UUID) error
}

func (s *userUsecaseStub) Create(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
	if s.createFn != nil { return s.createFn(ctx, input) }
	return nil, errors.New("unexpected Create call")
}

func (s *userUsecaseStub) List(ctx context.Context) ([]domain.User, error) {
	if s.listFn != nil { return s.listFn(ctx) }
	return nil, errors.New("unexpected List call")
}

func (s *userUsecaseStub) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if s.getByIDFn != nil { return s.getByIDFn(ctx, id) }
	return nil, errors.New("unexpected GetByID call")
}

func (s *userUsecaseStub) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if s.getByUsernameFn != nil { return s.getByUsernameFn(ctx, username) }
	return nil, errors.New("unexpected GetByUsername call")
}

func (s *userUsecaseStub) Update(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
	if s.updateFn != nil { return s.updateFn(ctx, id, input) }
	return nil, errors.New("unexpected Update call")
}

func (s *userUsecaseStub) UpdateStatus(ctx context.Context, id uuid.UUID, input usecase.UpdateUserStatusInput) (*domain.User, error) {
	if s.updateStatusFn != nil { return s.updateStatusFn(ctx, id, input) }
	return nil, errors.New("unexpected UpdateStatus call")
}

func (s *userUsecaseStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil { return s.deleteFn(ctx, id) }
	return errors.New("unexpected Delete call")
}

func newUserTestApp(service UserUsecase) *fiber.App {
	app := fiber.New()
	RegisterRoutes(app.Group("/api/v1"), NewUserHandler(service))
	return app
}

func userRequest(t *testing.T, app *fiber.App, method, path string, body []byte) *nethttp.Response {
	t.Helper()
	var reader io.Reader
	if body != nil { reader = bytes.NewReader(body) }
	req := httptest.NewRequest(method, path, reader)
	if body != nil { req.Header.Set("Content-Type", "application/json") }
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil { t.Fatalf("%s %s failed: %v", method, path, err) }
	return resp
}

func decodeUserBody[T any](t *testing.T, resp *nethttp.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var body T
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil { t.Fatalf("decode response: %v", err) }
	return body
}

func sampleUser(id uuid.UUID) *domain.User {
	now := time.Now().UTC()
	return &domain.User{
		UID: id, Username: "somchai", EmployeeCode: "EMP-001", Prefix: "นาย",
		FirstName: "สมชาย", LastName: "ใจดี", Role: domain.RoleTeacher,
		Status: domain.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
}

func TestUserHandlerCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		service := &userUsecaseStub{createFn: func(_ context.Context, input usecase.CreateUserInput) (*domain.User, error) {
			if input.Username != "somchai" || input.EmployeeCode != "EMP-001" || input.Prefix != "นาย" ||
				input.FirstName != "สมชาย" || input.LastName != "ใจดี" || input.Role != domain.RoleTeacher {
				t.Errorf("input = %+v", input)
			}
			return sampleUser(id), nil
		}}
		body := []byte(`{"username":"somchai","employee_code":"EMP-001","prefix":"นาย","first_name":"สมชาย","last_name":"ใจดี","role":"TEACHER"}`)
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodPost, "/api/v1/users", body)
		if resp.StatusCode != fiber.StatusCreated { t.Fatalf("status = %d", resp.StatusCode) }
		got := decodeUserBody[UserResponse](t, resp)
		if got.UID != id || got.Prefix != "นาย" || got.Role != "TEACHER" || got.Status != "ACTIVE" {
			t.Errorf("response = %+v", got)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp := userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodPost, "/api/v1/users", []byte(`{"username":`))
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[ErrorResponse](t, resp); got.Message != "invalid request body" { t.Errorf("response = %+v", got) }
	})

	t.Run("usecase error is mapped", func(t *testing.T) {
		service := &userUsecaseStub{createFn: func(context.Context, usecase.CreateUserInput) (*domain.User, error) {
			return nil, domain.ErrUsernameAlreadyExists
		}}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodPost, "/api/v1/users", []byte(`{}`))
		if resp.StatusCode != fiber.StatusConflict { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[ErrorResponse](t, resp); got.Message != "username already exists" { t.Errorf("response = %+v", got) }
	})
}

func TestUserHandlerList(t *testing.T) {
	id := uuid.New()
	service := &userUsecaseStub{listFn: func(context.Context) ([]domain.User, error) { return []domain.User{*sampleUser(id)}, nil }}
	resp := userRequest(t, newUserTestApp(service), nethttp.MethodGet, "/api/v1/users", nil)
	if resp.StatusCode != fiber.StatusOK { t.Fatalf("status = %d", resp.StatusCode) }
	got := decodeUserBody[[]UserResponse](t, resp)
	if len(got) != 1 || got[0].UID != id || got[0].EmployeeCode != "EMP-001" { t.Errorf("response = %+v", got) }

	service.listFn = func(context.Context) ([]domain.User, error) { return nil, errors.New("failed") }
	resp = userRequest(t, newUserTestApp(service), nethttp.MethodGet, "/api/v1/users", nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusInternalServerError { t.Fatalf("error status = %d", resp.StatusCode) }
}

func TestUserHandlerGetByID(t *testing.T) {
	id := uuid.New()

	t.Run("success", func(t *testing.T) {
		service := &userUsecaseStub{getByIDFn: func(_ context.Context, gotID uuid.UUID) (*domain.User, error) {
			if gotID != id { t.Errorf("id = %s, want %s", gotID, id) }
			return sampleUser(id), nil
		}}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodGet, "/api/v1/users/"+id.String(), nil)
		if resp.StatusCode != fiber.StatusOK { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[UserResponse](t, resp); got.UID != id { t.Errorf("response = %+v", got) }
	})

	t.Run("invalid ID", func(t *testing.T) {
		resp := userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodGet, "/api/v1/users/bad", nil)
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[ErrorResponse](t, resp); got.Message != "invalid user id" { t.Errorf("response = %+v", got) }
	})

	t.Run("not found", func(t *testing.T) {
		service := &userUsecaseStub{getByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return nil, domain.ErrUserNotFound }}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodGet, "/api/v1/users/"+id.String(), nil)
		if resp.StatusCode != fiber.StatusNotFound { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[ErrorResponse](t, resp); got.Message != "user not found" { t.Errorf("response = %+v", got) }
	})
}

func TestUserHandlerGetByUsernameRoutePrecedesIDRoute(t *testing.T) {
	service := &userUsecaseStub{
		getByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
			if username != "somchai" { t.Errorf("username = %q", username) }
			return sampleUser(uuid.New()), nil
		},
		getByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) {
			t.Fatal("GetByID must not be called")
			return nil, nil
		},
	}
	resp := userRequest(t, newUserTestApp(service), nethttp.MethodGet, "/api/v1/users/username/somchai", nil)
	if resp.StatusCode != fiber.StatusOK { t.Fatalf("status = %d", resp.StatusCode) }
	if got := decodeUserBody[UserResponse](t, resp); got.Username != "somchai" { t.Errorf("response = %+v", got) }

	service.getByUsernameFn = func(context.Context, string) (*domain.User, error) { return nil, errors.New("failed") }
	resp = userRequest(t, newUserTestApp(service), nethttp.MethodGet, "/api/v1/users/username/somchai", nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusInternalServerError { t.Fatalf("error status = %d", resp.StatusCode) }
}

func TestUserHandlerUpdate(t *testing.T) {
	id := uuid.New()
	body := []byte(`{"username":"somchai","employee_code":"EMP-001","prefix":"ดร.","first_name":"สมชาย","last_name":"ใจดี","role":"DIRECTOR"}`)

	t.Run("success", func(t *testing.T) {
		service := &userUsecaseStub{updateFn: func(_ context.Context, gotID uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
			if gotID != id || input.Username != "somchai" || input.EmployeeCode != "EMP-001" || input.Prefix != "ดร." ||
				input.FirstName != "สมชาย" || input.LastName != "ใจดี" || input.Role != domain.RoleDirector {
				t.Errorf("id/input = %s, %+v", gotID, input)
			}
			user := sampleUser(id); user.Prefix, user.Role = input.Prefix, input.Role
			return user, nil
		}}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodPut, "/api/v1/users/"+id.String(), body)
		if resp.StatusCode != fiber.StatusOK { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[UserResponse](t, resp); got.Prefix != "ดร." || got.Role != "DIRECTOR" { t.Errorf("response = %+v", got) }
	})

	t.Run("invalid ID", func(t *testing.T) {
		resp := userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodPut, "/api/v1/users/bad", body)
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp := userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodPut, "/api/v1/users/"+id.String(), []byte(`{"username":`))
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[ErrorResponse](t, resp); got.Message != "invalid request body" { t.Errorf("response = %+v", got) }
	})

	t.Run("usecase error", func(t *testing.T) {
		service := &userUsecaseStub{updateFn: func(context.Context, uuid.UUID, usecase.UpdateUserInput) (*domain.User, error) { return nil, domain.ErrEmployeeCodeAlreadyExists }}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodPut, "/api/v1/users/"+id.String(), body)
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusConflict { t.Fatalf("status = %d", resp.StatusCode) }
	})
}

func TestUserHandlerUpdateStatus(t *testing.T) {
	id := uuid.New()

	t.Run("success", func(t *testing.T) {
		service := &userUsecaseStub{updateStatusFn: func(_ context.Context, gotID uuid.UUID, input usecase.UpdateUserStatusInput) (*domain.User, error) {
			if gotID != id || input.Status != domain.StatusInactive { t.Errorf("id/input = %s, %+v", gotID, input) }
			user := sampleUser(id); user.Status = input.Status
			return user, nil
		}}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodPatch, "/api/v1/users/"+id.String()+"/status", []byte(`{"status":"INACTIVE"}`))
		if resp.StatusCode != fiber.StatusOK { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[UserResponse](t, resp); got.Status != "INACTIVE" { t.Errorf("response = %+v", got) }
	})

	t.Run("invalid ID", func(t *testing.T) {
		resp := userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodPatch, "/api/v1/users/bad/status", []byte(`{}`))
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp := userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodPatch, "/api/v1/users/"+id.String()+"/status", []byte(`{"status":`))
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
		if got := decodeUserBody[ErrorResponse](t, resp); got.Message != "invalid request body" { t.Errorf("response = %+v", got) }
	})

	t.Run("usecase error", func(t *testing.T) {
		service := &userUsecaseStub{updateStatusFn: func(context.Context, uuid.UUID, usecase.UpdateUserStatusInput) (*domain.User, error) { return nil, domain.ErrInvalidStatus }}
		resp := userRequest(t, newUserTestApp(service), nethttp.MethodPatch, "/api/v1/users/"+id.String()+"/status", []byte(`{"status":"BAD"}`))
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("status = %d", resp.StatusCode) }
	})
}

func TestUserHandlerDelete(t *testing.T) {
	id := uuid.New()
	service := &userUsecaseStub{deleteFn: func(_ context.Context, gotID uuid.UUID) error {
		if gotID != id { t.Errorf("id = %s, want %s", gotID, id) }
		return nil
	}}
	resp := userRequest(t, newUserTestApp(service), nethttp.MethodDelete, "/api/v1/users/"+id.String(), nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNoContent { t.Fatalf("status = %d", resp.StatusCode) }

	resp = userRequest(t, newUserTestApp(&userUsecaseStub{}), nethttp.MethodDelete, "/api/v1/users/bad", nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest { t.Fatalf("invalid ID status = %d", resp.StatusCode) }

	service.deleteFn = func(context.Context, uuid.UUID) error { return domain.ErrUserNotFound }
	resp = userRequest(t, newUserTestApp(service), nethttp.MethodDelete, "/api/v1/users/"+id.String(), nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound { t.Fatalf("not found status = %d", resp.StatusCode) }
}

func TestUserHandleErrorMappings(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"not found", domain.ErrUserNotFound, fiber.StatusNotFound, "user not found"},
		{"username conflict", domain.ErrUsernameAlreadyExists, fiber.StatusConflict, "username already exists"},
		{"employee conflict", domain.ErrEmployeeCodeAlreadyExists, fiber.StatusConflict, "employee code already exists"},
		{"invalid role", domain.ErrInvalidRole, fiber.StatusBadRequest, "invalid role"},
		{"invalid status", domain.ErrInvalidStatus, fiber.StatusBadRequest, "invalid status"},
		{"invalid ID", domain.ErrInvalidUserID, fiber.StatusBadRequest, domain.ErrInvalidUserID.Error()},
		{"username required", domain.ErrUsernameRequired, fiber.StatusBadRequest, domain.ErrUsernameRequired.Error()},
		{"employee required", domain.ErrEmployeeCodeRequired, fiber.StatusBadRequest, domain.ErrEmployeeCodeRequired.Error()},
		{"prefix required", domain.ErrPrefixRequired, fiber.StatusBadRequest, domain.ErrPrefixRequired.Error()},
		{"first name required", domain.ErrFirstNameRequired, fiber.StatusBadRequest, domain.ErrFirstNameRequired.Error()},
		{"last name required", domain.ErrLastNameRequired, fiber.StatusBadRequest, domain.ErrLastNameRequired.Error()},
		{"internal", errors.New("database failed"), fiber.StatusInternalServerError, "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error { return handleError(c, tt.err) })
			resp := userRequest(t, app, nethttp.MethodGet, "/", nil)
			if resp.StatusCode != tt.wantStatus { t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus) }
			if got := decodeUserBody[ErrorResponse](t, resp); got.Message != tt.wantBody { t.Errorf("response = %+v", got) }
		})
	}
}

func TestNewUserListResponseReturnsEmptyArrayForEmptyInput(t *testing.T) {
	got := newUserListResponse(nil)
	if got == nil || len(got) != 0 { t.Fatalf("newUserListResponse(nil) = %#v", got) }
}
