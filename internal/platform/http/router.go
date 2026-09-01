package http

import (
	healthhttp "sdms/internal/modules/health/delivery/http"

	topichttp "sdms/internal/modules/topic/delivery/http"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	topicusecase "sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "School Document Management System",
	})

	healthHandler := healthhttp.NewHandler(db)

	topicRepository := topicpostgres.NewRepository(db)

	topicService := topicusecase.NewService(
		topicRepository,
	)

	topicHandler := topichttp.NewHandler(
		topicService,
	)

	api := app.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/health", healthHandler.Health)

	topichttp.RegisterRoutes(
		v1,
		topicHandler,
	)

	return app
}
