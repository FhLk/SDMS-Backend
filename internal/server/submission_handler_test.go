package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/submission"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestSubmissionHandlerUpsertUsesAuthenticatedUser(t *testing.T) {
	userUUID := uuid.New()
	topicUUID := uuid.New()
	fieldUUID := uuid.New()
	submissionUUID := uuid.New()
	dueDate := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	submittedAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)
	status := submission.StatusSubmitted
	service := &stubSubmissionService{}
	service.upsert = func(_ context.Context, input submission.UpsertInput) (submission.Detail, error) {
		if input.UserUUID != userUUID || input.TopicUUID != topicUUID {
			t.Fatalf("upsert identity = %s/%s", input.UserUUID, input.TopicUUID)
		}
		if got := string(input.Values[fieldUUID.String()]); got != `"คำตอบ"` {
			t.Fatalf("value = %s", got)
		}
		return submission.Detail{
			Topic: model.Topic{
				UUIDModel:    model.UUIDModel{UUID: topicUUID},
				Division:     model.Division{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Code: "aca", Name: "ฝ่ายวิชาการ", ShortName: "วิชาการ"},
				AcademicYear: model.AcademicYear{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Year: 2569},
				Title:        "แบบส่ง",
				DueDate:      dueDate,
				IsOpen:       true,
				Fields: []model.TopicField{{
					UUIDModel: model.UUIDModel{UUID: fieldUUID},
					Position:  1,
					Type:      model.FieldText,
					Label:     "รายละเอียด",
					Required:  true,
					Options:   datatypes.JSON([]byte(`[]`)),
				}},
			},
			Submission: &model.Submission{
				UUIDModel:   model.UUIDModel{UUID: submissionUUID},
				Values:      datatypes.JSON([]byte(`{"` + fieldUUID.String() + `":"คำตอบ"}`)),
				SubmittedAt: submittedAt,
				UpdatedAt:   submittedAt,
			},
			Files:  []model.File{},
			Status: &status,
		}, nil
	}

	app := newSubmissionHandlerTestApp(userUUID, service)
	body := []byte(`{"values":{"` + fieldUUID.String() + `":"คำตอบ"}}`)
	request := httptest.NewRequest(http.MethodPut, "/topics/"+topicUUID.String()+"/my-submission", bytes.NewReader(body))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var decoded mySubmissionResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.Submission == nil || decoded.Submission.UUID != submissionUUID || decoded.Submission.Status != submission.StatusSubmitted {
		t.Fatalf("submission response = %+v", decoded.Submission)
	}
}

