// Package server configures the Fiber application and registers HTTP routes.
package server

import (
	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
)

type healthResponse struct {
	Status string `json:"status"`
}

// New builds the Fiber application with middleware and all API routes.
func New() *fiber.App {
	// A single global error handler guarantees that route, middleware, and
	// application errors all use the same JSON response shape.
	app := fiber.New(fiber.Config{
		ErrorHandler: handleError,
	})

	// Fiber does not recover panics by default. Register recovery before routes
	// so unexpected panics are forwarded to handleError as a JSON 500 response.
	app.Use(recoverer.New())

	// Grouping under /api keeps the public API prefix in one place.
	api := app.Group("/api")
	api.Get("/health", health)

	return app
}

// health is a lightweight liveness endpoint. Database reachability is already
// enforced during startup, so a running process can answer this without a query.
func health(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(healthResponse{Status: "ok"})
}
