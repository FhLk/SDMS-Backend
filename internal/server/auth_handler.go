package server

import (
	"context"
	"errors"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type authService interface {
	Login(context.Context, auth.LoginInput) (auth.LoginResult, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (auth.Profile, error)
}

// Dependencies contains services required by HTTP handlers.
type Dependencies struct {
	Auth              authService
	AuthConfig        config.AuthConfig
	UserAdmin         adminUserService
	OrganizationAdmin adminOrganizationService
	Topics            topicService
	Uploads           uploadService
	Submissions       submissionService
	Home              homeService
	Portfolio         portfolioService
}

type authHandler struct {
	service authService
	config  config.AuthConfig
}

type loginRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type messageResponse struct {
	Message string `json:"message"`
}

type divisionResponse struct {
	UUID      uuid.UUID `json:"uuid"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	ShortName string    `json:"short_name"`
}

type meResponse struct {
	UUID          uuid.UUID          `json:"uuid"`
	Username      string             `json:"username"`
	FullName      string             `json:"full_name"`
	Position      string             `json:"position"`
	Role          string             `json:"role"`
	Divisions     []divisionResponse `json:"divisions"`
	HeadDivisions []divisionResponse `json:"head_divisions"`
}

// login validates the request, delegates credential checks to the auth service,
// and returns the opaque session token only through a protected browser cookie.
func (h authHandler) login(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Authentication service unavailable")
	}

	var request loginRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	if request.Username == "" || request.Password == "" {
		return apperror.New(fiber.StatusBadRequest, "VALIDATION_ERROR", "Username and password are required")
	}

	result, err := h.service.Login(c.Context(), auth.LoginInput{
		Username:  request.Username,
		Password:  request.Password,
		IP:        c.IP(),
		UserAgent: c.Get(fiber.HeaderUserAgent),
	})
	if err != nil {
		return mapAuthError(err)
	}

	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	return c.Status(fiber.StatusOK).JSON(messageResponse{Message: "Logged in"})
}

// logout revokes the server-side session and expires its matching browser cookie.
func (h authHandler) logout(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Authentication service unavailable")
	}

	token := c.Cookies(h.config.CookieName)
	// Clear the browser cookie even if revoking the database session later fails.
	h.clearSessionCookie(c)
	if err := h.service.Logout(c.Context(), token); err != nil {
		return mapAuthError(err)
	}
	return c.Status(fiber.StatusOK).JSON(messageResponse{Message: "Logged out"})
}

// me นำ profile ที่ middleware ตรวจสอบแล้วมาสร้าง response โดยไม่ query ซ้ำ.
func (h authHandler) me(c fiber.Ctx) error {
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	return c.Status(fiber.StatusOK).JSON(newMeResponse(profile))
}

// setSessionCookie keeps the token inaccessible to JavaScript and limits normal
// cross-site sending with SameSite=Lax. Secure is enabled through configuration.
func (h authHandler) setSessionCookie(c fiber.Ctx, token string, expiresAt time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     h.config.CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.config.SessionTTL / time.Second),
		Expires:  expiresAt,
		Secure:   h.config.CookieSecure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// clearSessionCookie repeats the original cookie scope so browsers remove the
// same cookie instead of creating an expired cookie under a different scope.
func (h authHandler) clearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     h.config.CookieName,
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		Secure:   h.config.CookieSecure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// mapAuthError exposes stable, non-sensitive API errors while retaining a
// generic response for unexpected database or infrastructure failures.
func mapAuthError(err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return apperror.New(fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password")
	case errors.Is(err, auth.ErrUnauthenticated):
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newMeResponse(profile auth.Profile) meResponse {
	return meResponse{
		UUID:          profile.User.UUID,
		Username:      profile.User.Username,
		FullName:      profile.User.FullName,
		Position:      profile.User.Position,
		Role:          primaryRole(profile),
		Divisions:     divisionResponses(profile.Divisions),
		HeadDivisions: divisionResponses(profile.HeadDivisions),
	}
}

func primaryRole(profile auth.Profile) string {
	// A user may satisfy several role rules. Use the most privileged role as the
	// primary label while still returning headed divisions separately.
	switch {
	case profile.User.IsAdmin:
		return "admin"
	case profile.User.IsExec:
		return "executive"
	case len(profile.HeadDivisions) > 0:
		return "division_head"
	default:
		return "teacher"
	}
}

func divisionResponses(divisions []model.Division) []divisionResponse {
	responses := make([]divisionResponse, 0, len(divisions))
	for _, division := range divisions {
		responses = append(responses, divisionResponse{
			UUID:      division.UUID,
			Code:      division.Code,
			Name:      division.Name,
			ShortName: division.ShortName,
		})
	}
	return responses
}
