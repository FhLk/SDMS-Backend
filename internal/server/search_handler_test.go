package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	searchdomain "github.com/FhLk/SDMS-Backend/internal/search"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeSearchService struct {
	search func(context.Context, searchdomain.Input) (searchdomain.Result, error)
}

func (f fakeSearchService) Search(ctx context.Context, input searchdomain.Input) (searchdomain.Result, error) {
	return f.search(ctx, input)
}

func TestSearchParsesFiltersAndReturnsPagination(t *testing.T) {
	viewerUUID := uuid.New()
	byUUID := uuid.New()
	submissionUUID := uuid.New()
	topicUUID := uuid.New()
	fileUUID := uuid.New()
	fieldUUID := uuid.New()
	division := model.Division{
		UUIDModel: model.UUIDModel{UUID: uuid.New()},
		Code:      "aca",
		Name:      "ฝ่ายวิชาการ",
		ShortName: "วิชาการ",
	}
	submittedAt := time.Date(2026, time.October, 13, 3, 0, 0, 0, time.UTC)
	service := fakeSearchService{
		search: func(_ context.Context, input searchdomain.Input) (searchdomain.Result, error) {
			if input.Profile.User.UUID != viewerUUID || input.Query != "ภาษาไทย" || input.Division != "aca" {
				t.Fatalf("search identity/query input = %+v", input)
			}
			if input.Year == nil || *input.Year != 2569 || input.Semester == nil || *input.Semester != 2 {
				t.Fatalf("search year/semester input = %+v", input)
			}
			if input.By == nil || *input.By != byUUID || input.Page != 2 || input.PageSize != 10 {
				t.Fatalf("search by/pagination input = %+v", input)
			}
			return searchdomain.Result{
				Items: []searchdomain.Item{{
					SubmissionUUID: submissionUUID,
					Topic: searchdomain.Topic{
						UUID: topicUUID, Title: "หัวข้อภาษาไทย", Division: division,
						AcademicYear: 2569, Semester: 2,
						DueDate: time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC),
					},
					SubmittedBy: model.User{
						UUIDModel: model.UUIDModel{UUID: byUUID},
						Username:  "teacher.one",
						Prefix:    "นาย",
						FirstName: "ครู",
						LastName:  "หนึ่ง",
						Position:  "ครู",
					},
					Values: json.RawMessage(`{"note":"ข้อมูล"}`),
					Files: []searchdomain.File{{
						UUID: fileUUID, FieldUUID: fieldUUID, OriginalName: "หลักฐาน.pdf",
						MimeType: "application/pdf", SizeBytes: 7, CreatedAt: submittedAt,
					}},
					Status: searchdomain.StatusLate, SubmittedAt: submittedAt, UpdatedAt: submittedAt,
				}},
				Page: 2, PageSize: 10, Total: 11, TotalPages: 2,
			}, nil
		},
	}
	app := searchTestApp(service, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: viewerUUID}}})
	path := "/api/search?q=" + url.QueryEscape("ภาษาไทย") + "&division=aca&year=2569&semester=2&by=" + byUUID.String() + "&page=2&page_size=10"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "search-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var response searchResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].SubmissionUUID != submissionUUID || response.Items[0].Topic.UUID != topicUUID {
		t.Fatalf("items = %+v", response.Items)
	}
	if got := response.Items[0].SubmittedBy; got.Prefix != "นาย" || got.FirstName != "ครู" || got.LastName != "หนึ่ง" || got.FullName != "นายครู หนึ่ง" {
		t.Fatalf("submitted_by = %+v, want split fields and computed full_name", got)
	}
	if len(response.Items[0].Files) != 1 || response.Items[0].Files[0].UUID != fileUUID {
		t.Fatalf("files = %+v", response.Items[0].Files)
	}
	if response.Pagination.Page != 2 || response.Pagination.PageSize != 10 || response.Pagination.Total != 11 || response.Pagination.TotalPages != 2 {
		t.Fatalf("pagination = %+v", response.Pagination)
	}
}

func TestSearchUsesPaginationDefaults(t *testing.T) {
	called := false
	service := fakeSearchService{
		search: func(_ context.Context, input searchdomain.Input) (searchdomain.Result, error) {
			called = true
			if input.Page != 1 || input.PageSize != searchdomain.DefaultPageSize {
				t.Fatalf("pagination defaults = %d/%d", input.Page, input.PageSize)
			}
			return searchdomain.Result{Items: make([]searchdomain.Item, 0), Page: 1, PageSize: searchdomain.DefaultPageSize}, nil
		},
	}
	app := searchTestApp(service, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: uuid.New()}}})
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "search-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !called {
		t.Fatalf("status/called = %d/%v, want 200/true", res.StatusCode, called)
	}
}

func TestSearchRejectsInvalidQueryParametersBeforeService(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "invalid submitter", path: "/api/search?by=invalid"},
		{name: "invalid page", path: "/api/search?page=0"},
		{name: "oversized page", path: "/api/search?page_size=101"},
		{name: "invalid semester number", path: "/api/search?semester=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			service := fakeSearchService{
				search: func(context.Context, searchdomain.Input) (searchdomain.Result, error) {
					called = true
					return searchdomain.Result{}, nil
				},
			}
			app := searchTestApp(service, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: uuid.New()}}})
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "search-session"})
			res, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", res.StatusCode)
			}
			if called {
				t.Fatal("search service was called for invalid query parameters")
			}
			var body map[string]any
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body["code"] != "VALIDATION_ERROR" {
				t.Fatalf("error code = %v", body["code"])
			}
		})
	}
}

func searchTestApp(service searchService, profile auth.Profile) *fiber.App {
	authService := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "search-session" {
				return auth.Profile{}, auth.ErrUnauthenticated
			}
			return profile, nil
		},
	}
	return New(Dependencies{Auth: authService, AuthConfig: testAuthConfig(), Search: service})
}
