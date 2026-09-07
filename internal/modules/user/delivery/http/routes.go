package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *userHandler,
	userManager fiber.Handler,
	reviewerOnly fiber.Handler,
) {
	users := router.Group("/users")

	// Review roles need teacher identity data to inspect submissions by person,
	// but only administrators may change accounts.
	users.Get("/", reviewerOnly, handler.List)
	users.Get("/username/:username", reviewerOnly, handler.GetByUsername)
	users.Get("/:id", reviewerOnly, handler.GetByID)

	users.Post("/", userManager, handler.Create)
	users.Put("/:id", userManager, handler.Update)
	users.Patch("/:id/status", userManager, handler.UpdateStatus)
	users.Patch("/:id/password", userManager, handler.ResetPassword)
	users.Delete("/:id", userManager, handler.Delete)
}
