package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/home"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeHomeService struct {
	get   func(context.Context, auth.Profile) (home.Dashboard, error)
	stats func(context.Context, string, auth.Profile) (home.DivisionStatistics, error)
}

func (f fakeHomeService) Get(ctx context.Context, profile auth.Profile) (home.Dashboard, error) {
	return f.get(ctx, profile)
}

func (f fakeHomeService) DivisionStats(ctx context.Context, code string, profile auth.Profile) (home.DivisionStatistics, error) {
	return f.stats(ctx, code, profile)
}

func TestHomeReturnsPersonalAndOverviewData(t *testing.T) {
	userUUID := uuid.New()
	division := model.Division{
		UUIDModel: model.UUIDModel{UUID: uuid.New()},
		Code:      "aca",
		Name:      "ฝ่ายวิชาการ",
		ShortName: "วิชาการ",
		SortOrder: 1,
	}
	submittedAt := time.Date(2026, time.October, 13, 3, 0, 0, 0, time.UTC)
	service := fakeHomeService{
		get: func(_ context.Context, profile auth.Profile) (home.Dashboard, error) {
			if profile.User.UUID != userUUID {
				t.Fatalf("profile user = %s, want %s", profile.User.UUID, userUUID)
			}
			return home.Dashboard{
				Personal: home.Personal{
					Counts: home.Counts{TopicCount: 3, RequiredCount: 3, SubmittedCount: 1, PendingCount: 1, OverdueCount: 1},
					PendingTopics: []home.PendingTopic{{
						UUID: uuid.New(), Title: "ส่งรายงาน", Division: division, AcademicYear: 2569,
						Semester: 1, DueDate: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC), Status: home.StatusPending,
					}},
					RecentSubmissions: []home.RecentSubmission{{
						UUID: uuid.New(), TopicUUID: uuid.New(), Title: "รายงานเดิม", Division: division,
						AcademicYear: 2569, Semester: 1, DueDate: time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC),
						Status: home.StatusLate, SubmittedAt: submittedAt, UpdatedAt: submittedAt,
					}},
					Divisions: []home.DivisionSummary{{Division: division, Counts: home.Counts{RequiredCount: 3}}},
				},
				Overview: &home.Overview{
					School:    home.Counts{RequiredCount: 10, SubmittedCount: 7, PendingCount: 2, OverdueCount: 1},
					Divisions: []home.DivisionSummary{{Division: division, Counts: home.Counts{RequiredCount: 3}}},
				},
			}, nil
		},
	}
	app := homeTestApp(service, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}})
	req := httptest.NewRequest(http.MethodGet, "/api/home", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "home-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var response homeResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.My.Counts.RequiredCount != 3 || len(response.My.PendingTopics) != 1 || len(response.My.RecentSubmissions) != 1 {
		t.Fatalf("personal response = %+v", response.My)
	}
	if response.Overview == nil || response.Overview.School.RequiredCount != 10 {
		t.Fatalf("overview response = %+v", response.Overview)
	}
}

func TestDivisionStatisticsUsesPathCodeAndMapsErrors(t *testing.T) {
	userUUID := uuid.New()
	division := model.Division{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Code: "aca"}
	success := fakeHomeService{
		stats: func(_ context.Context, code string, profile auth.Profile) (home.DivisionStatistics, error) {
			if code != "aca" || profile.User.UUID != userUUID {
				t.Fatalf("DivisionStats(%q, %s)", code, profile.User.UUID)
			}
			return home.DivisionStatistics{Division: division, Counts: home.Counts{TopicCount: 2, RequiredCount: 5}}, nil
		},
	}
	app := homeTestApp(success, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}})
	req := httptest.NewRequest(http.MethodGet, "/api/divisions/aca/stats", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "home-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var response divisionStatisticsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Division.Code != "aca" || response.Stats.RequiredCount != 5 {
		t.Fatalf("response = %+v", response)
	}

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{name: "forbidden", serviceErr: home.ErrForbidden, wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "missing", serviceErr: home.ErrDivisionNotFound, wantStatus: http.StatusNotFound, wantCode: "DIVISION_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := fakeHomeService{
				stats: func(context.Context, string, auth.Profile) (home.DivisionStatistics, error) {
					return home.DivisionStatistics{}, tt.serviceErr
				},
			}
			app := homeTestApp(service, auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}})
			req := httptest.NewRequest(http.MethodGet, "/api/divisions/aca/stats", nil)
			req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "home-session"})
			res, err := app.Test(req)
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
}

func homeTestApp(service homeService, profile auth.Profile) *fiber.App {
	authService := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "home-session" {
				return auth.Profile{}, auth.ErrUnauthenticated
			}
			return profile, nil
		},
	}
	return New(Dependencies{Auth: authService, AuthConfig: testAuthConfig(), Home: service})
}
