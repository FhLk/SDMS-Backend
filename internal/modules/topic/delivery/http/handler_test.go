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
	"sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type topicServiceStub struct {
	createTopicFn func(context.Context, string, string) (*domain.Topic, error)
	findAllFn     func(context.Context) ([]domain.Topic, error)
	findByIDFn    func(context.Context, uuid.UUID) (*domain.Topic, error)
	updateFn      func(context.Context, uuid.UUID, string, string, bool) (*domain.Topic, error)
	deleteFn      func(context.Context, uuid.UUID) error
	createFieldFn func(context.Context, uuid.UUID, usecase.CreateFieldInput) (*domain.TopicField, error)
}

func (s *topicServiceStub) CreateTopic(ctx context.Context, name, description string) (*domain.Topic, error) {
	if s.createTopicFn != nil {
		return s.createTopicFn(ctx, name, description)
	}
	return nil, errors.New("unexpected CreateTopic call")
}

func (s *topicServiceStub) FindAll(ctx context.Context) ([]domain.Topic, error) {
	if s.findAllFn != nil {
		return s.findAllFn(ctx)
	}
	return nil, errors.New("unexpected FindAll call")
}

func (s *topicServiceStub) FindByID(ctx context.Context, id uuid.UUID) (*domain.Topic, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, errors.New("unexpected FindByID call")
}

func (s *topicServiceStub) Update(ctx context.Context, id uuid.UUID, name, description string, active bool) (*domain.Topic, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, name, description, active)
	}
	return nil, errors.New("unexpected Update call")
}

func (s *topicServiceStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return errors.New("unexpected Delete call")
}

func (s *topicServiceStub) CreateField(ctx context.Context, id uuid.UUID, input usecase.CreateFieldInput) (*domain.TopicField, error) {
	if s.createFieldFn != nil {
		return s.createFieldFn(ctx, id, input)
	}
	return nil, errors.New("unexpected CreateField call")
}

func newTopicTestApp(service TopicService) *fiber.App {
	app := fiber.New()
	RegisterTopicRoutes(app.Group("/api/v1"), NewTopicHandler(service))
	return app
}

func topicRequest(t *testing.T, app *fiber.App, method, path string, body []byte) *nethttp.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	return resp
}

