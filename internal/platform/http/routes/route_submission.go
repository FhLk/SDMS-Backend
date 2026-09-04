package routes

import (
	submissionhttp "sdms/internal/modules/submission/delivery/http"
	submissionpostgres "sdms/internal/modules/submission/repository/postgres"
	submissionusecase "sdms/internal/modules/submission/usecase"

	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	userpostgres "sdms/internal/modules/user/repository/postgres"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouteSubmission(
	v1 fiber.Router,
	db *gorm.DB,
) {
	submissionRepository := submissionpostgres.NewSubmissionRepository(db)

	topicRepository := topicpostgres.NewTopicRepository(db)

	fieldRepository := topicpostgres.NewFieldRepository(db)

	userRepository := userpostgres.NewUserRepository(db)

	submissionService := submissionusecase.NewSubmissionService(
		submissionRepository,
		topicRepository,
		fieldRepository,
		userRepository,
	)

	submissionHandler := submissionhttp.NewSubmissionHandler(submissionService)

	submissionhttp.RegisterRoutes(v1, submissionHandler)
}
