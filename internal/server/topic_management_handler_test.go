package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/google/uuid"
)

var (
	managementTopicUUID   = uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb40001")
	managementCreatorUUID = uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30006")
)

func TestManageTopicRequiresAdminHeadOrCreator(t *testing.T) {
	updateCalled := false
	service := fakeTopicService{
		get: func(_ context.Context, topicUUID uuid.UUID) (model.Topic, error) {
			return managedTopicFixture(topicUUID, managementCreatorUUID), nil
		},
		update: func(context.Context, uuid.UUID, topic.UpdateInput) (model.Topic, error) {
			updateCalled = true
			return model.Topic{}, nil
		},
	}
	app, _ := topicTestApp(service)
	target := "/api/topics/" + managementTopicUUID.String()
	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantCode   string
	}{
		{name: "missing session", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "teacher", token: "teacher-session", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "head of another division", token: "other-head-session", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := app.Test(managementRequest(http.MethodPatch, target, tt.token, `{"title":"แก้ไข"}`))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
			assertErrorResponse(t, res, tt.wantCode)
		})
	}
	if updateCalled {
		t.Fatal("update service was called before authorization")
	}
}

func TestTopicCreatorUpdatesTopic(t *testing.T) {
	var received topic.UpdateInput
	service := fakeTopicService{
		get: func(_ context.Context, topicUUID uuid.UUID) (model.Topic, error) {
			return managedTopicFixture(topicUUID, managementCreatorUUID), nil
		},
		update: func(_ context.Context, topicUUID uuid.UUID, input topic.UpdateInput) (model.Topic, error) {
			received = input
			updated := managedTopicFixture(topicUUID, managementCreatorUUID)
			updated.Title = *input.Title
			return updated, nil
		},
	}
	app, _ := topicTestApp(service)
	existingFieldUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb40003")
	body := `{
		"title":"หัวข้อฉบับแก้ไข",
		"is_secret":false,
		"fields":[
			{"uuid":"` + existingFieldUUID.String() + `","position":1,"type":"text","label":"ชื่อกิจกรรม","required":false},
			{"position":2,"type":"choice","label":"ระดับชั้น","options":["ม.1","ม.2"]}
		]
	}`
	res, err := app.Test(managementRequest(http.MethodPatch, "/api/topics/"+managementTopicUUID.String(), "creator-session", body))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if received.Title == nil || *received.Title != "หัวข้อฉบับแก้ไข" || received.IsSecret == nil || *received.IsSecret {
		t.Fatalf("Update input = %+v", received)
	}
	if received.Fields == nil || len(*received.Fields) != 2 {
		t.Fatalf("fields = %+v, want full form", received.Fields)
	}
	if (*received.Fields)[0].UUID != existingFieldUUID || (*received.Fields)[0].Required == nil || *(*received.Fields)[0].Required {
		t.Fatalf("existing field = %+v", (*received.Fields)[0])
	}
	if (*received.Fields)[1].UUID != uuid.Nil || (*received.Fields)[1].Required != nil {
		t.Fatalf("new field = %+v", (*received.Fields)[1])
	}
}

func TestAdminClosesReopensAndDeletesTopic(t *testing.T) {
	service := fakeTopicService{
		get: func(_ context.Context, topicUUID uuid.UUID) (model.Topic, error) {
			return managedTopicFixture(topicUUID, managementCreatorUUID), nil
		},
		setOpen: func(_ context.Context, topicUUID uuid.UUID, open bool) (model.Topic, error) {
			value := managedTopicFixture(topicUUID, managementCreatorUUID)
			value.IsOpen = open
			if !open {
				closedAt := time.Date(2026, time.October, 13, 9, 0, 0, 0, time.UTC)
				value.ClosedAt = &closedAt
			}
			return value, nil
		},
		delete: func(context.Context, uuid.UUID) error { return nil },
	}
	app, _ := topicTestApp(service)
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantOpen   *bool
	}{
		{name: "close", method: http.MethodPost, path: "/close", wantStatus: http.StatusOK, wantOpen: boolPointer(false)},
		{name: "reopen", method: http.MethodPost, path: "/reopen", wantStatus: http.StatusOK, wantOpen: boolPointer(true)},
		{name: "delete", method: http.MethodDelete, wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := "/api/topics/" + managementTopicUUID.String() + tt.path
			res, err := app.Test(managementRequest(tt.method, target, "admin-session", ""))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
			if tt.wantOpen != nil {
				var response topicResponse
				if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.IsOpen != *tt.wantOpen {
					t.Fatalf("is_open = %v, want %v", response.IsOpen, *tt.wantOpen)
				}
			}
		})
	}
}

func TestTopicManagementConflictsUseStableCodes(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		service  fakeTopicService
		wantCode string
	}{
		{
			name:   "field type is locked",
			method: http.MethodPatch,
			service: fakeTopicService{
				get: func(_ context.Context, topicUUID uuid.UUID) (model.Topic, error) {
					return managedTopicFixture(topicUUID, managementCreatorUUID), nil
				},
				update: func(context.Context, uuid.UUID, topic.UpdateInput) (model.Topic, error) {
					return model.Topic{}, topic.ErrFieldTypeLocked
				},
			},
			wantCode: "TOPIC_FIELD_TYPE_LOCKED",
		},
		{
			name:   "topic has submissions",
			method: http.MethodDelete,
			service: fakeTopicService{
				get: func(_ context.Context, topicUUID uuid.UUID) (model.Topic, error) {
					return managedTopicFixture(topicUUID, managementCreatorUUID), nil
				},
				delete: func(context.Context, uuid.UUID) error { return topic.ErrTopicHasSubmissions },
			},
			wantCode: "TOPIC_HAS_SUBMISSIONS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := topicTestApp(tt.service)
			body := ""
			if tt.method == http.MethodPatch {
				body = `{"title":"แก้ไข"}`
			}
			res, err := app.Test(managementRequest(tt.method, "/api/topics/"+managementTopicUUID.String()+tt.path, "admin-session", body))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want 409", res.StatusCode)
			}
			assertErrorResponse(t, res, tt.wantCode)
		})
	}
}

func managementRequest(method, target, token, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "sdms_session", Value: token})
	}
	return req
}

func managedTopicFixture(topicUUID, creatorUUID uuid.UUID) model.Topic {
	value := createdTopicFixture(topic.CreateInput{
		DivisionUUID:  uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30001"),
		CreatedByUUID: creatorUUID,
		Title:         "หัวข้อเดิม",
	})
	value.UUID = topicUUID
	return value
}

func boolPointer(value bool) *bool { return &value }
