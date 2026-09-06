package http

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	router.Post("/auth/login", handler.Login)
	router.Get("/auth/me", requireAuth, handler.Me)
	router.Patch("/auth/password", requireAuth, handler.ChangePassword)
}
