package http

import (
	healthhttp "sdms/internal/modules/health/delivery/http"
	"sdms/internal/platform/http/routes"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "School Document Management System",
	})

	healthHandler := healthhttp.NewHandler(db)

	api := app.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/health", healthHandler.Health)

	routes.NewRouteTopic(v1, db)
	routes.NewRouteUser(v1, db)

	return app
}
