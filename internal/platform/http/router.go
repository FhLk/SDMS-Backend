package http

import (
	"sdms/internal/config"
	healthhttp "sdms/internal/modules/health/delivery/http"
	"sdms/internal/platform/http/routes"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, configs ...*config.Config) *fiber.App {
	cfg := config.Load()
	if len(configs) > 0 && configs[0] != nil {
		cfg = configs[0]
	}

	bodyLimit := cfg.Upload.MaxSizeBytes + (1 * 1024 * 1024)
	app := fiber.New(fiber.Config{
		AppName:   "School Document Management System",
		BodyLimit: int(bodyLimit),
	})

	healthHandler := healthhttp.NewHandler(db)

	api := app.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/health", healthHandler.Health)

	routes.NewRouteTopic(v1, db)
	routes.NewRouteUser(v1, db)
	routes.NewRouteSubmission(v1, db, cfg.Upload)

	return app
}
