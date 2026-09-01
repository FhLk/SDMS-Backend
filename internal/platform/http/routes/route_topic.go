package routes

import (
	topichttp "sdms/internal/modules/topic/delivery/http"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	topicusecase "sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouteTopic(v1 fiber.Router, db *gorm.DB) {
	topicRepository := topicpostgres.NewRepository(db)

	topicService := topicusecase.NewService(
		topicRepository,
	)

	topicHandler := topichttp.NewHandler(topicService)

	topichttp.RegisterRoutes(v1, topicHandler)
}