func TestSubmissionHandlerReturnsAllFieldValidationDetails(t *testing.T) {
	userUUID := uuid.New()
	topicUUID := uuid.New()
	fieldUUID := uuid.New()
	service := &stubSubmissionService{
		upsert: func(context.Context, submission.UpsertInput) (submission.Detail, error) {
			return submission.Detail{}, &submission.ValidationError{Fields: []submission.FieldViolation{
				{Field: "values." + fieldUUID.String(), FieldUUID: &fieldUUID, Message: "field is required"},
				{Field: "values.other", Message: "field does not belong to this topic or is hidden"},
			}}
		},
	}
	app := newSubmissionHandlerTestApp(userUUID, service)
	request := httptest.NewRequest(http.MethodPut, "/topics/"+topicUUID.String()+"/my-submission", bytes.NewReader([]byte(`{"values":{}}`)))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
	var decoded struct {
		Code    string `json:"code"`
		Details struct {
			Fields []submission.FieldViolation `json:"fields"`
		} `json:"details"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if decoded.Code != "VALIDATION_ERROR" || len(decoded.Details.Fields) != 2 {
		t.Fatalf("error response = %+v", decoded)
	}
}

func TestSubmissionHandlerMapsNotAssignee(t *testing.T) {
	service := &stubSubmissionService{
		getMine: func(context.Context, uuid.UUID, uuid.UUID) (submission.Detail, error) {
			return submission.Detail{}, submission.ErrNotAssignee
		},
	}
	app := newSubmissionHandlerTestApp(uuid.New(), service)
	request := httptest.NewRequest(http.MethodGet, "/topics/"+uuid.NewString()+"/my-submission", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
	assertErrorResponse(t, response, "FORBIDDEN")
}

func TestSubmissionHandlerListsTopicTableWithFilter(t *testing.T) {
	userUUID := uuid.New()
	topicUUID := uuid.New()
	fieldUUID := uuid.New()
	submissionUUID := uuid.New()
	service := &stubSubmissionService{
		listTable: func(_ context.Context, gotTopicUUID uuid.UUID, profile auth.Profile, status string) (submission.TopicTable, error) {
			if gotTopicUUID != topicUUID || profile.User.UUID != userUUID || status != "done" {
				t.Fatalf("table input = %s/%s/%q", gotTopicUUID, profile.User.UUID, status)
			}
			return submission.TopicTable{
				Topic: model.Topic{
					UUIDModel:    model.UUIDModel{UUID: topicUUID},
					Division:     model.Division{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Code: "aca"},
					AcademicYear: model.AcademicYear{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Year: 2569},
					Title:        "ตารางคำตอบ",
					DueDate:      time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC),
					Fields: []model.TopicField{{
						UUIDModel: model.UUIDModel{UUID: fieldUUID},
						Position:  1,
						Type:      model.FieldText,
						Label:     "รายละเอียด",
						Required:  true,
					}},
				},
				Rows: []submission.TopicTableRow{{
					User:           model.User{UUIDModel: model.UUIDModel{UUID: userUUID}, Username: "teacher", FullName: "ครูทดสอบ", Position: "ครู"},
					SubmissionUUID: &submissionUUID,
					Values:         datatypes.JSON([]byte(`{"` + fieldUUID.String() + `":"ส่งแล้ว"}`)),
					Files:          []model.File{},
					Status:         submission.StatusSubmitted,
				}},
			}, nil
		},
	}
	app := newSubmissionHandlerTestApp(userUUID, service)
	request := httptest.NewRequest(http.MethodGet, "/topics/"+topicUUID.String()+"?status=done", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var decoded topicSubmissionTableResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode table response: %v", err)
	}
	if len(decoded.Rows) != 1 || decoded.Rows[0].SubmissionUUID == nil || *decoded.Rows[0].SubmissionUUID != submissionUUID {
		t.Fatalf("table response = %+v", decoded)
	}
}

func TestSubmissionHandlerPendingNamesForbidden(t *testing.T) {
	service := &stubSubmissionService{
		pendingNames: func(context.Context, uuid.UUID, auth.Profile) ([]model.User, error) {
			return nil, submission.ErrForbidden
		},
	}
	app := newSubmissionHandlerTestApp(uuid.New(), service)
	request := httptest.NewRequest(http.MethodGet, "/topics/"+uuid.NewString()+"/pending-names", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
	assertErrorResponse(t, response, "FORBIDDEN")
}

func newSubmissionHandlerTestApp(userUUID uuid.UUID, service submissionService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: handleError})
	app.Use(func(c fiber.Ctx) error {
		c.Locals(authenticatedProfileKey, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}})
		return c.Next()
	})
	handler := submissionHandler{service: service}
	app.Get("/me/pending", handler.listPending)
	app.Get("/topics/:uuid", handler.listTopicTable)
	app.Get("/topics/:uuid/my-submission", handler.getMine)
	app.Put("/topics/:uuid/my-submission", handler.upsert)
	app.Get("/topics/:uuid/pending-names", handler.listPendingNames)
	return app
}

type stubSubmissionService struct {
	listPending  func(context.Context, uuid.UUID) ([]submission.PendingItem, error)
	getMine      func(context.Context, uuid.UUID, uuid.UUID) (submission.Detail, error)
	upsert       func(context.Context, submission.UpsertInput) (submission.Detail, error)
	listTable    func(context.Context, uuid.UUID, auth.Profile, string) (submission.TopicTable, error)
	pendingNames func(context.Context, uuid.UUID, auth.Profile) ([]model.User, error)
}

func (s *stubSubmissionService) ListPending(ctx context.Context, userUUID uuid.UUID) ([]submission.PendingItem, error) {
	if s.listPending == nil {
		return nil, errors.New("unexpected ListPending call")
	}
	return s.listPending(ctx, userUUID)
}

func (s *stubSubmissionService) GetMine(ctx context.Context, topicUUID, userUUID uuid.UUID) (submission.Detail, error) {
	if s.getMine == nil {
		return submission.Detail{}, errors.New("unexpected GetMine call")
	}
	return s.getMine(ctx, topicUUID, userUUID)
}

func (s *stubSubmissionService) Upsert(ctx context.Context, input submission.UpsertInput) (submission.Detail, error) {
	if s.upsert == nil {
		return submission.Detail{}, errors.New("unexpected Upsert call")
	}
	return s.upsert(ctx, input)
}

func (s *stubSubmissionService) ListTopicTable(ctx context.Context, topicUUID uuid.UUID, profile auth.Profile, status string) (submission.TopicTable, error) {
	if s.listTable == nil {
		return submission.TopicTable{}, errors.New("unexpected ListTopicTable call")
	}
	return s.listTable(ctx, topicUUID, profile, status)
}

func (s *stubSubmissionService) ListPendingNames(ctx context.Context, topicUUID uuid.UUID, profile auth.Profile) ([]model.User, error) {
	if s.pendingNames == nil {
		return nil, errors.New("unexpected ListPendingNames call")
	}
	return s.pendingNames(ctx, topicUUID, profile)
}
