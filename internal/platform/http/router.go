package http

import (
	"time"

	"sdms/internal/config"
	healthhttp "sdms/internal/modules/health/delivery/http"
	userdomain "sdms/internal/modules/user/domain"
	userpostgres "sdms/internal/modules/user/repository/postgres"
	"sdms/internal/platform/audit"
	platformauth "sdms/internal/platform/auth"
	platformmiddleware "sdms/internal/platform/http/middleware"
	"sdms/internal/platform/http/routes"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, configs ...*config.Config) *fiber.App {
	cfg := config.Load()
	if len(configs) > 0 && configs[0] != nil {
		cfg = configs[0]
	}

	bodyLimit := cfg.Upload.MaxSizeBytes + (1 * 1024 * 1024)
	app := fiber.New(fiber.Config{AppName: "School Document Management System", BodyLimit: int(bodyLimit)})
	app.Use(cors.New(cors.Config{
		AllowOrigins:  cfg.App.AllowedOrigins,
		AllowMethods:  []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete, fiber.MethodOptions, fiber.MethodHead},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept", "Authorization", "Range"},
		ExposeHeaders: []string{"Content-Disposition", "Content-Length", "Content-Range", "Accept-Ranges"},
		MaxAge:        3600,
	}))

	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.Get("/health", healthhttp.NewHandler(db).Health)

	userRepository := userpostgres.NewUserRepository(db)
	tokens := platformauth.NewTokenManager(cfg.Auth.JWTSecret, time.Duration(cfg.Auth.TokenTTLHours)*time.Hour)
	authMiddleware := platformmiddleware.NewAuth(userRepository, tokens)

	adminOnly := authMiddleware.RequireRoles(userdomain.RoleAdmin)
	topicManager := authMiddleware.RequireRoles(userdomain.RoleAdmin, userdomain.RoleDirector)
	userManager := adminOnly
	teacherOnly := authMiddleware.RequireRoles(userdomain.RoleTeacher)
	reviewerOnly := authMiddleware.RequireRoles(userdomain.RoleAdmin, userdomain.RoleDirector, userdomain.RoleQA)

	routes.NewRouteAuth(v1, db, tokens, authMiddleware.RequireAuth)
	protected := v1.Group("", authMiddleware.RequireAuth)
	protected.Use(audit.Middleware(db))

	routes.NewRouteTopic(protected, db, topicManager)
	routes.NewRouteUser(protected, db, userManager, reviewerOnly)
	routes.NewRouteSubmission(protected, db, cfg.Upload, teacherOnly, reviewerOnly)
	audit.RegisterRoutes(protected, db, adminOnly)

	return app
}
