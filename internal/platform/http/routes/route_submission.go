package routes

import (
	"sdms/internal/config"
	submissionhttp "sdms/internal/modules/submission/delivery/http"
	submissionpostgres "sdms/internal/modules/submission/repository/postgres"
	submissionusecase "sdms/internal/modules/submission/usecase"

	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	userpostgres "sdms/internal/modules/user/repository/postgres"
	localstorage "sdms/internal/platform/storage/local"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouteSubmission(
	v1 fiber.Router,
	db *gorm.DB,
	uploadConfig config.UploadConfig,
) {
	submissionRepository := submissionpostgres.NewSubmissionRepository(db)
	fileRepository := submissionpostgres.NewSubmissionFileRepository(db)

	topicRepository := topicpostgres.NewTopicRepository(db)
	fieldRepository := topicpostgres.NewFieldRepository(db)
	userRepository := userpostgres.NewUserRepository(db)

	submissionService := submissionusecase.NewSubmissionService(
		submissionRepository,
		topicRepository,
		fieldRepository,
		userRepository,
	)

	storage, err := localstorage.New(uploadConfig.Dir)
	if err != nil {
		panic(err)
	}

	fileService := submissionusecase.NewSubmissionFileService(
		submissionRepository,
		fileRepository,
		fieldRepository,
		storage,
		uploadConfig.MaxSizeBytes,
	)

	submissionHandler := submissionhttp.NewSubmissionHandler(submissionService)
	fileHandler := submissionhttp.NewSubmissionFileHandler(fileService)

	submissionhttp.RegisterRoutes(v1, submissionHandler, fileHandler)
}
