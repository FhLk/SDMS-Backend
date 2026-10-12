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
func New(dependencies ...Dependencies) *fiber.App {
	var deps Dependencies
	if len(dependencies) > 0 {
		deps = dependencies[0]
	}

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

	auth := authHandler{service: deps.Auth, config: deps.AuthConfig}
	authRoutes := api.Group("/auth")
	authRoutes.Post("/login", auth.login)
	authRoutes.Post("/logout", auth.logout)
	// /me เป็น protected route แรกที่ใช้ middleware กลาง โดย middleware จะ
	// ตรวจ session และส่ง profile ต่อให้ handler ผ่าน request context.
	api.Get("/me", auth.authenticate, auth.me)

	return app
}

// health is a lightweight liveness endpoint. Database reachability is already
// enforced during startup, so a running process can answer this without a query.
func health(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(healthResponse{Status: "ok"})
}
