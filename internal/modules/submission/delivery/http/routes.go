package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *SubmissionHandler,
	fileHandler *SubmissionFileHandler,
) {
	submissions := router.Group("/topics/:id/submissions")

	submissions.Post("/", handler.Create)
	submissions.Get("/", handler.FindAll)

	submissions.Post("/:submissionID/files", fileHandler.Upload)
	submissions.Get("/:submissionID/files", fileHandler.FindAll)
	submissions.Get("/:submissionID", handler.FindByID)

	files := router.Group("/submission-files")
	files.Get("/:fileID", fileHandler.FindByID)
	files.Get("/:fileID/view", fileHandler.View)
	files.Get("/:fileID/download", fileHandler.Download)
	files.Delete("/:fileID", fileHandler.Delete)
}
