package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/portfolio"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakePortfolioService struct {
	list    func(context.Context, portfolio.ListInput) (portfolio.Portfolio, error)
	archive func(context.Context, portfolio.ArchiveInput) (portfolio.Archive, error)
}

func (f fakePortfolioService) List(ctx context.Context, input portfolio.ListInput) (portfolio.Portfolio, error) {
	return f.list(ctx, input)
}

func (f fakePortfolioService) OpenArchive(ctx context.Context, input portfolio.ArchiveInput) (portfolio.Archive, error) {
	return f.archive(ctx, input)
}

func TestPortfolioListUsesAuthenticatedUserAndReturnsYearGroups(t *testing.T) {
	userUUID := uuid.New()
	submissionUUID := uuid.New()
	fileUUID := uuid.New()
	fieldUUID := uuid.New()
	division := model.Division{
		UUIDModel: model.UUIDModel{UUID: uuid.New()},
		Code:      "aca",
		Name:      "ฝ่ายวิชาการ",
		ShortName: "วิชาการ",
	}
	submittedAt := time.Date(2026, time.October, 13, 3, 0, 0, 0, time.UTC)
	service := fakePortfolioService{
		list: func(_ context.Context, input portfolio.ListInput) (portfolio.Portfolio, error) {
			if input.UserUUID != userUUID || input.Year == nil || *input.Year != 2569 || input.Query != "ภาษาไทย" {
				t.Fatalf("List input = %+v", input)
			}
			return portfolio.Portfolio{Years: []portfolio.YearGroup{{
				AcademicYear: 2569,
				Submissions: []portfolio.Submission{{
					UUID: submissionUUID, TopicUUID: uuid.New(), TopicTitle: "รายงานภาษาไทย",
					Division: division, Semester: 1, DueDate: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC),
					Values: json.RawMessage(`{"note":"ข้อมูล"}`), Status: portfolio.StatusSubmitted,
					SubmittedAt: submittedAt, UpdatedAt: submittedAt,
					Files: []portfolio.File{{
						UUID: fileUUID, FieldUUID: fieldUUID, OriginalName: "หลักฐาน.pdf",
						MimeType: "application/pdf", SizeBytes: 7, CreatedAt: submittedAt,
					}},
				}},
			}}}, nil
		},
	}
	app := portfolioTestApp(service, userUUID)
	req := httptest.NewRequest(http.MethodGet, "/api/me/submissions?year=2569&q="+url.QueryEscape("ภาษาไทย"), nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "portfolio-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var response portfolioResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Years) != 1 || response.Years[0].AcademicYear != 2569 || len(response.Years[0].Submissions) != 1 {
		t.Fatalf("response years = %+v", response.Years)
	}
	item := response.Years[0].Submissions[0]
	if item.UUID != submissionUUID || item.TopicTitle != "รายงานภาษาไทย" || len(item.Files) != 1 || item.Files[0].UUID != fileUUID {
		t.Fatalf("submission response = %+v", item)
	}
}

func TestPortfolioArchiveStreamsZIPWithDownloadHeaders(t *testing.T) {
	userUUID := uuid.New()
	content := []byte("zip-stream")
	service := fakePortfolioService{
		archive: func(_ context.Context, input portfolio.ArchiveInput) (portfolio.Archive, error) {
			if input.UserUUID != userUUID || input.Year == nil || *input.Year != 2569 {
				t.Fatalf("OpenArchive input = %+v", input)
			}
			return portfolio.Archive{
				Filename: "sdms-files-2569.zip",
				Content:  io.NopCloser(bytes.NewReader(content)),
			}, nil
		},
	}
	app := portfolioTestApp(service, userUUID)
	req := httptest.NewRequest(http.MethodGet, "/api/me/files.zip?year=2569", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "portfolio-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q, want application/zip", got)
	}
	disposition, parameters, err := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parse Content-Disposition: %v", err)
	}
	if disposition != "attachment" || parameters["filename"] != "sdms-files-2569.zip" {
		t.Fatalf("Content-Disposition = %q/%q", disposition, parameters["filename"])
	}
	if got := res.Header.Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !bytes.Equal(body, content) {
		t.Fatalf("body = %q, want %q", body, content)
	}
}

func TestPortfolioRejectsInvalidYearBeforeCallingService(t *testing.T) {
	called := false
	service := fakePortfolioService{
		list: func(context.Context, portfolio.ListInput) (portfolio.Portfolio, error) {
			called = true
			return portfolio.Portfolio{}, nil
		},
	}
	app := portfolioTestApp(service, uuid.New())
	req := httptest.NewRequest(http.MethodGet, "/api/me/submissions?year=invalid", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "portfolio-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if called {
		t.Fatal("portfolio service was called for an invalid year")
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["code"] != "VALIDATION_ERROR" {
		t.Fatalf("error code = %v", body["code"])
	}
}

func portfolioTestApp(service portfolioService, userUUID uuid.UUID) *fiber.App {
	authService := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "portfolio-session" {
				return auth.Profile{}, auth.ErrUnauthenticated
			}
			return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}}, nil
		},
	}
	return New(Dependencies{Auth: authService, AuthConfig: testAuthConfig(), Portfolio: service})
}
