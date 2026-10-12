package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeAdminUserService struct {
	list          func(context.Context) ([]model.User, error)
	create        func(context.Context, admin.CreateUserInput) (model.User, error)
	update        func(context.Context, uuid.UUID, admin.UpdateUserInput) (model.User, error)
	resetPassword func(context.Context, uuid.UUID, string) error
	importUsers   func(context.Context, string, []byte) (admin.ImportResult, error)
}

func (f fakeAdminUserService) ListUsers(ctx context.Context) ([]model.User, error) {
	return f.list(ctx)
}

func (f fakeAdminUserService) CreateUser(ctx context.Context, input admin.CreateUserInput) (model.User, error) {
	return f.create(ctx, input)
}

func (f fakeAdminUserService) UpdateUser(ctx context.Context, userUUID uuid.UUID, input admin.UpdateUserInput) (model.User, error) {
	return f.update(ctx, userUUID, input)
}

func (f fakeAdminUserService) ResetPassword(ctx context.Context, userUUID uuid.UUID, password string) error {
	return f.resetPassword(ctx, userUUID, password)
}

func (f fakeAdminUserService) ImportUsers(ctx context.Context, filename string, content []byte) (admin.ImportResult, error) {
	return f.importUsers(ctx, filename, content)
}

func TestAdminUsersRequiresAdminRole(t *testing.T) {
	called := false
	users := fakeAdminUserService{
		list: func(context.Context) ([]model.User, error) {
			called = true
			return nil, nil
		},
	}
	app := adminUsersTestApp(users)

	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantCode   string
	}{
		{name: "missing session", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "teacher", token: "teacher-session", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
			if tt.token != "" {
				req.AddCookie(&http.Cookie{Name: "sdms_session", Value: tt.token})
			}
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
	if called {
		t.Fatal("user service was called before authorization")
	}
}

func TestAdminListsUsersWithoutPasswordHash(t *testing.T) {
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f850001")
	users := fakeAdminUserService{
		list: func(context.Context) ([]model.User, error) {
			return []model.User{{
				UUIDModel:    model.UUIDModel{UUID: userUUID},
				Username:     "teacher.one",
				PasswordHash: "must-not-leak",
				FullName:     "ครู หนึ่ง",
				Position:     "ครู",
			}}, nil
		},
	}
	app := adminUsersTestApp(users)
	req := adminRequest(http.MethodGet, "/api/admin/users", "")

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
	if len(body) != 1 || body[0]["uuid"] != userUUID.String() || body[0]["is_active"] != true {
		t.Fatalf("body = %+v, want active user", body)
	}
	if _, exists := body[0]["password_hash"]; exists {
		t.Fatal("response contains password_hash")
	}
}

func TestAdminCreatesUser(t *testing.T) {
	divisionUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f860001")
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f860002")
	var received admin.CreateUserInput
	users := fakeAdminUserService{
		create: func(_ context.Context, input admin.CreateUserInput) (model.User, error) {
			received = input
			return model.User{
				UUIDModel: model.UUIDModel{UUID: userUUID},
				Username:  input.Username,
				FullName:  input.FullName,
				Position:  input.Position,
				IsExec:    input.IsExec,
			}, nil
		},
	}
	app := adminUsersTestApp(users)
	body := `{"username":"teacher.one","password":"safe-password","full_name":"ครู หนึ่ง","position":"ครู","division_uuids":["` + divisionUUID.String() + `"],"is_exec":true}`
	req := adminRequest(http.MethodPost, "/api/admin/users", body)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if received.Username != "teacher.one" || received.Password != "safe-password" || !received.IsExec {
		t.Fatalf("CreateUser input = %+v", received)
	}
	if len(received.DivisionUUIDs) != 1 || received.DivisionUUIDs[0] != divisionUUID {
		t.Fatalf("division UUIDs = %v", received.DivisionUUIDs)
	}
}

func TestAdminCreateDuplicateUsernameReturnsConflict(t *testing.T) {
	users := fakeAdminUserService{
		create: func(context.Context, admin.CreateUserInput) (model.User, error) {
			return model.User{}, admin.ErrUsernameExists
		},
	}
	app := adminUsersTestApp(users)
	req := adminRequest(http.MethodPost, "/api/admin/users", `{"username":"duplicate","password":"safe-password","full_name":"ชื่อซ้ำ","position":"ครู"}`)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "USERNAME_EXISTS")
}

func TestAdminUpdatesUserWithPatchFields(t *testing.T) {
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f870001")
	var received admin.UpdateUserInput
	users := fakeAdminUserService{
		update: func(_ context.Context, gotUUID uuid.UUID, input admin.UpdateUserInput) (model.User, error) {
			if gotUUID != userUUID {
				t.Fatalf("user UUID = %s, want %s", gotUUID, userUUID)
			}
			received = input
			return model.User{UUIDModel: model.UUIDModel{UUID: userUUID}, Username: "teacher.one", FullName: *input.FullName, Position: "ครู"}, nil
		},
	}
	app := adminUsersTestApp(users)
	req := adminRequest(http.MethodPatch, "/api/admin/users/"+userUUID.String(), `{"full_name":"ชื่อใหม่","division_uuids":[],"is_active":false}`)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if received.FullName == nil || *received.FullName != "ชื่อใหม่" || received.IsActive == nil || *received.IsActive {
		t.Fatalf("UpdateUser input = %+v", received)
	}
	if received.DivisionUUIDs == nil || len(*received.DivisionUUIDs) != 0 {
		t.Fatalf("division UUIDs = %v, want an explicit empty slice", received.DivisionUUIDs)
	}
	if received.Username != nil || received.Position != nil || received.IsAdmin != nil || received.IsExec != nil {
		t.Fatalf("unsent PATCH fields were populated: %+v", received)
	}
}

