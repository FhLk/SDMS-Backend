package http

import "github.com/gofiber/fiber/v3"

func RegisterTopicRoutes(router fiber.Router, handler *TopicHandler, directorOnly fiber.Handler) {
	topics := router.Group("/topics")

	topics.Post("/", directorOnly, handler.Create)
	topics.Get("/", handler.FindAll)
	topics.Get("/:id", handler.FindTopic)
	topics.Put("/:id", directorOnly, handler.Update)
	topics.Delete("/:id", directorOnly, handler.Delete)

	topics.Post("/:id/fields", directorOnly, handler.CreateField)
	topics.Get("/:id/fields", handler.FindFields)
	topics.Get("/:id/fields/:fieldID", handler.FindField)
	topics.Put("/:id/fields/:fieldID", directorOnly, handler.UpdateField)
	topics.Delete("/:id/fields/:fieldID", directorOnly, handler.DeleteField)
}
