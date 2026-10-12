package server

import (
	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// requestContextKey เป็นชนิด key ภายใน package เพื่อไม่ให้ชนกับ Locals
// ที่ middleware หรือ library อื่นอาจบันทึกด้วย string ชื่อเดียวกัน.
type requestContextKey string

const authenticatedProfileKey requestContextKey = "authenticated-profile"

// permissionCheck รับ profile ที่ผ่านการตรวจ session แล้วและคืนผลการอนุญาต.
// รูปแบบ function นี้ทำให้ route นำ policy กลางมาเสียบใช้ได้โดยไม่ผูก policy กับ Fiber.
type permissionCheck func(auth.Profile) bool

// authenticate ตรวจ session cookie หนึ่งครั้งต่อ request แล้วเก็บ profile ไว้ใน Locals.
// Handler และ middleware ตัวถัดไปจึงใช้ข้อมูลชุดเดียวกันโดยไม่ query ฐานข้อมูลซ้ำ.
func (h authHandler) authenticate(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Authentication service unavailable")
	}

	profile, err := h.service.CurrentUser(c.Context(), c.Cookies(h.config.CookieName))
	if err != nil {
		return mapAuthError(err)
	}
	// ปฏิเสธ profile ว่างแม้ service จะไม่คืน error เพื่อไม่ให้ข้อมูลผิดปกติ
	// หลุดผ่านขอบเขต authentication ไปยัง handler ที่มีข้อมูลสำคัญ.
	if profile.User.UUID == uuid.Nil {
		return mapAuthError(auth.ErrUnauthenticated)
	}

	c.Locals(authenticatedProfileKey, profile)
	return c.Next()
}

// requirePermission สร้าง middleware สำหรับกฎสิทธิ์ของ route นั้น.
// ถ้ายังไม่ผ่าน authenticate จะตอบ 401 แต่ถ้าเข้าสู่ระบบแล้วและสิทธิ์ไม่พอจะตอบ 403.
func requirePermission(check permissionCheck) fiber.Handler {
	return func(c fiber.Ctx) error {
		profile, ok := authenticatedProfile(c)
		if !ok {
			return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		}
		if check == nil || !check(profile) {
			return apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
		}

		return c.Next()
	}
}

// authenticatedProfile อ่าน profile ที่ authenticate บันทึกไว้ให้ handler ใช้งานต่อ.
func authenticatedProfile(c fiber.Ctx) (auth.Profile, bool) {
	profile, ok := c.Locals(authenticatedProfileKey).(auth.Profile)
	return profile, ok
}
