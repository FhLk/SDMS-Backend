package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *SubmissionHandler,
	fileHandler *SubmissionFileHandler,
	teacherOnly fiber.Handler,
	reviewerOnly fiber.Handler,
) {
	submissions := router.Group("/topics/:id/submissions")
	submissions.Post("/", teacherOnly, handler.Create)
	submissions.Get("/", handler.FindAll)
	submissions.Get("/status", reviewerOnly, handler.SubmissionStatus)
	submissions.Get("/:submissionID", handler.FindByID)
	submissions.Put("/:submissionID", teacherOnly, handler.Update)
	submissions.Delete("/:submissionID", teacherOnly, handler.Delete)

	submissions.Post("/:submissionID/files", teacherOnly, fileHandler.Upload)
	submissions.Get("/:submissionID/files", fileHandler.FindAll)

	files := router.Group("/submission-files")
	files.Get("/:fileID", fileHandler.FindByID)
	files.Get("/:fileID/view", fileHandler.View)
	files.Get("/:fileID/download", fileHandler.Download)
	files.Delete("/:fileID", fileHandler.Delete)
}
