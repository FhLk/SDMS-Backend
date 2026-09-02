package http

import (
	"bytes"
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"sdms/internal/modules/user/domain"
	"sdms/internal/modules/user/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeUserUsecase struct {
	createFn        func(context.Context, usecase.CreateUserInput) (*domain.User, error)
	listFn          func(context.Context) ([]domain.User, error)
	getByIDFn       func(context.Context, uuid.UUID) (*domain.User, error)
	getByUsernameFn func(context.Context, string) (*domain.User, error)
	updateFn        func(context.Context, uuid.UUID, usecase.UpdateUserInput) (*domain.User, error)
	updateStatusFn  func(context.Context, uuid.UUID, usecase.UpdateUserStatusInput) (*domain.User, error)
	deleteFn        func(context.Context, uuid.UUID) error
}

func (f *fakeUserUsecase) Create(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
	return f.createFn(ctx, input)
}

func (f *fakeUserUsecase) List(ctx context.Context) ([]domain.User, error) {
	return f.listFn(ctx)
}

func (f *fakeUserUsecase) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return f.getByIDFn(ctx, id)
}

func (f *fakeUserUsecase) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return f.getByUsernameFn(ctx, username)
}

func (f *fakeUserUsecase) Update(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
	return f.updateFn(ctx, id, input)
}

func (f *fakeUserUsecase) UpdateStatus(ctx context.Context, id uuid.UUID, input usecase.UpdateUserStatusInput) (*domain.User, error) {
	return f.updateStatusFn(ctx, id, input)
}

func (f *fakeUserUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}

func newUserTestApp(service UserUsecase) *fiber.App {
	app := fiber.New()
	RegisterRoutes(app.Group("/api/v1"), NewUserHandler(service))
	return app
}

func doUserRequest(t *testing.T, app *fiber.App, request *nethttp.Request) *nethttp.Response {
	t.Helper()
	response, err := app.Test(request, fiber.TestConfig{
		Timeout:       0,
		FailOnTimeout: false,
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return response
}

func TestUserRoutesGetByUsernameDoesNotConflictWithID(t *testing.T) {
	service := &fakeUserUsecase{
		getByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
			if username != "somchai" {
				t.Errorf("username = %q, want %q", username, "somchai")
			}
			return &domain.User{
				UID:          uuid.New(),
				Username:     username,
				EmployeeCode: "EMP-001",
				Prefix:       "นาย",
				FirstName:    "สมชาย",
				LastName:     "ใจดี",
				Role:         domain.RoleTeacher,
				Status:       domain.StatusActive,
			}, nil
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
			t.Fatal("GetByID() must not be called for the username route")
			return nil, nil
		},
	}
	app := newUserTestApp(service)
	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/users/username/somchai", nil)

	response := doUserRequest(t, app, request)
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	var body UserResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Prefix != "นาย" || body.Role != string(domain.RoleTeacher) {
		t.Errorf("response = %+v", body)
	}
}

func TestUserHandlerUpdatePassesPrefixAndPathID(t *testing.T) {
	id := uuid.New()
	service := &fakeUserUsecase{
		updateFn: func(_ context.Context, gotID uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			if input.Prefix != "ดร." {
				t.Errorf("Prefix = %q, want %q", input.Prefix, "ดร.")
			}
			return &domain.User{
				UID:          id,
				Username:     input.Username,
				EmployeeCode: input.EmployeeCode,
				Prefix:       input.Prefix,
				FirstName:    input.FirstName,
				LastName:     input.LastName,
				Role:         input.Role,
				Status:       domain.StatusActive,
			}, nil
		},
	}
	app := newUserTestApp(service)
	body := []byte(`{"username":"somchai","employee_code":"EMP-001","prefix":"ดร.","first_name":"สมชาย","last_name":"ใจดี","role":"DIRECTOR"}`)
	request := httptest.NewRequest(nethttp.MethodPut, "/api/v1/users/"+id.String(), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	response := doUserRequest(t, app, request)
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
}

func TestUserHandlerUpdateStatusBindsRequest(t *testing.T) {
	id := uuid.New()
	service := &fakeUserUsecase{
		updateStatusFn: func(_ context.Context, gotID uuid.UUID, input usecase.UpdateUserStatusInput) (*domain.User, error) {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			if input.Status != domain.StatusInactive {
				t.Errorf("Status = %q, want %q", input.Status, domain.StatusInactive)
			}
			return &domain.User{UID: id, Status: input.Status}, nil
		},
	}
	app := newUserTestApp(service)
	request := httptest.NewRequest(
		nethttp.MethodPatch,
		"/api/v1/users/"+id.String()+"/status",
		bytes.NewBufferString(`{"status":"INACTIVE"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := doUserRequest(t, app, request)
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
}

func TestUserHandlerMapsRequiredFieldErrorToBadRequest(t *testing.T) {
	service := &fakeUserUsecase{
		createFn: func(_ context.Context, _ usecase.CreateUserInput) (*domain.User, error) {
			return nil, domain.ErrPrefixRequired
		},
	}
	app := newUserTestApp(service)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/users",
		bytes.NewBufferString(`{"username":"somchai"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := doUserRequest(t, app, request)
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
	}
	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Message != domain.ErrPrefixRequired.Error() {
		t.Errorf("message = %q, want %q", body.Message, domain.ErrPrefixRequired.Error())
	}
}
