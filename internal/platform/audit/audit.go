package audit

import (
	"strconv"
	"strings"
	"time"

	platformmiddleware "sdms/internal/platform/http/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LogModel struct {
	UID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"uid"`
	UserUID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_uid"`
	Method     string    `gorm:"type:varchar(16);not null;index" json:"method"`
	Path       string    `gorm:"type:text;not null;index" json:"path"`
	StatusCode int       `gorm:"not null;index" json:"status_code"`
	IPAddress  string    `gorm:"type:varchar(100)" json:"ip_address"`
	CreatedAt  time.Time `gorm:"not null;index" json:"created_at"`
}

func (LogModel) TableName() string { return "audit_logs" }

// Middleware records every authenticated API access. This is intentionally
// best-effort: an audit insert failure must not break the user's primary request.
func Middleware(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()
		user, ok := platformmiddleware.CurrentUser(c)
		if !ok {
			return err
		}
		status := c.Response().StatusCode()
		if status == 0 {
			status = fiber.StatusOK
		}
		_ = db.WithContext(c.Context()).Create(&LogModel{
			UID: uuid.New(), UserUID: user.UID, Method: c.Method(), Path: c.Path(),
			StatusCode: status, IPAddress: c.IP(),
		}).Error
		return err
	}
}

func RegisterRoutes(router fiber.Router, db *gorm.DB, adminOnly fiber.Handler) {
	router.Get("/audit-logs", adminOnly, func(c fiber.Ctx) error {
		limit := 100
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
				limit = parsed
			}
		}
		query := db.WithContext(c.Context()).Model(&LogModel{})
		if raw := strings.TrimSpace(c.Query("user_uid")); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid user_uid"})
			}
			query = query.Where("user_uid = ?", id)
		}
		if method := strings.TrimSpace(c.Query("method")); method != "" {
			query = query.Where("method = ?", strings.ToUpper(method))
		}
		var logs []LogModel
		if err := query.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "internal server error"})
		}
		return c.JSON(logs)
	})
}
