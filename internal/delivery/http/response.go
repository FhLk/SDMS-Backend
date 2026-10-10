package http

import "github.com/gofiber/fiber/v3"

type response struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Meta    any       `json:"meta,omitempty"`
	Error   *apiError `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func success(c fiber.Ctx, status int, data, meta any) error {
	return c.Status(status).JSON(response{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

func failure(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(response{
		Success: false,
		Error: &apiError{
			Code:    code,
			Message: message,
		},
	})
}
