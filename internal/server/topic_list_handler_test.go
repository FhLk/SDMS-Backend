package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/google/uuid"
)

func TestListDivisionsRequiresAuthenticationAndReturnsData(t *testing.T) {
	called := false
	service := fakeTopicService{
		listDivisions: func(context.Context) ([]model.Division, error) {
			called = true
			return []model.Division{{
				UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb60001")},
				Code:      "aca",
				Name:      "ฝ่ายบริหารวิชาการ",
				ShortName: "วิชาการ",
				SortOrder: 1,
			}}, nil
		},
	}
	app, _ := topicTestApp(service)

	unauthorized, err := app.Test(managementRequest(http.MethodGet, "/api/divisions", "", ""))
	if err != nil {
		t.Fatalf("unauthorized request: %v", err)
	}
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized || called {
		t.Fatalf("unauthorized status = %d, service called = %v", unauthorized.StatusCode, called)
	}

	res, err := app.Test(managementRequest(http.MethodGet, "/api/divisions", "teacher-session", ""))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !called {
		t.Fatalf("status = %d, service called = %v", res.StatusCode, called)
	}
	var response []topicListDivisionResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 || response[0].Code != "aca" || response[0].SortOrder != 1 {
		t.Fatalf("response = %+v", response)
	}
}

func TestListDivisionTopicsUsesSessionUserAndOptionalYear(t *testing.T) {
	status := topic.StatusPending
	var receivedYear *int16
	var receivedUserUUID uuid.UUID
	service := fakeTopicService{
		listDivisionTopics: func(_ context.Context, code string, year *int16, userUUID uuid.UUID) (topic.DivisionTopics, error) {
			if code != "aca" {
				t.Fatalf("code = %q, want aca", code)
			}
			receivedYear = year
			receivedUserUUID = userUUID
			return topic.DivisionTopics{
				Division: model.Division{
					UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb60002")},
					Code:      "aca",
					Name:      "ฝ่ายบริหารวิชาการ",
					ShortName: "วิชาการ",
					SortOrder: 1,
				},
				AcademicYear: model.AcademicYear{
					UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb60003")},
					Year:      2569,
					IsCurrent: true,
				},
				Topics: []topic.ListItem{{
					UUID:           uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb60004"),
					Semester:       1,
					Title:          "หัวข้อทดสอบ",
					DueDate:        time.Date(2026, time.October, 31, 0, 0, 0, 0, time.UTC),
					IsOpen:         true,
					SubmittedCount: 2,
					AssigneeCount:  5,
					IsAssigned:     true,
					MyStatus:       &status,
				}},
			}, nil
		},
	}
	app, _ := topicTestApp(service)

	res, err := app.Test(managementRequest(http.MethodGet, "/api/divisions/aca/topics", "teacher-session", ""))
	if err != nil {
		t.Fatalf("app.Test(current year) error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || receivedYear != nil || receivedUserUUID == uuid.Nil {
		t.Fatalf("status = %d, year = %v, user = %s", res.StatusCode, receivedYear, receivedUserUUID)
	}
	var response divisionTopicsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Topics) != 1 || response.Topics[0].SubmittedCount != 2 || response.Topics[0].AssigneeCount != 5 || response.Topics[0].MyStatus == nil || *response.Topics[0].MyStatus != topic.StatusPending {
		t.Fatalf("response = %+v", response)
	}

	yearResponse, err := app.Test(managementRequest(http.MethodGet, "/api/divisions/aca/topics?year=2568", "teacher-session", ""))
	if err != nil {
		t.Fatalf("app.Test(explicit year) error = %v", err)
	}
	defer yearResponse.Body.Close()
	if yearResponse.StatusCode != http.StatusOK || receivedYear == nil || *receivedYear != 2568 {
		t.Fatalf("status = %d, year = %v", yearResponse.StatusCode, receivedYear)
	}
}

func TestListDivisionTopicsRejectsInvalidYear(t *testing.T) {
	called := false
	service := fakeTopicService{
		listDivisionTopics: func(context.Context, string, *int16, uuid.UUID) (topic.DivisionTopics, error) {
			called = true
			return topic.DivisionTopics{}, nil
		},
	}
	app, _ := topicTestApp(service)
	res, err := app.Test(managementRequest(http.MethodGet, "/api/divisions/aca/topics?year=not-a-year", "teacher-session", ""))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest || called {
		t.Fatalf("status = %d, service called = %v", res.StatusCode, called)
	}
	var response struct {
		Details struct {
			Field string `json:"field"`
		} `json:"details"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Details.Field != "year" {
		t.Fatalf("details.field = %q, want year", response.Details.Field)
	}
}
