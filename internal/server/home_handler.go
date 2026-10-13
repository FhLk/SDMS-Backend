package server

import (
	"context"
	"errors"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/home"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type homeService interface {
	Get(context.Context, auth.Profile) (home.Dashboard, error)
	DivisionStats(context.Context, string, auth.Profile) (home.DivisionStatistics, error)
}

type homeHandler struct {
	service homeService
}

type homeCountsResponse struct {
	TopicCount     int64 `json:"topic_count"`
	RequiredCount  int64 `json:"required_count"`
	SubmittedCount int64 `json:"submitted_count"`
	PendingCount   int64 `json:"pending_count"`
	OverdueCount   int64 `json:"overdue_count"`
}

type homeDivisionResponse struct {
	UUID      uuid.UUID `json:"uuid"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	ShortName string    `json:"short_name"`
	SortOrder int16     `json:"sort_order"`
}

type homePendingTopicResponse struct {
	UUID         uuid.UUID            `json:"uuid"`
	Title        string               `json:"title"`
	Division     homeDivisionResponse `json:"division"`
	AcademicYear int16                `json:"academic_year"`
	Semester     int16                `json:"semester"`
	DueDate      string               `json:"due_date"`
	IsSecret     bool                 `json:"is_secret"`
	Status       string               `json:"status"`
}

type homeRecentSubmissionResponse struct {
	UUID         uuid.UUID            `json:"uuid"`
	TopicUUID    uuid.UUID            `json:"topic_uuid"`
	Title        string               `json:"title"`
	Division     homeDivisionResponse `json:"division"`
	AcademicYear int16                `json:"academic_year"`
	Semester     int16                `json:"semester"`
	DueDate      string               `json:"due_date"`
	Status       string               `json:"status"`
	SubmittedAt  time.Time            `json:"submitted_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

type homeDivisionSummaryResponse struct {
	Division homeDivisionResponse `json:"division"`
	Counts   homeCountsResponse   `json:"counts"`
}

type homePersonalResponse struct {
	Counts            homeCountsResponse             `json:"counts"`
	PendingTopics     []homePendingTopicResponse     `json:"pending_topics"`
	RecentSubmissions []homeRecentSubmissionResponse `json:"recent_submissions"`
	Divisions         []homeDivisionSummaryResponse  `json:"divisions"`
}

type homeOverviewResponse struct {
	School    homeCountsResponse            `json:"school"`
	Divisions []homeDivisionSummaryResponse `json:"divisions"`
}

type homeResponse struct {
	My       homePersonalResponse  `json:"my"`
	Overview *homeOverviewResponse `json:"overview"`
}

type divisionStatisticsResponse struct {
	Division homeDivisionResponse `json:"division"`
	Stats    homeCountsResponse   `json:"stats"`
}

// get รวมข้อมูลที่หน้าแรกต้องใช้ไว้ใน request เดียวเพื่อลดการเรียก API ซ้ำจาก frontend.
func (h homeHandler) get(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Home service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	dashboard, err := h.service.Get(c.Context(), profile)
	if err != nil {
		return mapHomeError(err)
	}
	return c.JSON(newHomeResponse(dashboard))
}

// divisionStats ส่งสถิติฝ่ายหลัง service ตรวจสิทธิ์ผู้บริหารหรือหัวหน้าฝ่ายนั้นแล้ว.
func (h homeHandler) divisionStats(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Home service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	statistics, err := h.service.DivisionStats(c.Context(), c.Params("code"), profile)
	if err != nil {
		return mapHomeError(err)
	}
	return c.JSON(divisionStatisticsResponse{
		Division: newHomeDivisionResponse(statistics.Division),
		Stats:    newHomeCountsResponse(statistics.Counts),
	})
}

func mapHomeError(err error) error {
	switch {
	case errors.Is(err, home.ErrInvalidProfile):
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	case errors.Is(err, home.ErrDivisionNotFound):
		return apperror.New(fiber.StatusNotFound, "DIVISION_NOT_FOUND", "Division not found")
	case errors.Is(err, home.ErrForbidden):
		return apperror.New(fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to view these statistics")
	default:
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
}

func newHomeResponse(dashboard home.Dashboard) homeResponse {
	response := homeResponse{My: newHomePersonalResponse(dashboard.Personal)}
	if dashboard.Overview != nil {
		response.Overview = &homeOverviewResponse{
			School:    newHomeCountsResponse(dashboard.Overview.School),
			Divisions: newHomeDivisionSummaryResponses(dashboard.Overview.Divisions),
		}
	}
	return response
}

func newHomePersonalResponse(personal home.Personal) homePersonalResponse {
	pending := make([]homePendingTopicResponse, 0, len(personal.PendingTopics))
	for _, topic := range personal.PendingTopics {
		pending = append(pending, homePendingTopicResponse{
			UUID:         topic.UUID,
			Title:        topic.Title,
			Division:     newHomeDivisionResponse(topic.Division),
			AcademicYear: topic.AcademicYear,
			Semester:     topic.Semester,
			DueDate:      topic.DueDate.Format("2006-01-02"),
			IsSecret:     topic.IsSecret,
			Status:       topic.Status,
		})
	}
	recent := make([]homeRecentSubmissionResponse, 0, len(personal.RecentSubmissions))
	for _, submission := range personal.RecentSubmissions {
		recent = append(recent, homeRecentSubmissionResponse{
			UUID:         submission.UUID,
			TopicUUID:    submission.TopicUUID,
			Title:        submission.Title,
			Division:     newHomeDivisionResponse(submission.Division),
			AcademicYear: submission.AcademicYear,
			Semester:     submission.Semester,
			DueDate:      submission.DueDate.Format("2006-01-02"),
			Status:       submission.Status,
			SubmittedAt:  submission.SubmittedAt,
			UpdatedAt:    submission.UpdatedAt,
		})
	}
	return homePersonalResponse{
		Counts:            newHomeCountsResponse(personal.Counts),
		PendingTopics:     pending,
		RecentSubmissions: recent,
		Divisions:         newHomeDivisionSummaryResponses(personal.Divisions),
	}
}

func newHomeDivisionSummaryResponses(summaries []home.DivisionSummary) []homeDivisionSummaryResponse {
	responses := make([]homeDivisionSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		responses = append(responses, homeDivisionSummaryResponse{
			Division: newHomeDivisionResponse(summary.Division),
			Counts:   newHomeCountsResponse(summary.Counts),
		})
	}
	return responses
}

func newHomeCountsResponse(counts home.Counts) homeCountsResponse {
	return homeCountsResponse{
		TopicCount:     counts.TopicCount,
		RequiredCount:  counts.RequiredCount,
		SubmittedCount: counts.SubmittedCount,
		PendingCount:   counts.PendingCount,
		OverdueCount:   counts.OverdueCount,
	}
}

func newHomeDivisionResponse(division model.Division) homeDivisionResponse {
	return homeDivisionResponse{
		UUID:      division.UUID,
		Code:      division.Code,
		Name:      division.Name,
		ShortName: division.ShortName,
		SortOrder: division.SortOrder,
	}
}
