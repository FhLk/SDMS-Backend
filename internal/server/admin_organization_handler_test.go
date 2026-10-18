package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeAdminOrganizationService struct {
	listDivisions          func(context.Context) ([]model.Division, error)
	setDivisionHead        func(context.Context, string, uuid.UUID) (model.Division, error)
	listAcademicYears      func(context.Context) ([]admin.AcademicYearSummary, error)
	createAcademicYear     func(context.Context, admin.CreateAcademicYearInput) (admin.AcademicYearSummary, error)
	setCurrentAcademicYear func(context.Context, admin.SetCurrentAcademicYearInput) (admin.AcademicYearSummary, error)
}

func (f fakeAdminOrganizationService) ListDivisions(ctx context.Context) ([]model.Division, error) {
	return f.listDivisions(ctx)
}

func (f fakeAdminOrganizationService) SetDivisionHead(ctx context.Context, code string, userUUID uuid.UUID) (model.Division, error) {
	return f.setDivisionHead(ctx, code, userUUID)
}

func TestAdminListsDivisionsWithHeads(t *testing.T) {
	headUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fa00001")
	service := fakeAdminOrganizationService{
		listDivisions: func(context.Context) ([]model.Division, error) {
			return []model.Division{
				{
					Code:      "aca",
					Name:      "ฝ่ายวิชาการ",
					ShortName: "วิชาการ",
					SortOrder: 1,
					HeadUser: &model.User{
						UUIDModel: model.UUIDModel{UUID: headUUID},
						Username:  "head.aca",
						Prefix:    "นางสาว",
						FirstName: "อรทัย",
						LastName:  "แสงวิชา",
						Position:  "หัวหน้าฝ่ายวิชาการ",
					},
				},
				{Code: "bud", Name: "ฝ่ายงบประมาณ", ShortName: "งบประมาณ", SortOrder: 2},
			}, nil
		},
	}
	app := adminOrganizationTestApp(service)
	req := adminRequest(http.MethodGet, "/api/admin/divisions", "")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body []adminDivisionResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("body length = %d, want 2", len(body))
	}
	if body[0].Code != "aca" || body[0].HeadUser == nil || body[0].HeadUser.UUID != headUUID {
		t.Fatalf("first division = %+v, want academic division with its head", body[0])
	}
	if body[0].HeadUser.FullName != "นางสาวอรทัย แสงวิชา" {
		t.Fatalf("head full_name = %q", body[0].HeadUser.FullName)
	}
	if body[1].Code != "bud" || body[1].HeadUser != nil {
		t.Fatalf("second division = %+v, want budget division without a head", body[1])
	}
}

func (f fakeAdminOrganizationService) ListAcademicYears(ctx context.Context) ([]admin.AcademicYearSummary, error) {
	return f.listAcademicYears(ctx)
}

func (f fakeAdminOrganizationService) CreateAcademicYear(ctx context.Context, input admin.CreateAcademicYearInput) (admin.AcademicYearSummary, error) {
	return f.createAcademicYear(ctx, input)
}

func (f fakeAdminOrganizationService) SetCurrentAcademicYear(ctx context.Context, input admin.SetCurrentAcademicYearInput) (admin.AcademicYearSummary, error) {
	return f.setCurrentAcademicYear(ctx, input)
}

func TestAdminOrganizationRequiresAdminRole(t *testing.T) {
	called := false
	service := fakeAdminOrganizationService{
		listAcademicYears: func(context.Context) ([]admin.AcademicYearSummary, error) {
			called = true
			return nil, nil
		},
	}
	app := adminOrganizationTestApp(service)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/years", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "teacher-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "FORBIDDEN")
	if called {
		t.Fatal("organization service was called before authorization")
	}
}

func TestAdminSetsDivisionHead(t *testing.T) {
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fa10001")
	divisionUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fa10002")
	service := fakeAdminOrganizationService{
		setDivisionHead: func(_ context.Context, code string, gotUserUUID uuid.UUID) (model.Division, error) {
			if code != "aca" || gotUserUUID != userUUID {
				t.Fatalf("SetDivisionHead(%q, %s), want aca and %s", code, gotUserUUID, userUUID)
			}
			return model.Division{
				UUIDModel: model.UUIDModel{UUID: divisionUUID},
				Code:      "aca",
				Name:      "ฝ่ายวิชาการ",
				ShortName: "วิชาการ",
				SortOrder: 1,
				HeadUser: &model.User{
					UUIDModel: model.UUIDModel{UUID: userUUID},
					Username:  "teacher.one",
					FullName:  "ครู หนึ่ง",
					Position:  "ครู",
				},
			}, nil
		},
	}
	app := adminOrganizationTestApp(service)
	req := adminRequest(http.MethodPut, "/api/admin/divisions/aca/head", `{"user_uuid":"`+userUUID.String()+`"}`)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body adminDivisionResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "aca" || body.HeadUser == nil || body.HeadUser.UUID != userUUID {
		t.Fatalf("body = %+v, want academic division with selected head", body)
	}
}

