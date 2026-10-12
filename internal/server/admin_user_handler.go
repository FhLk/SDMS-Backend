package server

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type adminUserService interface {
	ListUsers(context.Context) ([]model.User, error)
	CreateUser(context.Context, admin.CreateUserInput) (model.User, error)
	UpdateUser(context.Context, uuid.UUID, admin.UpdateUserInput) (model.User, error)
	ResetPassword(context.Context, uuid.UUID, string) error
	ImportUsers(context.Context, string, []byte) (admin.ImportResult, error)
}

const maximumUserImportSize = 3 << 20

type adminUserHandler struct {
	service adminUserService
}

type createUserRequest struct {
	Username      string      `json:"username"`
	Password      string      `json:"password"`
	FullName      string      `json:"full_name"`
	Position      string      `json:"position"`
	DivisionUUIDs []uuid.UUID `json:"division_uuids"`
	IsExec        bool        `json:"is_exec"`
	IsAdmin       bool        `json:"is_admin"`
}

type updateUserRequest struct {
	Username      *string      `json:"username"`
	FullName      *string      `json:"full_name"`
	Position      *string      `json:"position"`
	DivisionUUIDs *[]uuid.UUID `json:"division_uuids"`
	IsExec        *bool        `json:"is_exec"`
	IsAdmin       *bool        `json:"is_admin"`
	IsActive      *bool        `json:"is_active"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

type adminUserResponse struct {
	UUID          uuid.UUID          `json:"uuid"`
	Username      string             `json:"username"`
	FullName      string             `json:"full_name"`
	Position      string             `json:"position"`
	IsExec        bool               `json:"is_exec"`
	IsAdmin       bool               `json:"is_admin"`
	IsActive      bool               `json:"is_active"`
	Divisions     []divisionResponse `json:"divisions"`
	DeactivatedAt *time.Time         `json:"deactivated_at"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// list คืนผู้ใช้ทั้งที่เปิดและปิดใช้งาน โดยไม่ส่ง password hash ออกทาง API.
func (h adminUserHandler) list(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "User administration service unavailable")
	}

	users, err := h.service.ListUsers(c.Context())
	if err != nil {
		return mapAdminUserError(err)
	}
	responses := make([]adminUserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, newAdminUserResponse(user))
	}
	return c.Status(fiber.StatusOK).JSON(responses)
}

// create ตรวจ request แล้วส่งต่อให้ service บันทึกผู้ใช้ ฝ่าย และรหัสผ่านแบบ hash.
func (h adminUserHandler) create(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "User administration service unavailable")
	}

	var request createUserRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	user, err := h.service.CreateUser(c.Context(), admin.CreateUserInput{
		Username:      request.Username,
		Password:      request.Password,
		FullName:      request.FullName,
		Position:      request.Position,
		DivisionUUIDs: request.DivisionUUIDs,
		IsExec:        request.IsExec,
		IsAdmin:       request.IsAdmin,
	})
	if err != nil {
		return mapAdminUserError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(newAdminUserResponse(user))
}

// update ใช้ pointer fields เพื่อให้ PATCH เปลี่ยนเฉพาะค่าที่ client ส่งมา.
func (h adminUserHandler) update(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "User administration service unavailable")
	}

	userUUID, err := parseUserUUID(c)
	if err != nil {
		return err
	}
	var request updateUserRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	user, err := h.service.UpdateUser(c.Context(), userUUID, admin.UpdateUserInput{
		Username:      request.Username,
		FullName:      request.FullName,
		Position:      request.Position,
		DivisionUUIDs: request.DivisionUUIDs,
		IsExec:        request.IsExec,
		IsAdmin:       request.IsAdmin,
		IsActive:      request.IsActive,
	})
	if err != nil {
		return mapAdminUserError(err)
	}
	return c.Status(fiber.StatusOK).JSON(newAdminUserResponse(user))
}

// resetPassword เปลี่ยนรหัสผ่านผ่าน endpoint แยก เพื่อไม่ให้ PATCH ทั่วไป
// เปลี่ยน credential โดยไม่ได้ตั้งใจ; service จะยกเลิก session เดิมทั้งหมด.
func (h adminUserHandler) resetPassword(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "User administration service unavailable")
	}

	userUUID, err := parseUserUUID(c)
	if err != nil {
		return err
	}
	var request resetPasswordRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	if err := h.service.ResetPassword(c.Context(), userUUID, request.Password); err != nil {
		return mapAdminUserError(err)
	}
	return c.Status(fiber.StatusOK).JSON(messageResponse{Message: "Password updated"})
}

