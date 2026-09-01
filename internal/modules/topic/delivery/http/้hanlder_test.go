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

	"sdms/internal/modules/topic/domain"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeTopicService struct {
	createFn func(
		ctx context.Context,
		name string,
		description string,
	) (*domain.Topic, error)

	findAllFn func(
		ctx context.Context,
	) ([]domain.Topic, error)

	findByIDFn func(
		ctx context.Context,
		id uuid.UUID,
	) (*domain.Topic, error)

	updateFn func(
		ctx context.Context,
		id uuid.UUID,
		name string,
		description string,
		isActive bool,
	) (*domain.Topic, error)

	deleteFn func(
		ctx context.Context,
		id uuid.UUID,
	) error
}

func (f *fakeTopicService) Create(
	ctx context.Context,
	name string,
	description string,
) (*domain.Topic, error) {
	if f.createFn != nil {
		return f.createFn(ctx, name, description)
	}
	return nil, nil
}

func (f *fakeTopicService) FindAll(
	ctx context.Context,
) ([]domain.Topic, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx)
	}
	return nil, nil
}

func (f *fakeTopicService) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.Topic, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *fakeTopicService) Update(
	ctx context.Context,
	id uuid.UUID,
	name string,
	description string,
	isActive bool,
) (*domain.Topic, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, name, description, isActive)
	}
	return nil, nil
}

func (f *fakeTopicService) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

func setupTestApp(service TopicService) *fiber.App {
	app := fiber.New()
	handler := NewHandler(service)

	v1 := app.Group("/api/v1")
	RegisterRoutes(v1, handler)

	return app
}

func performRequest(
	t *testing.T,
	app *fiber.App,
	req *nethttp.Request,
) *nethttp.Response {
	t.Helper()

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout:       0,
		FailOnTimeout: false,
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return resp
}

