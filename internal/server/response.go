package server

import "github.com/gofiber/fiber/v3"

const successMessage = "success"

type successResponse struct {
	Status  int    `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
}

type errorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// ResponseSuccess returns the shared JSON success envelope. JSON operations
// intentionally use HTTP 200 consistently; streaming and documentation routes
// bypass this helper because their response body is not JSON.
func ResponseSuccess(c fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(successResponse{
		Status:  fiber.StatusOK,
		Data:    data,
		Message: successMessage,
	})
}

// ResponseError returns the shared JSON error envelope while preserving the
// HTTP status selected by the domain-to-HTTP error mapper.
func ResponseError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(errorResponse{
		Status:  status,
		Message: message,
	})
}
