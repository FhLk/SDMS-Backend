package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

type fakeAuthService struct {
	login       func(context.Context, auth.LoginInput) (auth.LoginResult, error)
	logout      func(context.Context, string) error
	currentUser func(context.Context, string) (auth.Profile, error)
}

func (f fakeAuthService) Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error) {
	return f.login(ctx, input)
}

func (f fakeAuthService) Logout(ctx context.Context, token string) error {
	return f.logout(ctx, token)
}

func (f fakeAuthService) CurrentUser(ctx context.Context, token string) (auth.Profile, error) {
	return f.currentUser(ctx, token)
}

func TestLoginSetsSecureHTTPOnlySessionCookie(t *testing.T) {
	expiresAt := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	var received auth.LoginInput
	service := fakeAuthService{
		login: func(_ context.Context, input auth.LoginInput) (auth.LoginResult, error) {
			received = input
			return auth.LoginResult{Token: "raw-session-token", ExpiresAt: expiresAt}, nil
		},
	}
	app := New(Dependencies{
		Auth: service,
		AuthConfig: config.AuthConfig{
			SessionTTL:   2 * time.Hour,
			CookieName:   "school_session",
			CookieSecure: true,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"teacher.one","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "auth-test")
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if received.Username != "teacher.one" || received.Password != "secret" || received.UserAgent != "auth-test" {
		t.Fatalf("Login input = %+v, want request credentials and user agent", received)
	}
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want exactly one", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != "school_session" || cookie.Value != "raw-session-token" {
		t.Fatalf("cookie = %s=%s, want school_session=raw-session-token", cookie.Name, cookie.Value)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v Path:%q", cookie.HttpOnly, cookie.Secure, cookie.SameSite, cookie.Path)
	}
	if got, want := cookie.MaxAge, int((2*time.Hour)/time.Second); got != want {
		t.Fatalf("cookie MaxAge = %d, want %d", got, want)
	}
}

func TestLoginUsesOneResponseForInvalidCredentials(t *testing.T) {
	service := fakeAuthService{
		login: func(_ context.Context, _ auth.LoginInput) (auth.LoginResult, error) {
			return auth.LoginResult{}, auth.ErrInvalidCredentials
		},
	}
	app := New(Dependencies{Auth: service, AuthConfig: testAuthConfig()})

	var previousBody string
	for _, username := range []string{"missing.user", "existing.user"} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"`+username+`","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test() error = %v", err)
		}
		body, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			t.Fatalf("read response: %v", readErr)
		}
		if got, want := res.StatusCode, http.StatusUnauthorized; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		if previousBody != "" && string(body) != previousBody {
			t.Fatalf("invalid credential responses differ: %q != %q", body, previousBody)
		}
		previousBody = string(body)
	}
}

func TestLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	var revokedToken string
	service := fakeAuthService{
		logout: func(_ context.Context, token string) error {
			revokedToken = token
			return nil
		},
	}
	app := New(Dependencies{Auth: service, AuthConfig: testAuthConfig()})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "session-to-revoke"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if revokedToken != "session-to-revoke" {
		t.Fatalf("revoked token = %q, want session-to-revoke", revokedToken)
	}
	cookies := res.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "sdms_session" || cookies[0].MaxAge >= 0 {
		t.Fatalf("clear cookies = %v, want an expired sdms_session cookie", cookies)
	}
}

func TestMeReturnsIdentityRoleAndDivisions(t *testing.T) {
	service := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "valid-session" {
				t.Fatalf("session token = %q, want valid-session", token)
			}
			return auth.Profile{
				User: model.User{
					UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d16f-4f09-7c20-8cc7-e8c3363ea1b0")},
					Username:  "head.one",
					FullName:  "หัวหน้าฝ่าย ตัวอย่าง",
					Position:  "ครู",
				},
				Divisions: []model.Division{{
					UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d16f-4f09-7c20-8cc7-e8c3363ea1b1")},
					Code:      "aca",
					Name:      "ฝ่ายวิชาการ",
					ShortName: "วิชาการ",
				}},
				HeadDivisions: []model.Division{{
					UUIDModel: model.UUIDModel{UUID: uuid.MustParse("0199d16f-4f09-7c20-8cc7-e8c3363ea1b1")},
					Code:      "aca",
					Name:      "ฝ่ายวิชาการ",
					ShortName: "วิชาการ",
				}},
			}, nil
		},
	}
	app := New(Dependencies{Auth: service, AuthConfig: testAuthConfig()})
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "valid-session"})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body meResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.FullName != "หัวหน้าฝ่าย ตัวอย่าง" || body.Position != "ครู" || body.Role != "division_head" {
		t.Fatalf("identity = %+v, want name, position, and division_head role", body)
	}
	if got, want := body.UUID.String(), "0199d16f-4f09-7c20-8cc7-e8c3363ea1b0"; got != want {
		t.Fatalf("uuid = %q, want %q", got, want)
	}
	if len(body.Divisions) != 1 || body.Divisions[0].Code != "aca" || body.Divisions[0].UUID == uuid.Nil || len(body.HeadDivisions) != 1 {
		t.Fatalf("divisions = %+v, headed = %+v", body.Divisions, body.HeadDivisions)
	}
}

func TestMeRejectsMissingSession(t *testing.T) {
	service := fakeAuthService{
		currentUser: func(_ context.Context, _ string) (auth.Profile, error) {
			return auth.Profile{}, auth.ErrUnauthenticated
		},
	}
	app := New(Dependencies{Auth: service, AuthConfig: testAuthConfig()})
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "UNAUTHORIZED")
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		SessionTTL: 24 * time.Hour,
		CookieName: "sdms_session",
	}
}