func TestAdminResetsPassword(t *testing.T) {
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f880001")
	var receivedPassword string
	users := fakeAdminUserService{
		resetPassword: func(_ context.Context, gotUUID uuid.UUID, password string) error {
			if gotUUID != userUUID {
				t.Fatalf("user UUID = %s, want %s", gotUUID, userUUID)
			}
			receivedPassword = password
			return nil
		},
	}
	app := adminUsersTestApp(users)
	req := adminRequest(http.MethodPost, "/api/admin/users/"+userUUID.String()+"/password", `{"password":"new-password"}`)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if receivedPassword != "new-password" {
		t.Fatalf("password = %q, want new-password", receivedPassword)
	}
}

func TestAdminImportsUsersFromMultipartFile(t *testing.T) {
	const csvContent = "ชื่อ-สกุล,username,รหัสผ่านเริ่มต้น,ตำแหน่ง,ฝ่าย\nครู หนึ่ง,teacher.one,safe-password,ครู,วิชาการ\n"
	users := fakeAdminUserService{
		importUsers: func(_ context.Context, filename string, content []byte) (admin.ImportResult, error) {
			if filename != "users.csv" {
				t.Fatalf("filename = %q, want users.csv", filename)
			}
			if string(content) != csvContent {
				t.Fatalf("content = %q, want uploaded CSV", content)
			}
			return admin.ImportResult{Imported: 1}, nil
		},
	}
	app := adminUsersTestApp(users)
	req := multipartAdminRequest(t, "/api/admin/users/import", "users.csv", []byte(csvContent))

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body admin.ImportResult
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Imported != 1 {
		t.Fatalf("imported = %d, want 1", body.Imported)
	}
}

func TestAdminImportRequiresAdminRole(t *testing.T) {
	called := false
	users := fakeAdminUserService{
		importUsers: func(context.Context, string, []byte) (admin.ImportResult, error) {
			called = true
			return admin.ImportResult{}, nil
		},
	}
	app := adminUsersTestApp(users)
	req := multipartAdminRequest(t, "/api/admin/users/import", "users.csv", []byte("content"))
	req.Header.Set("Cookie", "sdms_session=teacher-session")

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
		t.Fatal("import service was called before authorization")
	}
}

func TestAdminImportReturnsErrorsForEveryInvalidRow(t *testing.T) {
	users := fakeAdminUserService{
		importUsers: func(context.Context, string, []byte) (admin.ImportResult, error) {
			return admin.ImportResult{}, &admin.ImportValidationError{Rows: []admin.ImportRowError{
				{Row: 2, Field: "username", Message: "username is invalid"},
				{Row: 4, Field: "divisions", Message: "division does not exist"},
			}}
		},
	}
	app := adminUsersTestApp(users)
	req := multipartAdminRequest(t, "/api/admin/users/import", "users.csv", []byte("secret-value"))

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if bytes.Contains(responseBody, []byte("secret-value")) {
		t.Fatal("response leaked uploaded cell content")
	}
	var body struct {
		Code    string `json:"code"`
		Details struct {
			Rows []admin.ImportRowError `json:"rows"`
		} `json:"details"`
	}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "IMPORT_VALIDATION_FAILED" || len(body.Details.Rows) != 2 {
		t.Fatalf("body = %s, want two structured row errors", responseBody)
	}
}

func TestAdminDownloadsUserImportTemplates(t *testing.T) {
	app := adminUsersTestApp(fakeAdminUserService{})
	tests := []struct {
		path        string
		contentType string
		filename    string
		prefix      []byte
	}{
		{
			path:        "/api/admin/users/import/template.csv",
			contentType: "text/csv",
			filename:    "sdms-users-template.csv",
			prefix:      []byte{0xef, 0xbb, 0xbf},
		},
		{
			path:        "/api/admin/users/import/template.xlsx",
			contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			filename:    "sdms-users-template.xlsx",
			prefix:      []byte("PK"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			req := adminRequest(http.MethodGet, tt.path, "")
			res, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if got, want := res.StatusCode, http.StatusOK; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
			if !strings.Contains(res.Header.Get("Content-Type"), tt.contentType) {
				t.Fatalf("Content-Type = %q, want %q", res.Header.Get("Content-Type"), tt.contentType)
			}
			if !strings.Contains(res.Header.Get("Content-Disposition"), tt.filename) {
				t.Fatalf("Content-Disposition = %q, want filename %q", res.Header.Get("Content-Disposition"), tt.filename)
			}
			content, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read template: %v", err)
			}
			if !bytes.HasPrefix(content, tt.prefix) {
				t.Fatalf("template prefix = %x, want %x", content[:min(len(content), len(tt.prefix))], tt.prefix)
			}
		})
	}
}

func adminUsersTestApp(users adminUserService) *fiber.App {
	adminUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f89a001")
	teacherUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f89a002")
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
	return New(Dependencies{Auth: authService, AuthConfig: testAuthConfig(), UserAdmin: users})
}

func adminRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "admin-session"})
	return req
}

func multipartAdminRequest(t *testing.T, target, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "admin-session"})
	return req
}
