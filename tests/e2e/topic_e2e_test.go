package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	topichttp "sdms/internal/modules/topic/delivery/http"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	topicusecase "sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type topicResponse struct {
	UID         uuid.UUID `json:"uid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
}

type errorResponse struct {
	Message string `json:"message"`
}

func setupE2EApp(t *testing.T) *fiber.App {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.Run(
		ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("sdms_e2e"),
		tcpostgres.WithUsername("sdms"),
		tcpostgres.WithPassword("sdms_password"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	testcontainers.CleanupContainer(t, container)

	dsn, err := container.ConnectionString(
		ctx,
		"sslmode=disable",
	)
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	db, err := gorm.Open(
		gormpostgres.Open(dsn),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	if err := db.AutoMigrate(
		&topicpostgres.TopicModel{},
	); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	repository := topicpostgres.NewRepository(db)
	service := topicusecase.NewService(repository)
	handler := topichttp.NewHandler(service)

	app := fiber.New()

	v1 := app.Group("/api/v1")
	topichttp.RegisterRoutes(v1, handler)

	return app
}

func performRequest(
	t *testing.T,
	app *fiber.App,
	method string,
	path string,
	body []byte,
) *nethttp.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req := httptest.NewRequest(
		method,
		path,
		reader,
	)

	if body != nil {
		req.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	resp, err := app.Test(
		req,
		fiber.TestConfig{
			Timeout:       0,
			FailOnTimeout: false,
		},
	)
	if err != nil {
		t.Fatalf(
			"%s %s failed: %v",
			method,
			path,
			err,
		)
	}

	return resp
}

func decodeJSON[T any](
	t *testing.T,
	resp *nethttp.Response,
) T {
	t.Helper()

	defer resp.Body.Close()

	var result T

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf(
			"failed to decode response body: %v",
			err,
		)
	}

	return result
}

func TestTopicE2E_CRUDFlow(t *testing.T) {
	app := setupE2EApp(t)

	// ---------------------------------------------------------------------
	// 1. CREATE
	// POST /api/v1/topics
	// ---------------------------------------------------------------------

	createBody := []byte(`{
		"name": "งานวิชาการ",
		"description": "เอกสารงานวิชาการ"
	}`)

	createResp := performRequest(
		t,
		app,
		nethttp.MethodPost,
		"/api/v1/topics",
		createBody,
	)

	if createResp.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		createResp.Body.Close()

		t.Fatalf(
			"create: expected status %d, got %d, body=%s",
			fiber.StatusCreated,
			createResp.StatusCode,
			string(body),
		)
	}

	created := decodeJSON[topicResponse](t, createResp)

	if created.UID == uuid.Nil {
		t.Fatal(
			"create: expected generated UUID, got uuid.Nil",
		)
	}

	if created.Name != "งานวิชาการ" {
		t.Errorf(
			"create: expected name %q, got %q",
			"งานวิชาการ",
			created.Name,
		)
	}

	if created.Description != "เอกสารงานวิชาการ" {
		t.Errorf(
			"create: expected description %q, got %q",
			"เอกสารงานวิชาการ",
			created.Description,
		)
	}

	if !created.IsActive {
		t.Error(
			"create: expected is_active=true",
		)
	}

	topicPath := fmt.Sprintf(
		"/api/v1/topics/%s",
		created.UID.String(),
	)

	// ---------------------------------------------------------------------
	// 2. FIND BY ID
	// GET /api/v1/topics/:id
	// ---------------------------------------------------------------------

	getResp := performRequest(
		t,
		app,
		nethttp.MethodGet,
		topicPath,
		nil,
	)

	if getResp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()

		t.Fatalf(
			"find by id: expected status %d, got %d, body=%s",
			fiber.StatusOK,
			getResp.StatusCode,
			string(body),
		)
	}

	found := decodeJSON[topicResponse](t, getResp)

	if found.UID != created.UID {
		t.Errorf(
			"find by id: expected ID %s, got %s",
			created.UID,
			found.UID,
		)
	}

	if found.Name != created.Name {
		t.Errorf(
			"find by id: expected name %q, got %q",
			created.Name,
			found.Name,
		)
	}

	// ---------------------------------------------------------------------
	// 3. UPDATE
	// PUT /api/v1/topics/:id
	// ---------------------------------------------------------------------

	updateBody := []byte(`{
		"name": "งานวิชาการ 2569",
		"description": "เอกสารงานวิชาการประจำปี 2569",
		"is_active": false
	}`)

	updateResp := performRequest(
		t,
		app,
		nethttp.MethodPut,
		topicPath,
		updateBody,
	)

	if updateResp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		updateResp.Body.Close()

		t.Fatalf(
			"update: expected status %d, got %d, body=%s",
			fiber.StatusOK,
			updateResp.StatusCode,
			string(body),
		)
	}

	updated := decodeJSON[topicResponse](t, updateResp)

	if updated.UID != created.UID {
		t.Errorf(
			"update: expected ID %s, got %s",
			created.UID,
			updated.UID,
		)
	}

	if updated.Name != "งานวิชาการ 2569" {
		t.Errorf(
			"update: expected name %q, got %q",
			"งานวิชาการ 2569",
			updated.Name,
		)
	}

	if updated.Description != "เอกสารงานวิชาการประจำปี 2569" {
		t.Errorf(
			"update: unexpected description %q",
			updated.Description,
		)
	}

	if updated.IsActive {
		t.Error(
			"update: expected is_active=false",
		)
	}

	// Verify UPDATE was really persisted in PostgreSQL by reading it again.
	verifyUpdateResp := performRequest(
		t,
		app,
		nethttp.MethodGet,
		topicPath,
		nil,
	)

	if verifyUpdateResp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(verifyUpdateResp.Body)
		verifyUpdateResp.Body.Close()

		t.Fatalf(
			"verify update: expected status %d, got %d, body=%s",
			fiber.StatusOK,
			verifyUpdateResp.StatusCode,
			string(body),
		)
	}

	persisted := decodeJSON[topicResponse](
		t,
		verifyUpdateResp,
	)

	if persisted.Name != "งานวิชาการ 2569" {
		t.Errorf(
			"verify update: expected persisted name %q, got %q",
			"งานวิชาการ 2569",
			persisted.Name,
		)
	}

	if persisted.IsActive {
		t.Error(
			"verify update: expected persisted is_active=false",
		)
	}

	// ---------------------------------------------------------------------
	// 4. FIND ALL
	// GET /api/v1/topics
	// ---------------------------------------------------------------------

	listResp := performRequest(
		t,
		app,
		nethttp.MethodGet,
		"/api/v1/topics",
		nil,
	)

	if listResp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		listResp.Body.Close()

		t.Fatalf(
			"find all: expected status %d, got %d, body=%s",
			fiber.StatusOK,
			listResp.StatusCode,
			string(body),
		)
	}

	topics := decodeJSON[[]topicResponse](
		t,
		listResp,
	)

	if len(topics) != 1 {
		t.Fatalf(
			"find all: expected 1 topic, got %d: %+v",
			len(topics),
			topics,
		)
	}

	if topics[0].UID != created.UID {
		t.Errorf(
			"find all: expected ID %s, got %s",
			created.UID,
			topics[0].UID,
		)
	}

	if topics[0].Name != "งานวิชาการ 2569" {
		t.Errorf(
			"find all: expected updated name %q, got %q",
			"งานวิชาการ 2569",
			topics[0].Name,
		)
	}

	// ---------------------------------------------------------------------
	// 5. DELETE
	// DELETE /api/v1/topics/:id
	// ---------------------------------------------------------------------

	deleteResp := performRequest(
		t,
		app,
		nethttp.MethodDelete,
		topicPath,
		nil,
	)
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(deleteResp.Body)

		t.Fatalf(
			"delete: expected status %d, got %d, body=%s",
			fiber.StatusNoContent,
			deleteResp.StatusCode,
			string(body),
		)
	}

	// ---------------------------------------------------------------------
	// 6. VERIFY DELETE
	// GET /api/v1/topics/:id should now return 404.
	// ---------------------------------------------------------------------

	afterDeleteResp := performRequest(
		t,
		app,
		nethttp.MethodGet,
		topicPath,
		nil,
	)

	if afterDeleteResp.StatusCode != fiber.StatusNotFound {
		body, _ := io.ReadAll(afterDeleteResp.Body)
		afterDeleteResp.Body.Close()

		t.Fatalf(
			"after delete: expected status %d, got %d, body=%s",
			fiber.StatusNotFound,
			afterDeleteResp.StatusCode,
			string(body),
		)
	}

	notFound := decodeJSON[errorResponse](
		t,
		afterDeleteResp,
	)

	if notFound.Message == "" {
		t.Error(
			"after delete: expected error message",
		)
	}
}
