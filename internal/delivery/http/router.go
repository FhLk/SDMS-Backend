package http

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FhLk/SDMS-Backend/internal/usecase"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func NewRouter(userUsecase *usecase.UserUsecase, db *sql.DB) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "SDMS API",
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New())
	app.Use(helmet.New())
	app.Use(cors.New())

	app.Get("/health", healthHandler(db))

	userHandler := NewUserHandler(userUsecase)
	users := app.Group("/api/v1/users")
	users.Post("/", userHandler.Create)
	users.Get("/", userHandler.List)
	users.Get("/:id", userHandler.GetByID)
	users.Put("/:id", userHandler.Update)
	users.Delete("/:id", userHandler.Delete)

	return app
}

func healthHandler(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := db.PingContext(c.Context()); err != nil {
			return failure(c, fiber.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database is unavailable")
		}
		return success(c, fiber.StatusOK, fiber.Map{"status": "ok"}, nil)
	}
}

func errorHandler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "an internal error occurred"
	code := "INTERNAL_ERROR"

	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		status = fiberError.Code
		message = fiberError.Message
		code = "HTTP_ERROR"
		if status == fiber.StatusNotFound {
			code = "ROUTE_NOT_FOUND"
			message = "route not found"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		status = fiber.StatusGatewayTimeout
		code = "TIMEOUT"
		message = "request timed out"
	}

	return failure(c, status, code, message)
}
