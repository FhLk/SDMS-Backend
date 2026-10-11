package server

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHealth(t *testing.T) {
	app := New()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var body healthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := body.Status, "ok"; got != want {
		t.Fatalf("status body = %q, want %q", got, want)
	}
}

func TestNotFoundUsesStandardErrorResponse(t *testing.T) {
	app := New()
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "NOT_FOUND")
}

func TestPanicUsesStandardErrorResponse(t *testing.T) {
	app := New()
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("test panic")
	})
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "INTERNAL_SERVER_ERROR")
}

func assertErrorResponse(t *testing.T, res *http.Response, wantCode string) {
	t.Helper()

	contentType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("response fields = %v, want exactly code and message", body)
	}
	if got := body["code"]; got != wantCode {
		t.Fatalf("code = %v, want %q", got, wantCode)
	}
	if message, ok := body["message"].(string); !ok || message == "" {
		t.Fatalf("message = %v, want a non-empty string", body["message"])
	}
}
