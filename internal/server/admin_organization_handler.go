package server

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type adminOrganizationService interface {
	SetDivisionHead(context.Context, string, uuid.UUID) (model.Division, error)
	ListAcademicYears(context.Context) ([]admin.AcademicYearSummary, error)
	CreateAcademicYear(context.Context, admin.CreateAcademicYearInput) (admin.AcademicYearSummary, error)
	SetCurrentAcademicYear(context.Context, admin.SetCurrentAcademicYearInput) (admin.AcademicYearSummary, error)
}

type adminOrganizationHandler struct {
	service adminOrganizationService
}

type setDivisionHeadRequest struct {
	UserUUID uuid.UUID `json:"user_uuid"`
}

type createAcademicYearRequest struct {
	Year            int16  `json:"year"`
	CurrentSemester *int16 `json:"current_semester"`
}

type setCurrentAcademicYearRequest struct {
	CurrentSemester *int16 `json:"current_semester"`
}

type divisionHeadUserResponse struct {
	UUID      uuid.UUID `json:"uuid"`
	Username  string    `json:"username"`
	Prefix    string    `json:"prefix"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	FullName  string    `json:"full_name"`
	Position  string    `json:"position"`
}

type adminDivisionResponse struct {
	UUID      uuid.UUID                 `json:"uuid"`
	Code      string                    `json:"code"`
	Name      string                    `json:"name"`
	ShortName string                    `json:"short_name"`
	SortOrder int16                     `json:"sort_order"`
	HeadUser  *divisionHeadUserResponse `json:"head_user"`
}

// setDivisionHead รับ UUID ของบุคลากรคนใหม่ ส่วน service จะตรวจบทบาท
// และแทนที่หัวหน้าคนเดิมภายใน transaction.
func (h adminOrganizationHandler) setDivisionHead(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Organization administration service unavailable")
	}

	var request setDivisionHeadRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	division, err := h.service.SetDivisionHead(c.Context(), c.Params("code"), request.UserUUID)
	if err != nil {
		return mapAdminOrganizationError(err)
	}
	return c.Status(fiber.StatusOK).JSON(newAdminDivisionResponse(division))
}

// listAcademicYears คืนทุกปีเรียงจากใหม่ไปเก่า พร้อมจำนวนหัวข้อของแต่ละปี.
func (h adminOrganizationHandler) listAcademicYears(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Organization administration service unavailable")
	}

	years, err := h.service.ListAcademicYears(c.Context())
	if err != nil {
		return mapAdminOrganizationError(err)
	}
	return c.Status(fiber.StatusOK).JSON(years)
}

// createAcademicYear สร้างปีใหม่โดยยังไม่เปลี่ยนปีปัจจุบัน
// เพื่อไม่ให้การสร้างข้อมูลธรรมดามี side effect ต่อหัวข้อใหม่โดยไม่ตั้งใจ.
func (h adminOrganizationHandler) createAcademicYear(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Organization administration service unavailable")
	}

	var request createAcademicYearRequest
	if err := c.Bind().Body(&request); err != nil {
		return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
	}
	year, err := h.service.CreateAcademicYear(c.Context(), admin.CreateAcademicYearInput{
		Year:            request.Year,
		CurrentSemester: request.CurrentSemester,
	})
	if err != nil {
		return mapAdminOrganizationError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(year)
}

// setCurrentAcademicYear ยอมให้ไม่ส่ง body เพื่อใช้ภาคเรียนเดิมของปีเป้าหมาย
// หรือส่ง current_semester เพื่อสลับปีและภาคเรียนในคำขอเดียว.
func (h adminOrganizationHandler) setCurrentAcademicYear(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Organization administration service unavailable")
	}

	yearNumber, err := parseAcademicYear(c.Params("year"))
	if err != nil {
		return err
	}
	var request setCurrentAcademicYearRequest
	if len(bytes.TrimSpace(c.Body())) > 0 {
		if err := c.Bind().Body(&request); err != nil {
			return apperror.Wrap(fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err)
		}
	}

	year, err := h.service.SetCurrentAcademicYear(c.Context(), admin.SetCurrentAcademicYearInput{
		Year:            yearNumber,
		CurrentSemester: request.CurrentSemester,
	})
	if err != nil {
		return mapAdminOrganizationError(err)
	}
	return c.Status(fiber.StatusOK).JSON(year)
}

func parseAcademicYear(value string) (int16, error) {
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return 0, apperror.New(fiber.StatusBadRequest, "INVALID_YEAR", "Invalid academic year")
	}
	return int16(parsed), nil
}

func mapAdminOrganizationError(err error) error {
	var validationErr *admin.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return apperror.Wrap(fiber.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message, err)
	case errors.Is(err, admin.ErrDivisionNotFound):
		return apperror.New(fiber.StatusNotFound, "DIVISION_NOT_FOUND", "Division not found")
	case errors.Is(err, admin.ErrUserNotFound):
		return apperror.New(fiber.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, admin.ErrExecutiveCannotHead):
		return apperror.New(fiber.StatusConflict, "EXECUTIVE_CANNOT_BE_DIVISION_HEAD", "Executive users cannot be division heads")
	case errors.Is(err, admin.ErrInactiveUserCannotHead):
		return apperror.New(fiber.StatusConflict, "INACTIVE_USER_CANNOT_BE_DIVISION_HEAD", "Inactive users cannot be division heads")
	case errors.Is(err, admin.ErrUserAlreadyDivisionHead):
		return apperror.New(fiber.StatusConflict, "USER_ALREADY_DIVISION_HEAD", "User is already the head of another division")
	case errors.Is(err, admin.ErrAcademicYearExists):
		return apperror.New(fiber.StatusConflict, "ACADEMIC_YEAR_EXISTS", "Academic year already exists")
	case errors.Is(err, admin.ErrAcademicYearNotFound):
		return apperror.New(fiber.StatusNotFound, "ACADEMIC_YEAR_NOT_FOUND", "Academic year not found")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newAdminDivisionResponse(division model.Division) adminDivisionResponse {
	response := adminDivisionResponse{
		UUID:      division.UUID,
		Code:      division.Code,
		Name:      division.Name,
		ShortName: division.ShortName,
		SortOrder: division.SortOrder,
	}
	if division.HeadUser != nil {
		response.HeadUser = &divisionHeadUserResponse{
			UUID:      division.HeadUser.UUID,
			Username:  division.HeadUser.Username,
			Prefix:    division.HeadUser.Prefix,
			FirstName: division.HeadUser.FirstName,
			LastName:  division.HeadUser.LastName,
			FullName:  division.HeadUser.DisplayName(),
			Position:  division.HeadUser.Position,
		}
	}
	return response
}
