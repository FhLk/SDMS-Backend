package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	searchdomain "github.com/FhLk/SDMS-Backend/internal/search"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type searchService interface {
	Search(context.Context, searchdomain.Input) (searchdomain.Result, error)
}

type searchHandler struct {
	service searchService
}

type searchTopicResponse struct {
	UUID         uuid.UUID        `json:"uuid"`
	Title        string           `json:"title"`
	Division     divisionResponse `json:"division"`
	AcademicYear int16            `json:"academic_year"`
	Semester     int16            `json:"semester"`
	DueDate      string           `json:"due_date"`
	IsOpen       bool             `json:"is_open"`
	IsSecret     bool             `json:"is_secret"`
}

type searchUserResponse struct {
	UUID     uuid.UUID `json:"uuid"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Position string    `json:"position"`
}

type searchFileResponse struct {
	UUID         uuid.UUID `json:"uuid"`
	FieldUUID    uuid.UUID `json:"field_uuid"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Position     int16     `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
}

type searchItemResponse struct {
	SubmissionUUID uuid.UUID            `json:"submission_uuid"`
	Topic          searchTopicResponse  `json:"topic"`
	SubmittedBy    searchUserResponse   `json:"submitted_by"`
	Values         json.RawMessage      `json:"values"`
	Files          []searchFileResponse `json:"files"`
	Status         string               `json:"status"`
	SubmittedAt    time.Time            `json:"submitted_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type searchPaginationResponse struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

type searchResponse struct {
	Items      []searchItemResponse     `json:"items"`
	Pagination searchPaginationResponse `json:"pagination"`
}

// search รับตัวกรองจาก query string แต่ใช้ profile จาก session เท่านั้น
// เพื่อให้ service คำนวณ scope ของหัวข้อลับก่อน count และ pagination.
func (h searchHandler) search(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Search service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	input, err := parseSearchInput(c, profile)
	if err != nil {
		return mapSearchError(err)
	}
	result, err := h.service.Search(c.Context(), input)
	if err != nil {
		return mapSearchError(err)
	}
	return c.JSON(newSearchResponse(result))
}

func parseSearchInput(c fiber.Ctx, profile auth.Profile) (searchdomain.Input, error) {
	year, err := parseOptionalSearchInt16(c.Query("year"), "year", "academic year must be a number")
	if err != nil {
		return searchdomain.Input{}, err
	}
	semester, err := parseOptionalSearchInt16(c.Query("semester"), "semester", "semester must be a number")
	if err != nil {
		return searchdomain.Input{}, err
	}
	by, err := parseOptionalSearchUUID(c.Query("by"))
	if err != nil {
		return searchdomain.Input{}, err
	}
	page, err := parseSearchPageValue(c.Query("page"), "page", 1, 1_000_000)
	if err != nil {
		return searchdomain.Input{}, err
	}
	pageSize, err := parseSearchPageValue(c.Query("page_size"), "page_size", searchdomain.DefaultPageSize, searchdomain.MaxPageSize)
	if err != nil {
		return searchdomain.Input{}, err
	}
	return searchdomain.Input{
		Profile:  profile,
		Query:    c.Query("q"),
		Division: c.Query("division"),
		Year:     year,
		Semester: semester,
		By:       by,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func parseOptionalSearchInt16(value, field, message string) (*int16, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return nil, &searchdomain.ValidationError{Field: field, Message: message}
	}
	result := int16(parsed)
	return &result, nil
}

func parseOptionalSearchUUID(value string) (*uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return nil, &searchdomain.ValidationError{Field: "by", Message: "by must be a valid user UUID"}
	}
	return &parsed, nil
}

func parseSearchPageValue(value, field string, defaultValue, maximum int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 || parsed > int64(maximum) {
		return 0, &searchdomain.ValidationError{
			Field:   field,
			Message: field + " must be between 1 and " + strconv.Itoa(maximum),
		}
	}
	return int(parsed), nil
}

func mapSearchError(err error) error {
	var validationErr *searchdomain.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return apperror.WrapWithDetails(
			fiber.StatusBadRequest,
			"VALIDATION_ERROR",
			validationErr.Message,
			map[string]any{"field": validationErr.Field},
			err,
		)
	case errors.Is(err, searchdomain.ErrInvalidProfile):
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newSearchResponse(result searchdomain.Result) searchResponse {
	items := make([]searchItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		files := make([]searchFileResponse, 0, len(item.Files))
		for _, file := range item.Files {
			files = append(files, searchFileResponse{
				UUID:         file.UUID,
				FieldUUID:    file.FieldUUID,
				OriginalName: file.OriginalName,
				MimeType:     file.MimeType,
				SizeBytes:    file.SizeBytes,
				Position:     file.Position,
				CreatedAt:    file.CreatedAt,
			})
		}
		items = append(items, searchItemResponse{
			SubmissionUUID: item.SubmissionUUID,
			Topic: searchTopicResponse{
				UUID:  item.Topic.UUID,
				Title: item.Topic.Title,
				Division: divisionResponse{
					UUID:      item.Topic.Division.UUID,
					Code:      item.Topic.Division.Code,
					Name:      item.Topic.Division.Name,
					ShortName: item.Topic.Division.ShortName,
				},
				AcademicYear: item.Topic.AcademicYear,
				Semester:     item.Topic.Semester,
				DueDate:      item.Topic.DueDate.Format("2006-01-02"),
				IsOpen:       item.Topic.IsOpen,
				IsSecret:     item.Topic.IsSecret,
			},
			SubmittedBy: searchUserResponse{
				UUID:     item.SubmittedBy.UUID,
				Username: item.SubmittedBy.Username,
				FullName: item.SubmittedBy.FullName,
				Position: item.SubmittedBy.Position,
			},
			Values:      item.Values,
			Files:       files,
			Status:      item.Status,
			SubmittedAt: item.SubmittedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return searchResponse{
		Items: items,
		Pagination: searchPaginationResponse{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	}
}
