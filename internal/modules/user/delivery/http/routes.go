package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *userHandler,
	directorOnly fiber.Handler,
) {
	users := router.Group("/users")
	users.Use(directorOnly)
	users.Post("/", handler.Create)
	users.Get("/", handler.List)
	users.Get("/username/:username", handler.GetByUsername)
	users.Get("/:id", handler.GetByID)
	users.Put("/:id", handler.Update)
	users.Patch("/:id/status", handler.UpdateStatus)
	users.Patch("/:id/password", handler.ResetPassword)
	users.Delete("/:id", handler.Delete)
}
