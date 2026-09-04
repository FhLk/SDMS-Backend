package routes

import (
	submissionpostgres "sdms/internal/modules/submission/repository/postgres"
	topichttp "sdms/internal/modules/topic/delivery/http"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	topicusecase "sdms/internal/modules/topic/usecase"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouteTopic(v1 fiber.Router, db *gorm.DB) {
	topicRepository := topicpostgres.NewTopicRepository(db)
	topicFieldRepository := topicpostgres.NewFieldRepository(db)
	submissionRepository := submissionpostgres.NewSubmissionRepository(db)

	topicService := topicusecase.NewTopicService(
		topicRepository,
		topicFieldRepository,
		submissionRepository,
	)

	topicHandler := topichttp.NewTopicHandler(topicService)

	topichttp.RegisterTopicRoutes(v1, topicHandler)
}
