package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *userHandler,
) {
	users := router.Group("/users")
	users.Post("/", handler.Create)
	users.Get("/", handler.List)
	users.Get("/username/:username", handler.GetByUsername)
	users.Get("/:id", handler.GetByID)
	users.Put("/:id", handler.Update)
	users.Patch("/:id/status", handler.UpdateStatus)
	users.Delete("/:id", handler.Delete)
}
