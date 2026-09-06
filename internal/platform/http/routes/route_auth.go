package routes

import (
	authhttp "sdms/internal/modules/auth/delivery/http"
	authusecase "sdms/internal/modules/auth/usecase"
	userpostgres "sdms/internal/modules/user/repository/postgres"
	platformauth "sdms/internal/platform/auth"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func NewRouteAuth(router fiber.Router, db *gorm.DB, tokens *platformauth.TokenManager, requireAuth fiber.Handler) {
	userRepository := userpostgres.NewUserRepository(db)
	authService := authusecase.NewService(userRepository, tokens)
	authHandler := authhttp.NewHandler(authService)
	authhttp.RegisterRoutes(router, authHandler, requireAuth)
}
