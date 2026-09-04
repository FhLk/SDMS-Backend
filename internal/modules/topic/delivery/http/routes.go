package http

import "github.com/gofiber/fiber/v3"

func RegisterTopicRoutes(router fiber.Router, handler *TopicHandler) {
	topics := router.Group("/topics")

	topics.Post("/", handler.Create)
	topics.Get("/", handler.FindAll)
	topics.Get("/:id", handler.FindTopic)
	topics.Put("/:id", handler.Update)
	topics.Delete("/:id", handler.Delete)

	topics.Post("/:id/fields", handler.CreateField)
	topics.Get("/:id/fields", handler.FindFields)
	topics.Get("/:id/fields/:fieldID", handler.FindField)
	topics.Put("/:id/fields/:fieldID", handler.UpdateField)
	topics.Delete("/:id/fields/:fieldID", handler.DeleteField)
}