// importUsers รับไฟล์ multipart field ชื่อ file และจำกัดขนาดก่อนอ่านเข้า memory.
// การตรวจทุกแถวและ transaction แบบ all-or-nothing อยู่ใน service เพื่อให้ HTTP handler
// ทำหน้าที่เฉพาะแปลง request/response และไม่ผูกกฎธุรกิจกับ Fiber.
func (h adminUserHandler) importUsers(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "User administration service unavailable")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return apperror.New(fiber.StatusBadRequest, "FILE_REQUIRED", "Import file is required")
	}
	if fileHeader.Size > maximumUserImportSize {
		return apperror.New(fiber.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "Import file must not exceed 3 MiB")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_FILE", "Cannot open import file", err)
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(io.LimitReader(file, maximumUserImportSize+1))
	if err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_FILE", "Cannot read import file", err)
	}
	if len(content) > maximumUserImportSize {
		return apperror.New(fiber.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "Import file must not exceed 3 MiB")
	}

	result, err := h.service.ImportUsers(c.Context(), fileHeader.Filename, content)
	if err != nil {
		return mapAdminUserError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// downloadImportTemplate ส่งไฟล์ตัวอย่างที่สร้างจาก header ชุดเดียวกับ parser
// ทำให้ CSV และ XLSX ไม่หลุดจากรูปแบบที่ระบบรองรับเมื่อมีการแก้ไขในอนาคต.
func (h adminUserHandler) downloadImportTemplate(c fiber.Ctx, format string) error {
	content, contentType, filename, err := admin.BuildUserImportTemplate(format)
	if err != nil {
		return mapAdminUserError(err)
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Attachment(filename)
	return c.Status(fiber.StatusOK).Send(content)
}

func (h adminUserHandler) downloadCSVImportTemplate(c fiber.Ctx) error {
	return h.downloadImportTemplate(c, "csv")
}

func (h adminUserHandler) downloadXLSXImportTemplate(c fiber.Ctx) error {
	return h.downloadImportTemplate(c, "xlsx")
}

func parseUserUUID(c fiber.Ctx) (uuid.UUID, error) {
	parsed, err := uuid.Parse(c.Params("uuid"))
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, apperror.New(fiber.StatusBadRequest, "INVALID_UUID", "Invalid user UUID")
	}
	return parsed, nil
}

func mapAdminUserError(err error) error {
	var importValidationErr *admin.ImportValidationError
	var validationErr *admin.ValidationError
	switch {
	case errors.As(err, &importValidationErr):
		return apperror.WrapWithDetails(
			fiber.StatusBadRequest,
			"IMPORT_VALIDATION_FAILED",
			"Import file contains invalid rows",
			map[string]any{"rows": importValidationErr.Rows},
			err,
		)
	case errors.Is(err, admin.ErrUnsupportedImportFile):
		return apperror.New(fiber.StatusBadRequest, "UNSUPPORTED_FILE_TYPE", "Only .csv and .xlsx files are supported")
	case errors.As(err, &validationErr):
		return apperror.Wrap(fiber.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message, err)
	case errors.Is(err, admin.ErrDivisionNotFound):
		return apperror.New(fiber.StatusBadRequest, "INVALID_DIVISION", "One or more divisions do not exist")
	case errors.Is(err, admin.ErrUsernameExists):
		return apperror.New(fiber.StatusConflict, "USERNAME_EXISTS", "Username already exists")
	case errors.Is(err, admin.ErrUserNotFound):
		return apperror.New(fiber.StatusNotFound, "USER_NOT_FOUND", "User not found")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newAdminUserResponse(user model.User) adminUserResponse {
	return adminUserResponse{
		UUID:          user.UUID,
		Username:      user.Username,
		FullName:      user.FullName,
		Position:      user.Position,
		IsExec:        user.IsExec,
		IsAdmin:       user.IsAdmin,
		IsActive:      user.DeactivatedAt == nil,
		Divisions:     divisionResponses(user.Divisions),
		DeactivatedAt: user.DeactivatedAt,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}