func TestHandler_Create_Success(t *testing.T) {
	topicID := uuid.New()
	now := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)

	service := &fakeTopicService{
		createFn: func(
			ctx context.Context,
			name string,
			description string,
		) (*domain.Topic, error) {
			if name != "งานวิชาการ" {
				t.Errorf("expected name %q, got %q", "งานวิชาการ", name)
			}
			if description != "เอกสารงานวิชาการ" {
				t.Errorf("expected description %q, got %q", "เอกสารงานวิชาการ", description)
			}

			return &domain.Topic{
				UID:         topicID,
				Name:        name,
				Description: description,
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	app := setupTestApp(service)

	body := `{
		"name": "งานวิชาการ",
		"description": "เอกสารงานวิชาการ"
	}`

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status %d, got %d", fiber.StatusCreated, resp.StatusCode)
	}

	var response TopicResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.UID != topicID {
		t.Errorf("expected ID %s, got %s", topicID, response.UID)
	}
	if response.Name != "งานวิชาการ" {
		t.Errorf("expected name %q, got %q", "งานวิชาการ", response.Name)
	}
	if response.Description != "เอกสารงานวิชาการ" {
		t.Errorf("expected description %q, got %q", "เอกสารงานวิชาการ", response.Description)
	}
	if !response.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestHandler_Create_InvalidJSON(t *testing.T) {
	createCalled := false

	service := &fakeTopicService{
		createFn: func(
			ctx context.Context,
			name string,
			description string,
		) (*domain.Topic, error) {
			createCalled = true
			return nil, nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"name":`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
	if createCalled {
		t.Error("service Create should not be called when JSON is invalid")
	}
}

func TestHandler_Create_ValidationError(t *testing.T) {
	service := &fakeTopicService{
		createFn: func(
			ctx context.Context,
			name string,
			description string,
		) (*domain.Topic, error) {
			return nil, domain.ErrTopicNameEmpty
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"name":"","description":"test"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["message"] != domain.ErrTopicNameEmpty.Error() {
		t.Errorf("expected message %q, got %q", domain.ErrTopicNameEmpty.Error(), response["message"])
	}
}

func TestHandler_Create_InternalServerError(t *testing.T) {
	service := &fakeTopicService{
		createFn: func(
			ctx context.Context,
			name string,
			description string,
		) (*domain.Topic, error) {
			return nil, errors.New("database unavailable")
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"name":"งานวิชาการ","description":"test"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", fiber.StatusInternalServerError, resp.StatusCode)
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["message"] != "internal server error" {
		t.Errorf("expected generic internal error, got %q", response["message"])
	}
}

func TestHandler_FindAll_Success(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	service := &fakeTopicService{
		findAllFn: func(ctx context.Context) ([]domain.Topic, error) {
			return []domain.Topic{
				{UID: id1, Name: "งานวิชาการ", IsActive: true},
				{UID: id2, Name: "งานบุคคล", IsActive: true},
			}, nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/topics", nil)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}

	var response []TopicResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(response))
	}
	if response[0].UID != id1 {
		t.Errorf("expected first ID %s, got %s", id1, response[0].UID)
	}
	if response[1].Name != "งานบุคคล" {
		t.Errorf("expected second name %q, got %q", "งานบุคคล", response[1].Name)
	}
}

func TestHandler_FindAll_InternalServerError(t *testing.T) {
	service := &fakeTopicService{
		findAllFn: func(ctx context.Context) ([]domain.Topic, error) {
			return nil, errors.New("database unavailable")
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/topics", nil)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", fiber.StatusInternalServerError, resp.StatusCode)
	}
}

func TestHandler_FindByID_Success(t *testing.T) {
	topicID := uuid.New()

	service := &fakeTopicService{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Topic, error) {
			if id != topicID {
				t.Errorf("expected ID %s, got %s", topicID, id)
			}

			return &domain.Topic{
				UID:         topicID,
				Name:        "งานวิชาการ",
				Description: "test",
				IsActive:    true,
			}, nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/topics/"+topicID.String(),
		nil,
	)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}

	var response TopicResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.UID != topicID {
		t.Errorf("expected ID %s, got %s", topicID, response.UID)
	}
}

func TestHandler_FindByID_InvalidUUID(t *testing.T) {
	serviceCalled := false

	service := &fakeTopicService{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Topic, error) {
			serviceCalled = true
			return nil, nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/topics/not-a-uuid", nil)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
	if serviceCalled {
		t.Error("service FindByID should not be called when UUID is invalid")
	}
}

func TestHandler_FindByID_NotFound(t *testing.T) {
	topicID := uuid.New()

	service := &fakeTopicService{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Topic, error) {
			return nil, domain.ErrTopicNotFound
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/topics/"+topicID.String(),
		nil,
	)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected status %d, got %d", fiber.StatusNotFound, resp.StatusCode)
	}
}

func TestHandler_Update_Success(t *testing.T) {
	topicID := uuid.New()

	service := &fakeTopicService{
		updateFn: func(
			ctx context.Context,
			id uuid.UUID,
			name string,
			description string,
			isActive bool,
		) (*domain.Topic, error) {
			if id != topicID {
				t.Errorf("expected ID %s, got %s", topicID, id)
			}
			if name != "งานวิชาการใหม่" {
				t.Errorf("expected name %q, got %q", "งานวิชาการใหม่", name)
			}
			if description != "รายละเอียดใหม่" {
				t.Errorf("expected description %q, got %q", "รายละเอียดใหม่", description)
			}
			if isActive {
				t.Error("expected isActive false")
			}

			return &domain.Topic{
				UID:         id,
				Name:        name,
				Description: description,
				IsActive:    isActive,
			}, nil
		},
	}

	app := setupTestApp(service)

	body := `{
		"name": "งานวิชาการใหม่",
		"description": "รายละเอียดใหม่",
		"is_active": false
	}`

	req := httptest.NewRequest(
		nethttp.MethodPut,
		"/api/v1/topics/"+topicID.String(),
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}

	var response TopicResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.UID != topicID {
		t.Errorf("expected ID %s, got %s", topicID, response.UID)
	}
	if response.IsActive {
		t.Error("expected IsActive false")
	}
}

func TestHandler_Update_InvalidUUID(t *testing.T) {
	updateCalled := false

	service := &fakeTopicService{
		updateFn: func(
			ctx context.Context,
			id uuid.UUID,
			name string,
			description string,
			isActive bool,
		) (*domain.Topic, error) {
			updateCalled = true
			return nil, nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodPut,
		"/api/v1/topics/not-a-uuid",
		bytes.NewBufferString(`{"name":"test","description":"test","is_active":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
	if updateCalled {
		t.Error("service Update should not be called when UUID is invalid")
	}
}

func TestHandler_Update_InvalidJSON(t *testing.T) {
	updateCalled := false
	topicID := uuid.New()

	service := &fakeTopicService{
		updateFn: func(
			ctx context.Context,
			id uuid.UUID,
			name string,
			description string,
			isActive bool,
		) (*domain.Topic, error) {
			updateCalled = true
			return nil, nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodPut,
		"/api/v1/topics/"+topicID.String(),
		bytes.NewBufferString(`{"name":`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
	if updateCalled {
		t.Error("service Update should not be called when JSON is invalid")
	}
}

func TestHandler_Update_NotFound(t *testing.T) {
	topicID := uuid.New()

	service := &fakeTopicService{
		updateFn: func(
			ctx context.Context,
			id uuid.UUID,
			name string,
			description string,
			isActive bool,
		) (*domain.Topic, error) {
			return nil, domain.ErrTopicNotFound
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodPut,
		"/api/v1/topics/"+topicID.String(),
		bytes.NewBufferString(`{"name":"test","description":"test","is_active":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected status %d, got %d", fiber.StatusNotFound, resp.StatusCode)
	}
}

func TestHandler_Delete_Success(t *testing.T) {
	topicID := uuid.New()
	deleteCalled := false

	service := &fakeTopicService{
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			deleteCalled = true
			if id != topicID {
				t.Errorf("expected ID %s, got %s", topicID, id)
			}
			return nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodDelete,
		"/api/v1/topics/"+topicID.String(),
		nil,
	)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNoContent {
		t.Errorf("expected status %d, got %d", fiber.StatusNoContent, resp.StatusCode)
	}
	if !deleteCalled {
		t.Error("expected Delete service to be called")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body for 204 response, got %q", string(body))
	}
}

func TestHandler_Delete_InvalidUUID(t *testing.T) {
	deleteCalled := false

	service := &fakeTopicService{
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			deleteCalled = true
			return nil
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(nethttp.MethodDelete, "/api/v1/topics/not-a-uuid", nil)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
	if deleteCalled {
		t.Error("service Delete should not be called when UUID is invalid")
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	topicID := uuid.New()

	service := &fakeTopicService{
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			return domain.ErrTopicNotFound
		},
	}

	app := setupTestApp(service)

	req := httptest.NewRequest(
		nethttp.MethodDelete,
		"/api/v1/topics/"+topicID.String(),
		nil,
	)
	resp := performRequest(t, app, req)
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected status %d, got %d", fiber.StatusNotFound, resp.StatusCode)
	}
}