func decodeTopicBody[T any](t *testing.T, resp *nethttp.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var body T
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

func TestTopicHandlerCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		service := &topicServiceStub{createTopicFn: func(_ context.Context, name, description string) (*domain.Topic, error) {
			if name != "งานวิชาการ" || description != "รายละเอียด" {
				t.Errorf("CreateTopic() arguments = %q, %q", name, description)
			}
			return &domain.Topic{UID: id, Name: name, Description: description, IsActive: true}, nil
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodPost, "/api/v1/topics", []byte(`{"name":"งานวิชาการ","description":"รายละเอียด"}`))
		if resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[TopicResponse](t, resp)
		if body.UID != id || body.Name != "งานวิชาการ" || !body.IsActive {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp := topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodPost, "/api/v1/topics", []byte(`{"name":`))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["message"] != "invalid request body" {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		service := &topicServiceStub{createTopicFn: func(context.Context, string, string) (*domain.Topic, error) {
			return nil, domain.ErrTopicNameEmpty
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodPost, "/api/v1/topics", []byte(`{"name":""}`))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["message"] != domain.ErrTopicNameEmpty.Error() {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		service := &topicServiceStub{createTopicFn: func(context.Context, string, string) (*domain.Topic, error) {
			return nil, errors.New("database failed")
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodPost, "/api/v1/topics", []byte(`{"name":"topic"}`))
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["message"] != "internal server error" {
			t.Errorf("response = %+v", body)
		}
	})
}

func TestTopicHandlerFindAll(t *testing.T) {
	now := time.Now().UTC()
	service := &topicServiceStub{findAllFn: func(context.Context) ([]domain.Topic, error) {
		return []domain.Topic{{UID: uuid.New(), Name: "one", IsActive: true, CreatedAt: now, UpdatedAt: now}}, nil
	}}
	resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodGet, "/api/v1/topics", nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeTopicBody[[]TopicResponse](t, resp)
	if len(body) != 1 || body[0].Name != "one" || !body[0].CreatedAt.Equal(now) {
		t.Errorf("response = %+v", body)
	}

	service.findAllFn = func(context.Context) ([]domain.Topic, error) { return nil, errors.New("failed") }
	resp = topicRequest(t, newTopicTestApp(service), nethttp.MethodGet, "/api/v1/topics", nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("error status = %d", resp.StatusCode)
	}
}

func TestTopicHandlerFindByID(t *testing.T) {
	id := uuid.New()

	t.Run("success", func(t *testing.T) {
		service := &topicServiceStub{findByIDFn: func(_ context.Context, got uuid.UUID) (*domain.Topic, error) {
			if got != id {
				t.Errorf("id = %s, want %s", got, id)
			}
			return &domain.Topic{UID: id, Name: "topic"}, nil
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodGet, "/api/v1/topics/"+id.String(), nil)
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if body := decodeTopicBody[TopicResponse](t, resp); body.UID != id {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		resp := topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodGet, "/api/v1/topics/not-a-uuid", nil)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["message"] != "invalid topic id" {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		service := &topicServiceStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) {
			return nil, domain.ErrTopicNotFound
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodGet, "/api/v1/topics/"+id.String(), nil)
		if resp.StatusCode != fiber.StatusNotFound {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["message"] != domain.ErrTopicNotFound.Error() {
			t.Errorf("response = %+v", body)
		}
	})
}

func TestTopicHandlerUpdate(t *testing.T) {
	id := uuid.New()

	t.Run("success", func(t *testing.T) {
		service := &topicServiceStub{updateFn: func(_ context.Context, gotID uuid.UUID, name, description string, active bool) (*domain.Topic, error) {
			if gotID != id || name != "new" || description != "detail" || active {
				t.Errorf("Update() arguments = %s, %q, %q, %v", gotID, name, description, active)
			}
			return &domain.Topic{UID: id, Name: name, Description: description, IsActive: active}, nil
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodPut, "/api/v1/topics/"+id.String(), []byte(`{"name":"new","description":"detail","is_active":false}`))
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[TopicResponse](t, resp)
		if body.UID != id || body.Name != "new" || body.IsActive {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		resp := topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodPut, "/api/v1/topics/bad", []byte(`{}`))
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp := topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodPut, "/api/v1/topics/"+id.String(), []byte(`{"name":`))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["message"] != "invalid request body" {
			t.Errorf("response = %+v", body)
		}
	})
}

func TestTopicHandlerDelete(t *testing.T) {
	id := uuid.New()

	service := &topicServiceStub{deleteFn: func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Errorf("id = %s, want %s", got, id)
		}
		return nil
	}}
	resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodDelete, "/api/v1/topics/"+id.String(), nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	resp = topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodDelete, "/api/v1/topics/bad", nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("invalid ID status = %d", resp.StatusCode)
	}

	service.deleteFn = func(context.Context, uuid.UUID) error { return domain.ErrTopicNotFound }
	resp = topicRequest(t, newTopicTestApp(service), nethttp.MethodDelete, "/api/v1/topics/"+id.String(), nil)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("not found status = %d", resp.StatusCode)
	}
}

func TestTopicHandlerCreateField(t *testing.T) {
	topicID := uuid.New()

	t.Run("uses route ID and binds every field", func(t *testing.T) {
		fieldID := uuid.New()
		service := &topicServiceStub{createFieldFn: func(_ context.Context, gotID uuid.UUID, input usecase.CreateFieldInput) (*domain.TopicField, error) {
			if gotID != topicID {
				t.Errorf("topic ID = %s, want %s", gotID, topicID)
			}
			if input.Label != "เอกสาร" || input.Type != domain.FieldTypeFile || !input.Required || input.Position != 4 {
				t.Errorf("input = %+v", input)
			}
			return &domain.TopicField{UID: fieldID, TopicUID: topicID, Label: input.Label, Type: input.Type, Required: input.Required, Position: input.Position}, nil
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodPost, "/api/v1/topics/"+topicID.String()+"/fields", []byte(`{"label":"เอกสาร","type":"file","required":true,"position":4}`))
		if resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[TopicFieldResponse](t, resp)
		if body.UID != fieldID || body.TopicUID != topicID || body.Type != "file" || !body.Required || body.Position != 4 {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("invalid topic ID", func(t *testing.T) {
		resp := topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodPost, "/api/v1/topics/bad/fields", []byte(`{}`))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["error"] != "invalid topic id" {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		resp := topicRequest(t, newTopicTestApp(&topicServiceStub{}), nethttp.MethodPost, "/api/v1/topics/"+topicID.String()+"/fields", []byte(`{"label":`))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["error"] != "invalid request body" {
			t.Errorf("response = %+v", body)
		}
	})

	t.Run("domain validation error", func(t *testing.T) {
		service := &topicServiceStub{createFieldFn: func(context.Context, uuid.UUID, usecase.CreateFieldInput) (*domain.TopicField, error) {
			return nil, domain.ErrTopicFieldInvalidType
		}}
		resp := topicRequest(t, newTopicTestApp(service), nethttp.MethodPost, "/api/v1/topics/"+topicID.String()+"/fields", []byte(`{"label":"x","type":"bad"}`))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body := decodeTopicBody[map[string]string](t, resp)
		if body["error"] != domain.ErrTopicFieldInvalidType.Error() {
			t.Errorf("response = %+v", body)
		}
	})
}

func TestTopicErrorMappingForEveryFieldValidationError(t *testing.T) {
	tests := []error{
		domain.ErrTopicFieldInvalidTopicUID,
		domain.ErrTopicFieldLabelRequired,
		domain.ErrTopicFieldInvalidType,
		domain.ErrTopicFieldInvalidPosition,
	}
	for _, wantErr := range tests {
		t.Run(wantErr.Error(), func(t *testing.T) {
			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error { return handleError(c, wantErr) })
			resp := topicRequest(t, app, nethttp.MethodGet, "/", nil)
			if resp.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			body := decodeTopicBody[map[string]string](t, resp)
			if body["error"] != wantErr.Error() {
				t.Errorf("response = %+v", body)
			}
		})
	}
}
