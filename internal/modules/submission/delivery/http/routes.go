package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *SubmissionHandler,
) {
	submissions := router.Group("/topics/:id/submissions")

	submissions.Post("/", handler.Create)
	submissions.Get("/", handler.FindAll)
	submissions.Get("/:submissionID", handler.FindByID)
}
