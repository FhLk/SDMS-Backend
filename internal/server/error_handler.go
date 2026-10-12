package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/gofiber/fiber/v3"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// handleError converts every Fiber/application error into the public
// {"code":"...","message":"..."} contract.
func handleError(c fiber.Ctx, err error) error {
	// Unknown errors default to a generic 500 response so internal details such
	// as SQL statements, file paths, and stack data are not leaked to clients.
	status := fiber.StatusInternalServerError
	response := errorResponse{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "Internal server error",
	}

	var appErr *apperror.Error
	var fiberErr *fiber.Error
	switch {
	case errors.As(err, &appErr):
		// Application errors have an explicit, frontend-friendly error contract.
		status = appErr.Status
		response.Code = appErr.Code
		response.Message = appErr.Message
		response.Details = appErr.Details
	case errors.As(err, &fiberErr):
		// Fiber errors cover framework cases such as 404 and 405.
		status = fiberErr.Code
		response.Code = statusCode(status)
		response.Message = fiberErr.Message
	}

	// Keep the full server-side error for diagnosis while returning only the safe
	// response assembled above to the caller.
	if status >= fiber.StatusInternalServerError {
		log.Printf("HTTP request failed: %v", err)
	}

	return c.Status(status).JSON(response)
}

// statusCode maps HTTP statuses to stable codes that clients can branch on.
func statusCode(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "BAD_REQUEST"
	case fiber.StatusUnauthorized:
		return "UNAUTHORIZED"
	case fiber.StatusForbidden:
		return "FORBIDDEN"
	case fiber.StatusNotFound:
		return "NOT_FOUND"
	case fiber.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case fiber.StatusConflict:
		return "CONFLICT"
	case fiber.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case fiber.StatusUnprocessableEntity:
		return "UNPROCESSABLE_ENTITY"
	case fiber.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		if text := http.StatusText(status); text != "" {
			return "HTTP_ERROR"
		}
		return "INTERNAL_SERVER_ERROR"
	}
}
