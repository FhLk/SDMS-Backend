package server

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"strconv"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/portfolio"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type portfolioService interface {
	List(context.Context, portfolio.ListInput) (portfolio.Portfolio, error)
	OpenArchive(context.Context, portfolio.ArchiveInput) (portfolio.Archive, error)
}

type portfolioHandler struct {
	service portfolioService
}

type portfolioFileResponse struct {
	UUID         uuid.UUID `json:"uuid"`
	FieldUUID    uuid.UUID `json:"field_uuid"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Position     int16     `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
}

type portfolioSubmissionResponse struct {
	UUID        uuid.UUID               `json:"uuid"`
	TopicUUID   uuid.UUID               `json:"topic_uuid"`
	TopicTitle  string                  `json:"topic_title"`
	Division    divisionResponse        `json:"division"`
	Semester    int16                   `json:"semester"`
	DueDate     string                  `json:"due_date"`
	IsOpen      bool                    `json:"is_open"`
	IsSecret    bool                    `json:"is_secret"`
	Values      json.RawMessage         `json:"values"`
	Status      string                  `json:"status"`
	SubmittedAt time.Time               `json:"submitted_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	Files       []portfolioFileResponse `json:"files"`
}

type portfolioYearResponse struct {
	AcademicYear int16                         `json:"academic_year"`
	Submissions  []portfolioSubmissionResponse `json:"submissions"`
}

type portfolioResponse struct {
	Years []portfolioYearResponse `json:"years"`
}

// list ใช้ UUID จาก session เท่านั้น แล้วส่งตัวกรองปีและคำค้นให้ service
// จึงไม่มีช่องทางให้ client ขอประวัติการส่งของผู้ใช้อื่น.
func (h portfolioHandler) list(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Portfolio service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	year, err := parseOptionalPortfolioYear(c.Query("year"))
	if err != nil {
		return mapPortfolioError(err)
	}
	result, err := h.service.List(c.Context(), portfolio.ListInput{
		UserUUID: profile.User.UUID,
		Year:     year,
		Query:    c.Query("q"),
	})
	if err != nil {
		return mapPortfolioError(err)
	}
	return c.JSON(newPortfolioResponse(result))
}

// downloadArchive ตั้ง response เป็นไฟล์แนบและส่ง pipe จาก service ให้ Fiber โดยตรง.
// fasthttp จะปิด ReadCloser หลังส่งเสร็จหรือ client ตัดการเชื่อมต่อ ทำให้ goroutine หยุดตามไปด้วย.
func (h portfolioHandler) downloadArchive(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Portfolio service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	year, err := parseOptionalPortfolioYear(c.Query("year"))
	if err != nil {
		return mapPortfolioError(err)
	}
	archive, err := h.service.OpenArchive(c.Context(), portfolio.ArchiveInput{
		UserUUID: profile.User.UUID,
		Year:     year,
	})
	if err != nil {
		return mapPortfolioError(err)
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": archive.Filename})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, disposition)
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if err := c.SendStream(archive.Content); err != nil {
		_ = archive.Content.Close()
		return err
	}
	return nil
}

func parseOptionalPortfolioYear(value string) (*int16, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return nil, &portfolio.ValidationError{Field: "year", Message: "academic year must be a number"}
	}
	year := int16(parsed)
	return &year, nil
}

func mapPortfolioError(err error) error {
	var validationErr *portfolio.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return apperror.WrapWithDetails(
			fiber.StatusBadRequest,
			"VALIDATION_ERROR",
			validationErr.Message,
			map[string]any{"field": validationErr.Field},
			err,
		)
	case errors.Is(err, portfolio.ErrInvalidUser):
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newPortfolioResponse(result portfolio.Portfolio) portfolioResponse {
	years := make([]portfolioYearResponse, 0, len(result.Years))
	for _, year := range result.Years {
		submissions := make([]portfolioSubmissionResponse, 0, len(year.Submissions))
		for _, item := range year.Submissions {
			files := make([]portfolioFileResponse, 0, len(item.Files))
			for _, file := range item.Files {
				files = append(files, portfolioFileResponse{
					UUID:         file.UUID,
					FieldUUID:    file.FieldUUID,
					OriginalName: file.OriginalName,
					MimeType:     file.MimeType,
					SizeBytes:    file.SizeBytes,
					Position:     file.Position,
					CreatedAt:    file.CreatedAt,
				})
			}
			submissions = append(submissions, portfolioSubmissionResponse{
				UUID:       item.UUID,
				TopicUUID:  item.TopicUUID,
				TopicTitle: item.TopicTitle,
				Division: divisionResponse{
					UUID:      item.Division.UUID,
					Code:      item.Division.Code,
					Name:      item.Division.Name,
					ShortName: item.Division.ShortName,
				},
				Semester:    item.Semester,
				DueDate:     item.DueDate.Format("2006-01-02"),
				IsOpen:      item.IsOpen,
				IsSecret:    item.IsSecret,
				Values:      item.Values,
				Status:      item.Status,
				SubmittedAt: item.SubmittedAt,
				UpdatedAt:   item.UpdatedAt,
				Files:       files,
			})
		}
		years = append(years, portfolioYearResponse{
			AcademicYear: year.AcademicYear,
			Submissions:  submissions,
		})
	}
	return portfolioResponse{Years: years}
}
