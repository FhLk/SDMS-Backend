package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeTopicService struct {
	create func(context.Context, topic.CreateInput) (model.Topic, error)
}

func (f fakeTopicService) Create(ctx context.Context, input topic.CreateInput) (model.Topic, error) {
	return f.create(ctx, input)
}

func TestCreateTopicRequiresDivisionHeadOrAdmin(t *testing.T) {
	called := false
	service := fakeTopicService{
		create: func(context.Context, topic.CreateInput) (model.Topic, error) {
			called = true
			return model.Topic{}, nil
		},
	}
	app, divisionUUID := topicTestApp(service)
	body := validCreateTopicBody(divisionUUID)
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
			req := topicRequest("/api/topics", tt.token, body)
			res, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if got := res.StatusCode; got != tt.wantStatus {
				t.Fatalf("status = %d, want %d", got, tt.wantStatus)
			}
			assertErrorResponse(t, res, tt.wantCode)
		})
	}
	if called {
		t.Fatal("topic service was called before authorization")
	}
}

func TestDivisionHeadCreatesTopic(t *testing.T) {
	var received topic.CreateInput
	service := fakeTopicService{
		create: func(_ context.Context, input topic.CreateInput) (model.Topic, error) {
			received = input
			return createdTopicFixture(input), nil
		},
	}
	app, divisionUUID := topicTestApp(service)
	body := `{
		"division_uuid":"` + divisionUUID.String() + `",
		"title":"รายงานประจำเดือน",
		"due_date":"2026-10-31",
		"is_secret":true,
		"fields":[
			{"position":2,"type":"choice","label":"ระดับชั้น","options":["ม.1","ม.2"]},
			{"position":1,"type":"text","label":"ชื่อกิจกรรม","required":false}
		]
	}`
	req := topicRequest("/api/topics", "head-session", body)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if received.DivisionUUID != divisionUUID || received.Title != "รายงานประจำเดือน" || !received.IsSecret {
		t.Fatalf("Create input = %+v", received)
	}
	if len(received.Fields) != 2 || !received.Fields[0].Required || received.Fields[1].Required {
		t.Fatalf("required defaults = %+v, want default true then explicit false", received.Fields)
	}

	var response topicResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Division.UUID != divisionUUID || response.AcademicYear.Year != 2569 || response.Semester != 1 {
		t.Fatalf("period/division response = %+v", response)
	}
	if response.DueDate != "2026-10-31" || !response.IsOpen || !response.IsSecret || response.AssigneeCount != 2 {
		t.Fatalf("topic response = %+v", response)
	}
	if len(response.Fields) != 2 || response.Fields[0].Position != 1 || len(response.Fields[1].Options) != 2 {
		t.Fatalf("fields response = %+v", response.Fields)
	}
}

func TestAdminCreatesTopicForAnyDivision(t *testing.T) {
	called := false
	service := fakeTopicService{
		create: func(_ context.Context, input topic.CreateInput) (model.Topic, error) {
			called = true
			return createdTopicFixture(input), nil
		},
	}
	app, divisionUUID := topicTestApp(service)
	req := topicRequest("/api/topics", "admin-session", validCreateTopicBody(divisionUUID))

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if !called {
		t.Fatal("admin request did not reach topic service")
	}
}

func TestCreateTopicValidationReturnsFieldPath(t *testing.T) {
	service := fakeTopicService{
		create: func(context.Context, topic.CreateInput) (model.Topic, error) {
			return model.Topic{}, &admin.ValidationError{
				Field:   "fields[1].options",
				Message: "choice field requires at least two options",
			}
		},
	}
	app, divisionUUID := topicTestApp(service)
	req := topicRequest("/api/topics", "admin-session", validCreateTopicBody(divisionUUID))

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var response struct {
		Code    string `json:"code"`
		Details struct {
			Field string `json:"field"`
		} `json:"details"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != "VALIDATION_ERROR" || response.Details.Field != "fields[1].options" {
		t.Fatalf("response = %+v, want exact invalid field path", response)
	}
}

func topicTestApp(service topicService) (*fiber.App, uuid.UUID) {
	divisionUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30001")
	otherDivisionUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30002")
	adminUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30003")
	headUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30004")
	teacherUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb30005")
	authService := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			switch token {
			case "admin-session":
				return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: adminUUID}, IsAdmin: true}}, nil
			case "head-session":
				return auth.Profile{
					User:          model.User{UUIDModel: model.UUIDModel{UUID: headUUID}},
					HeadDivisions: []model.Division{{UUIDModel: model.UUIDModel{UUID: divisionUUID}}},
				}, nil
			case "other-head-session":
				return auth.Profile{
					User:          model.User{UUIDModel: model.UUIDModel{UUID: headUUID}},
					HeadDivisions: []model.Division{{UUIDModel: model.UUIDModel{UUID: otherDivisionUUID}}},
				}, nil
			case "teacher-session":
				return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: teacherUUID}}}, nil
			default:
				return auth.Profile{}, auth.ErrUnauthenticated
			}
		},
	}
	return New(Dependencies{Auth: authService, AuthConfig: testAuthConfig(), Topics: service}), divisionUUID
}

func validCreateTopicBody(divisionUUID uuid.UUID) string {
	return `{"division_uuid":"` + divisionUUID.String() + `","title":"หัวข้อ","due_date":"2026-10-31","fields":[{"type":"text","label":"รายละเอียด"}]}`
}

func topicRequest(target, token, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "sdms_session", Value: token})
	}
	return req
}

func createdTopicFixture(input topic.CreateInput) model.Topic {
	topicUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb40001")
	yearUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb40002")
	fieldOneUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb40003")
	fieldTwoUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb40004")
	now := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)
	return model.Topic{
		UUIDModel:        model.UUIDModel{UUID: topicUUID},
		DivisionUUID:     input.DivisionUUID,
		AcademicYearUUID: yearUUID,
		Semester:         1,
		Title:            input.Title,
		DueDate:          time.Date(2026, time.October, 31, 0, 0, 0, 0, time.UTC),
		IsOpen:           true,
		IsSecret:         input.IsSecret,
		CreatedByUUID:    input.CreatedByUUID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Division: model.Division{
			UUIDModel: model.UUIDModel{UUID: input.DivisionUUID},
			Code:      "aca",
			Name:      "ฝ่ายวิชาการ",
			ShortName: "วิชาการ",
		},
		AcademicYear: model.AcademicYear{UUIDModel: model.UUIDModel{UUID: yearUUID}, Year: 2569},
		Fields: []model.TopicField{
			{UUIDModel: model.UUIDModel{UUID: fieldOneUUID}, Position: 1, Type: model.FieldText, Label: "ชื่อกิจกรรม", Required: false, Options: datatypes.JSON([]byte(`[]`))},
			{UUIDModel: model.UUIDModel{UUID: fieldTwoUUID}, Position: 2, Type: model.FieldChoice, Label: "ระดับชั้น", Required: true, Options: datatypes.JSON([]byte(`["ม.1","ม.2"]`))},
		},
		Assignees: []model.TopicAssignee{{}, {}},
	}
}
