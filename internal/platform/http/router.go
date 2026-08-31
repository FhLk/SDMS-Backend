package http

import (
	healthhttp "sdms/internal/modules/health/delivery/http"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "SDMS Backend",
	})

	healthHandler := healthhttp.NewHandler(db)

	api := app.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/health", healthHandler.Health)

	return app
}
