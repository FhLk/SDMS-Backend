package http

import (
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		db: db,
	}
}

func (h *Handler) Health(c fiber.Ctx) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":   "error",
			"database": "unavailable",
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":   "error",
			"database": "unavailable",
		})
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"database": "connected",
		"service":  "School Document Management System",
	})
}
