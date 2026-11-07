package server

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSwaggerDocumentation(t *testing.T) {
	app := New()

	t.Run("serves UI", func(t *testing.T) {
		response, err := app.Test(httptest.NewRequest("GET", "/api/docs", nil))
		if err != nil {
			t.Fatalf("request Swagger UI: %v", err)
		}
		defer response.Body.Close()

		if response.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("read Swagger UI: %v", err)
		}
		if !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
			t.Fatalf("content type = %q, want HTML", response.Header.Get("Content-Type"))
		}
		if !strings.Contains(string(body), "SDMS API Documentation") {
			t.Fatalf("Swagger UI does not contain the configured title; body starts with %q", string(body[:min(len(body), 500)]))
		}
		if !strings.Contains(string(body), "openapi.yaml") {
			t.Fatalf("Swagger UI does not reference the OpenAPI document; body starts with %q", string(body[:min(len(body), 500)]))
		}
	})

	t.Run("serves complete OpenAPI document", func(t *testing.T) {
		response, err := app.Test(httptest.NewRequest("GET", "/api/openapi.yaml", nil))
		if err != nil {
			t.Fatalf("request OpenAPI document: %v", err)
		}
		defer response.Body.Close()

		if response.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("read OpenAPI document: %v", err)
		}
		document := string(body)
		if !strings.Contains(response.Header.Get("Content-Type"), "application/yaml") {
			t.Fatalf("content type = %q, want application/yaml", response.Header.Get("Content-Type"))
		}
		if !strings.Contains(document, "openapi: 3.0.3") {
			t.Fatalf("OpenAPI version is missing")
		}
		if got := strings.Count(document, "operationId:"); got != 38 {
			t.Fatalf("operation count = %d, want 38", got)
		}
	})
}
