package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *SubmissionHandler,
	fileHandler *SubmissionFileHandler,
	teacherOnly fiber.Handler,
) {
	submissions := router.Group("/topics/:id/submissions")

	submissions.Post("/", teacherOnly, handler.Create)
	submissions.Get("/", handler.FindAll)

	submissions.Post("/:submissionID/files", teacherOnly, fileHandler.Upload)
	submissions.Get("/:submissionID/files", fileHandler.FindAll)
	submissions.Get("/:submissionID", handler.FindByID)

	files := router.Group("/submission-files")
	files.Get("/:fileID", fileHandler.FindByID)
	files.Get("/:fileID/view", fileHandler.View)
	files.Get("/:fileID/download", fileHandler.Download)
	files.Delete("/:fileID", fileHandler.Delete)
}
