package server

import (
	"errors"
	"log"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/gofiber/fiber/v3"
)

// handleError converts every Fiber/application error into the shared public
// {"status": <http status>, "message": "..."} contract.
func handleError(c fiber.Ctx, err error) error {
	// Unknown errors default to a generic 500 response so internal details such
	// as SQL statements, file paths, and stack data are not leaked to clients.
	status := fiber.StatusInternalServerError
	message := "Internal server error"

	var appErr *apperror.Error
	var fiberErr *fiber.Error
	switch {
	case errors.As(err, &appErr):
		// Application errors have an explicit, frontend-friendly error contract.
		status = appErr.Status
		message = appErr.Message
	case errors.As(err, &fiberErr):
		// Fiber errors cover framework cases such as 404 and 405.
		status = fiberErr.Code
		message = fiberErr.Message
	}

	// Keep the full server-side error for diagnosis while returning only the safe
	// response assembled above to the caller.
	if status >= fiber.StatusInternalServerError {
		log.Printf("HTTP request failed: %v", err)
	}

	return ResponseError(c, status, message)
}