func TestAdminRejectsExecutiveAsDivisionHead(t *testing.T) {
	service := fakeAdminOrganizationService{
		setDivisionHead: func(context.Context, string, uuid.UUID) (model.Division, error) {
			return model.Division{}, admin.ErrExecutiveCannotHead
		},
	}
	app := adminOrganizationTestApp(service)
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fa20001")
	req := adminRequest(http.MethodPut, "/api/admin/divisions/aca/head", `{"user_uuid":"`+userUUID.String()+`"}`)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "EXECUTIVE_CANNOT_BE_DIVISION_HEAD")
}

func TestAdminListsAcademicYearsWithTopicCounts(t *testing.T) {
	service := fakeAdminOrganizationService{
		listAcademicYears: func(context.Context) ([]admin.AcademicYearSummary, error) {
			return []admin.AcademicYearSummary{{Year: 2569, IsCurrent: true, CurrentSemester: 1, TopicCount: 4}}, nil
		},
	}
	app := adminOrganizationTestApp(service)
	req := adminRequest(http.MethodGet, "/api/admin/years", "")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body []admin.AcademicYearSummary
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 || body[0].TopicCount != 4 || !body[0].IsCurrent {
		t.Fatalf("body = %+v, want current year with four topics", body)
	}
}

func TestAuthenticatedUserListsAcademicYearsWithoutAdminMetadata(t *testing.T) {
	yearUUID := uuid.New()
	service := fakeAdminOrganizationService{
		listAcademicYears: func(context.Context) ([]admin.AcademicYearSummary, error) {
			return []admin.AcademicYearSummary{{
				UUID: yearUUID, Year: 2569, IsCurrent: true, CurrentSemester: 2,
				TopicCount: 4,
			}}, nil
		},
	}
	app := adminOrganizationTestApp(service)
	req := httptest.NewRequest(http.MethodGet, "/api/years", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "teacher-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 || body[0]["uuid"] != yearUUID.String() || body[0]["year"] != float64(2569) || body[0]["current_semester"] != float64(2) {
		t.Fatalf("body = %+v", body)
	}
	if _, exists := body[0]["topic_count"]; exists {
		t.Fatal("public academic year response contains topic_count")
	}
	if _, exists := body[0]["created_at"]; exists {
		t.Fatal("public academic year response contains created_at")
	}
}

func TestAdminCreatesAndSetsCurrentAcademicYear(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		service := fakeAdminOrganizationService{
			createAcademicYear: func(_ context.Context, input admin.CreateAcademicYearInput) (admin.AcademicYearSummary, error) {
				if input.Year != 2570 || input.CurrentSemester == nil || *input.CurrentSemester != 1 {
					t.Fatalf("CreateAcademicYear input = %+v", input)
				}
				return admin.AcademicYearSummary{Year: input.Year, CurrentSemester: *input.CurrentSemester}, nil
			},
		}
		app := adminOrganizationTestApp(service)
		req := adminRequest(http.MethodPost, "/api/admin/years", `{"year":2570,"current_semester":1}`)

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test() error = %v", err)
		}
		defer res.Body.Close()
		if got, want := res.StatusCode, http.StatusCreated; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run("set current", func(t *testing.T) {
		service := fakeAdminOrganizationService{
			setCurrentAcademicYear: func(_ context.Context, input admin.SetCurrentAcademicYearInput) (admin.AcademicYearSummary, error) {
				if input.Year != 2570 || input.CurrentSemester == nil || *input.CurrentSemester != 2 {
					t.Fatalf("SetCurrentAcademicYear input = %+v", input)
				}
				return admin.AcademicYearSummary{Year: input.Year, IsCurrent: true, CurrentSemester: *input.CurrentSemester}, nil
			},
		}
		app := adminOrganizationTestApp(service)
		req := adminRequest(http.MethodPost, "/api/admin/years/2570/current", `{"current_semester":2}`)

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test() error = %v", err)
		}
		defer res.Body.Close()
		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		var body admin.AcademicYearSummary
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !body.IsCurrent || body.CurrentSemester != 2 {
			t.Fatalf("body = %+v, want current second semester", body)
		}
	})

	t.Run("set current preserves semester when body is empty", func(t *testing.T) {
		service := fakeAdminOrganizationService{
			setCurrentAcademicYear: func(_ context.Context, input admin.SetCurrentAcademicYearInput) (admin.AcademicYearSummary, error) {
				if input.Year != 2570 || input.CurrentSemester != nil {
					t.Fatalf("SetCurrentAcademicYear input = %+v, want nil semester", input)
				}
				return admin.AcademicYearSummary{Year: input.Year, IsCurrent: true, CurrentSemester: 2}, nil
			},
		}
		app := adminOrganizationTestApp(service)
		req := adminRequest(http.MethodPost, "/api/admin/years/2570/current", "")

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test() error = %v", err)
		}
		defer res.Body.Close()
		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})
}

func adminOrganizationTestApp(service adminOrganizationService) *fiber.App {
	adminUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fa30001")
	teacherUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fa30002")
	authService := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			switch token {
			case "admin-session":
				return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: adminUUID}, IsAdmin: true}}, nil
			case "teacher-session":
				return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: teacherUUID}}}, nil
			default:
				return auth.Profile{}, auth.ErrUnauthenticated
			}
		},
	}
	return New(Dependencies{
		Auth:              authService,
		AuthConfig:        testAuthConfig(),
		OrganizationAdmin: service,
	})
}
