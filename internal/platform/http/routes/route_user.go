package routes

import (
	userhttp "sdms/internal/modules/user/delivery/http"
	userpostgres "sdms/internal/modules/user/repository/postgres"
	userusecase "sdms/internal/modules/user/usecase"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouteUser(v1 fiber.Router, db *gorm.DB) {
	userRepository := userpostgres.NewUserRepository(db)
	userService := userusecase.NewUserService(userRepository)
	userHandler := userhttp.NewUserHandler(userService)

	userhttp.RegisterRoutes(v1, userHandler)
}
