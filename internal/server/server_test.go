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
	decodeSuccessData(t, res, &body)
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

func assertErrorResponse(t *testing.T, res *http.Response, _ string) {
	t.Helper()

	contentType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var body errorResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := body.Status, res.StatusCode; got != want {
		t.Fatalf("body status = %d, want %d", got, want)
	}
	if body.Message == "" {
		t.Fatal("message is empty")
	}
}

func decodeSuccessData(t *testing.T, res *http.Response, target any) {
	t.Helper()

	var body struct {
		Status  int             `json:"status"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode success response: %v", err)
	}
	if body.Status != http.StatusOK {
		t.Fatalf("body status = %d, want %d", body.Status, http.StatusOK)
	}
	if body.Message != successMessage {
		t.Fatalf("message = %q, want %q", body.Message, successMessage)
	}
	if target == nil {
		if len(body.Data) != 0 {
			t.Fatalf("data = %s, want omitted", body.Data)
		}
		return
	}
	if len(body.Data) == 0 {
		t.Fatal("data is missing")
	}
	if err := json.Unmarshal(body.Data, target); err != nil {
		t.Fatalf("decode success data: %v", err)
	}
}
