package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
) {
	topics := router.Group("/topics")

	topics.Post("/", handler.Create)
	topics.Get("/", handler.FindAll)
	topics.Get("/:id", handler.FindTopic)
	topics.Put("/:id", handler.Update)
	topics.Delete("/:id", handler.Delete)
}
