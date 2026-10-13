// Package server configures the Fiber application and registers HTTP routes.
package server

import (
	"github.com/FhLk/SDMS-Backend/internal/authorization"
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
	topics := topicHandler{service: deps.Topics}
	api.Post("/topics", auth.authenticate, topics.create)
	api.Patch("/topics/:uuid", auth.authenticate, topics.update)
	api.Post("/topics/:uuid/close", auth.authenticate, topics.close)
	api.Post("/topics/:uuid/reopen", auth.authenticate, topics.reopen)
	api.Delete("/topics/:uuid", auth.authenticate, topics.delete)
	api.Get("/divisions", auth.authenticate, topics.listDivisions)
	api.Get("/divisions/:code/topics", auth.authenticate, topics.listDivisionTopics)

	// ทุก route ใต้ /admin ใช้ authentication และ policy แอดมินชุดเดียวกัน
	// จึงไม่ต้องตรวจบทบาทแอดมินซ้ำในแต่ละ handler.
	users := adminUserHandler{service: deps.UserAdmin}
	organization := adminOrganizationHandler{service: deps.OrganizationAdmin}
	adminRoutes := api.Group("/admin", auth.authenticate, requirePermission(authorization.CanManageUsers))
	adminRoutes.Get("/users", users.list)
	adminRoutes.Post("/users", users.create)
	adminRoutes.Post("/users/import", users.importUsers)
	adminRoutes.Get("/users/import/template.csv", users.downloadCSVImportTemplate)
	adminRoutes.Get("/users/import/template.xlsx", users.downloadXLSXImportTemplate)
	adminRoutes.Patch("/users/:uuid", users.update)
	adminRoutes.Post("/users/:uuid/password", users.resetPassword)
	adminRoutes.Put("/divisions/:code/head", organization.setDivisionHead)
	adminRoutes.Get("/years", organization.listAcademicYears)
	adminRoutes.Post("/years", organization.createAcademicYear)
	adminRoutes.Post("/years/:year/current", organization.setCurrentAcademicYear)

	return app
}

// health is a lightweight liveness endpoint. Database reachability is already
// enforced during startup, so a running process can answer this without a query.
func health(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(healthResponse{Status: "ok"})
}
