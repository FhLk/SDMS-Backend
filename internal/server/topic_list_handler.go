package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/topic"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type topicListDivisionResponse struct {
	UUID      uuid.UUID `json:"uuid"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	ShortName string    `json:"short_name"`
	SortOrder int16     `json:"sort_order"`
}

type topicListAcademicYearResponse struct {
	UUID      uuid.UUID `json:"uuid"`
	Year      int16     `json:"year"`
	IsCurrent bool      `json:"is_current"`
}

type topicListItemResponse struct {
	UUID           uuid.UUID  `json:"uuid"`
	Semester       int16      `json:"semester"`
	Title          string     `json:"title"`
	DueDate        string     `json:"due_date"`
	IsOpen         bool       `json:"is_open"`
	IsSecret       bool       `json:"is_secret"`
	CreatedByUUID  uuid.UUID  `json:"created_by_uuid"`
	SubmittedCount int64      `json:"submitted_count"`
	AssigneeCount  int64      `json:"assignee_count"`
	IsAssigned     bool       `json:"is_assigned"`
	MyStatus       *string    `json:"my_status"`
	MySubmittedAt  *time.Time `json:"my_submitted_at"`
	ClosedAt       *time.Time `json:"closed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type divisionTopicsResponse struct {
	Division     topicListDivisionResponse     `json:"division"`
	AcademicYear topicListAcademicYearResponse `json:"academic_year"`
	Topics       []topicListItemResponse       `json:"topics"`
}

// listDivisions คืนฝ่ายทุกฝ่ายให้ผู้ใช้ที่เข้าสู่ระบบ โดยเรียงตามลำดับกลางจากฐานข้อมูล.
func (h topicHandler) listDivisions(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Topic service unavailable")
	}
	divisions, err := h.service.ListDivisions(c.Context())
	if err != nil {
		return mapTopicError(err)
	}
	response := make([]topicListDivisionResponse, 0, len(divisions))
	for _, division := range divisions {
		response = append(response, newTopicListDivisionResponse(division))
	}
	return c.JSON(response)
}

// listDivisionTopics ใช้ user UUID จาก session เพื่อให้ service คำนวณสถานะของฉัน
// โดยไม่ยอมรับ user UUID จาก query ซึ่งอาจถูกปลอมแปลงได้.
func (h topicHandler) listDivisionTopics(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Topic service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	year, err := parseOptionalTopicYear(c.Query("year"))
	if err != nil {
		return mapTopicError(err)
	}
	result, err := h.service.ListDivisionTopics(c.Context(), c.Params("code"), year, profile.User.UUID)
	if err != nil {
		return mapTopicError(err)
	}
	return c.JSON(newDivisionTopicsResponse(result))
}

func parseOptionalTopicYear(value string) (*int16, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return nil, &admin.ValidationError{Field: "year", Message: "academic year must be a number"}
	}
	year := int16(parsed)
	return &year, nil
}

func newTopicListDivisionResponse(division model.Division) topicListDivisionResponse {
	return topicListDivisionResponse{
		UUID:      division.UUID,
		Code:      division.Code,
		Name:      division.Name,
		ShortName: division.ShortName,
		SortOrder: division.SortOrder,
	}
}

func newDivisionTopicsResponse(result topic.DivisionTopics) divisionTopicsResponse {
	topics := make([]topicListItemResponse, 0, len(result.Topics))
	for _, item := range result.Topics {
		topics = append(topics, topicListItemResponse{
			UUID:           item.UUID,
			Semester:       item.Semester,
			Title:          item.Title,
			DueDate:        item.DueDate.Format("2006-01-02"),
			IsOpen:         item.IsOpen,
			IsSecret:       item.IsSecret,
			CreatedByUUID:  item.CreatedByUUID,
			SubmittedCount: item.SubmittedCount,
			AssigneeCount:  item.AssigneeCount,
			IsAssigned:     item.IsAssigned,
			MyStatus:       item.MyStatus,
			MySubmittedAt:  item.MySubmittedAt,
			ClosedAt:       item.ClosedAt,
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		})
	}
	return divisionTopicsResponse{
		Division: newTopicListDivisionResponse(result.Division),
		AcademicYear: topicListAcademicYearResponse{
			UUID:      result.AcademicYear.UUID,
			Year:      result.AcademicYear.Year,
			IsCurrent: result.AcademicYear.IsCurrent,
		},
		Topics: topics,
	}
}
