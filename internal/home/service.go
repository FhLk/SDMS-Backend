// Package home รวมข้อมูลหน้าแรกและสถิติจาก snapshot ผู้ต้องส่งใน topic_assignees.
package home

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/authorization"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StatusSubmitted = "submitted"
	StatusLate      = "late"
	StatusPending   = "pending"
	StatusOverdue   = "overdue"
)

var (
	ErrInvalidProfile   = errors.New("home profile is invalid")
	ErrDivisionNotFound = errors.New("home division not found")
	ErrForbidden        = errors.New("home statistics access is forbidden")
)

var bangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

type Counts struct {
	TopicCount     int64
	RequiredCount  int64
	SubmittedCount int64
	PendingCount   int64
	OverdueCount   int64
}

type PendingTopic struct {
	UUID         uuid.UUID
	Title        string
	Division     model.Division
	AcademicYear int16
	Semester     int16
	DueDate      time.Time
	IsSecret     bool
	Status       string
}

type RecentSubmission struct {
	UUID         uuid.UUID
	TopicUUID    uuid.UUID
	Title        string
	Division     model.Division
	AcademicYear int16
	Semester     int16
	DueDate      time.Time
	Status       string
	SubmittedAt  time.Time
	UpdatedAt    time.Time
}

type DivisionSummary struct {
	Division model.Division
	Counts   Counts
}

type Personal struct {
	Counts            Counts
	PendingTopics     []PendingTopic
	RecentSubmissions []RecentSubmission
	Divisions         []DivisionSummary
}

type Overview struct {
	School    Counts
	Divisions []DivisionSummary
}

type Dashboard struct {
	Personal Personal
	Overview *Overview
}

type DivisionStatistics struct {
	Division model.Division
	Counts   Counts
}

type Service struct {
	db  *gorm.DB
	now func() time.Time
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now}
}

type assignmentRow struct {
	TopicUUID        uuid.UUID
	Title            string
	DueDate          time.Time
	IsSecret         bool
	Semester         int16
	DivisionUUID     uuid.UUID
	DivisionCode     string
	DivisionName     string
	DivisionShort    string
	DivisionOrder    int16
	AcademicYear     int16
	SubmissionUUID   *uuid.UUID
	SubmittedAt      *time.Time
	SubmissionUpdate *time.Time
}

type aggregateRow struct {
	DivisionUUID   uuid.UUID
	TopicCount     int64
	RequiredCount  int64
	SubmittedCount int64
	PendingCount   int64
	OverdueCount   int64
}

// Get อ่านข้อมูลทุกส่วนใน read-only repeatable-read transaction เพื่อให้ตัวเลข
// รายการค้าง และรายการส่งล่าสุดอ้างอิง snapshot ฐานข้อมูลชุดเดียวกัน.
func (s *Service) Get(ctx context.Context, profile auth.Profile) (Dashboard, error) {
	if profile.User.UUID == uuid.Nil {
		return Dashboard{}, ErrInvalidProfile
	}

	now := s.currentTime()
	var dashboard Dashboard
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		divisions, err := listDivisions(tx)
		if err != nil {
			return err
		}
		assignments, err := listAssignments(tx, profile.User.UUID)
		if err != nil {
			return err
		}
		dashboard.Personal = buildPersonal(assignments, divisions, now)

		// หน้าแรกเปิดเผยภาพรวมแบบ aggregate ให้หัวหน้าฝ่ายและผู้บริหารตาม requirement
		// แต่ endpoint เจาะฝ่ายยังคงตรวจ CanViewDivisionStats แยกต่างหาก.
		if profile.User.IsExec || len(profile.HeadDivisions) > 0 {
			summaries, err := listDivisionSummaries(tx, divisions, now)
			if err != nil {
				return err
			}
			dashboard.Overview = &Overview{
				School:    sumDivisionCounts(summaries),
				Divisions: summaries,
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Dashboard{}, err
	}
	return dashboard, nil
}

// DivisionStats คืนสถิติของฝ่ายเดียวและใช้ policy กลางป้องกันหัวหน้าฝ่ายอื่น
// รวมถึงแอดมินที่ไม่มีบทบาทด้านเนื้อหาจากการดูข้อมูล aggregate ของฝ่ายนั้น.
func (s *Service) DivisionStats(ctx context.Context, divisionCode string, profile auth.Profile) (DivisionStatistics, error) {
	if profile.User.UUID == uuid.Nil {
		return DivisionStatistics{}, ErrInvalidProfile
	}
	divisionCode = strings.ToLower(strings.TrimSpace(divisionCode))
	if divisionCode == "" {
		return DivisionStatistics{}, ErrDivisionNotFound
	}

	var result DivisionStatistics
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var division model.Division
		query := tx.Where("code = ?", divisionCode).Limit(1).Find(&division)
		if query.Error != nil {
			return fmt.Errorf("find home statistics division: %w", query.Error)
		}
		if query.RowsAffected == 0 {
			return ErrDivisionNotFound
		}
		if !authorization.CanViewDivisionStats(profile, division.UUID) {
			return ErrForbidden
		}

		counts, err := aggregateDivision(tx, division.UUID, s.currentTime())
		if err != nil {
			return err
		}
		result = DivisionStatistics{Division: division, Counts: counts}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return DivisionStatistics{}, err
	}
	return result, nil
}

func listDivisions(db *gorm.DB) ([]model.Division, error) {
	divisions := make([]model.Division, 0)
	if err := db.Order("sort_order, code").Find(&divisions).Error; err != nil {
		return nil, fmt.Errorf("list home divisions: %w", err)
	}
	return divisions, nil
}

func listAssignments(db *gorm.DB, userUUID uuid.UUID) ([]assignmentRow, error) {
	rows := make([]assignmentRow, 0)
	err := db.Table("topic_assignees AS assigned").
		Select(`
			topics.uuid AS topic_uuid,
			topics.title,
			topics.due_date,
			topics.is_secret,
			topics.semester,
			divisions.uuid AS division_uuid,
			divisions.code AS division_code,
			divisions.name AS division_name,
			divisions.short_name AS division_short,
			divisions.sort_order AS division_order,
			academic_years.year AS academic_year,
			submissions.uuid AS submission_uuid,
			submissions.submitted_at,
			submissions.updated_at AS submission_update`).
		Joins("JOIN topics ON topics.uuid = assigned.topic_uuid AND topics.is_open = ?", true).
		Joins("JOIN divisions ON divisions.uuid = topics.division_uuid").
		Joins("JOIN academic_years ON academic_years.uuid = topics.academic_year_uuid").
		Joins("LEFT JOIN submissions ON submissions.topic_uuid = assigned.topic_uuid AND submissions.user_uuid = assigned.user_uuid").
		Where("assigned.user_uuid = ?", userUUID).
		Order("topics.due_date, topics.created_at, topics.uuid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list home assignments: %w", err)
	}
	return rows, nil
}

func buildPersonal(rows []assignmentRow, divisions []model.Division, now time.Time) Personal {
	personal := Personal{
		PendingTopics:     make([]PendingTopic, 0),
		RecentSubmissions: make([]RecentSubmission, 0),
		Divisions:         zeroDivisionSummaries(divisions),
	}
	divisionIndexes := make(map[uuid.UUID]int, len(personal.Divisions))
	for index, summary := range personal.Divisions {
		divisionIndexes[summary.Division.UUID] = index
	}

	for _, row := range rows {
		personal.Counts.TopicCount++
		personal.Counts.RequiredCount++
		divisionIndex, exists := divisionIndexes[row.DivisionUUID]
		if exists {
			personal.Divisions[divisionIndex].Counts.TopicCount++
			personal.Divisions[divisionIndex].Counts.RequiredCount++
		}

		division := divisionFromAssignment(row)
		if row.SubmissionUUID != nil && row.SubmittedAt != nil {
			status := submittedStatus(*row.SubmittedAt, row.DueDate)
			personal.Counts.SubmittedCount++
			if exists {
				personal.Divisions[divisionIndex].Counts.SubmittedCount++
			}
			updatedAt := *row.SubmittedAt
			if row.SubmissionUpdate != nil {
				updatedAt = *row.SubmissionUpdate
			}
			personal.RecentSubmissions = append(personal.RecentSubmissions, RecentSubmission{
				UUID:         *row.SubmissionUUID,
				TopicUUID:    row.TopicUUID,
				Title:        row.Title,
				Division:     division,
				AcademicYear: row.AcademicYear,
				Semester:     row.Semester,
				DueDate:      row.DueDate,
				Status:       status,
				SubmittedAt:  *row.SubmittedAt,
				UpdatedAt:    updatedAt,
			})
			continue
		}

		status := StatusPending
		if afterDueDate(now, row.DueDate) {
			status = StatusOverdue
			personal.Counts.OverdueCount++
			if exists {
				personal.Divisions[divisionIndex].Counts.OverdueCount++
			}
		} else {
			personal.Counts.PendingCount++
			if exists {
				personal.Divisions[divisionIndex].Counts.PendingCount++
			}
		}
		personal.PendingTopics = append(personal.PendingTopics, PendingTopic{
			UUID:         row.TopicUUID,
			Title:        row.Title,
			Division:     division,
			AcademicYear: row.AcademicYear,
			Semester:     row.Semester,
			DueDate:      row.DueDate,
			IsSecret:     row.IsSecret,
			Status:       status,
		})
	}

	sort.SliceStable(personal.RecentSubmissions, func(left, right int) bool {
		if !personal.RecentSubmissions[left].SubmittedAt.Equal(personal.RecentSubmissions[right].SubmittedAt) {
			return personal.RecentSubmissions[left].SubmittedAt.After(personal.RecentSubmissions[right].SubmittedAt)
		}
		return personal.RecentSubmissions[left].UUID.String() > personal.RecentSubmissions[right].UUID.String()
	})
	if len(personal.RecentSubmissions) > 5 {
		personal.RecentSubmissions = personal.RecentSubmissions[:5]
	}
	return personal
}

func listDivisionSummaries(db *gorm.DB, divisions []model.Division, now time.Time) ([]DivisionSummary, error) {
	rows := make([]aggregateRow, 0)
	today := now.In(bangkokLocation).Format("2006-01-02")
	err := db.Table("divisions").
		Select(`
			divisions.uuid AS division_uuid,
			COUNT(DISTINCT topics.uuid) FILTER (WHERE topics.uuid IS NOT NULL) AS topic_count,
			COUNT(assigned.uuid) AS required_count,
			COUNT(submissions.uuid) AS submitted_count,
			COUNT(assigned.uuid) FILTER (WHERE submissions.uuid IS NULL AND topics.due_date >= ?) AS pending_count,
			COUNT(assigned.uuid) FILTER (WHERE submissions.uuid IS NULL AND topics.due_date < ?) AS overdue_count`, today, today).
		Joins("LEFT JOIN topics ON topics.division_uuid = divisions.uuid AND topics.is_open = ?", true).
		Joins("LEFT JOIN topic_assignees AS assigned ON assigned.topic_uuid = topics.uuid").
		Joins("LEFT JOIN submissions ON submissions.topic_uuid = assigned.topic_uuid AND submissions.user_uuid = assigned.user_uuid").
		Group("divisions.uuid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate home division statistics: %w", err)
	}

	byDivision := make(map[uuid.UUID]Counts, len(rows))
	for _, row := range rows {
		byDivision[row.DivisionUUID] = countsFromAggregate(row)
	}
	summaries := zeroDivisionSummaries(divisions)
	for index := range summaries {
		summaries[index].Counts = byDivision[summaries[index].Division.UUID]
	}
	return summaries, nil
}

func aggregateDivision(db *gorm.DB, divisionUUID uuid.UUID, now time.Time) (Counts, error) {
	today := now.In(bangkokLocation).Format("2006-01-02")
	var row aggregateRow
	err := db.Table("topics").
		Select(`
			?::uuid AS division_uuid,
			COUNT(DISTINCT topics.uuid) AS topic_count,
			COUNT(assigned.uuid) AS required_count,
			COUNT(submissions.uuid) AS submitted_count,
			COUNT(assigned.uuid) FILTER (WHERE submissions.uuid IS NULL AND topics.due_date >= ?) AS pending_count,
			COUNT(assigned.uuid) FILTER (WHERE submissions.uuid IS NULL AND topics.due_date < ?) AS overdue_count`, divisionUUID, today, today).
		Joins("LEFT JOIN topic_assignees AS assigned ON assigned.topic_uuid = topics.uuid").
		Joins("LEFT JOIN submissions ON submissions.topic_uuid = assigned.topic_uuid AND submissions.user_uuid = assigned.user_uuid").
		Where("topics.division_uuid = ? AND topics.is_open = ?", divisionUUID, true).
		Scan(&row).Error
	if err != nil {
		return Counts{}, fmt.Errorf("aggregate one division statistics: %w", err)
	}
	return countsFromAggregate(row), nil
}

func zeroDivisionSummaries(divisions []model.Division) []DivisionSummary {
	summaries := make([]DivisionSummary, 0, len(divisions))
	for _, division := range divisions {
		summaries = append(summaries, DivisionSummary{Division: division})
	}
	return summaries
}

func sumDivisionCounts(divisions []DivisionSummary) Counts {
	var total Counts
	for _, division := range divisions {
		total.TopicCount += division.Counts.TopicCount
		total.RequiredCount += division.Counts.RequiredCount
		total.SubmittedCount += division.Counts.SubmittedCount
		total.PendingCount += division.Counts.PendingCount
		total.OverdueCount += division.Counts.OverdueCount
	}
	return total
}

func countsFromAggregate(row aggregateRow) Counts {
	return Counts{
		TopicCount:     row.TopicCount,
		RequiredCount:  row.RequiredCount,
		SubmittedCount: row.SubmittedCount,
		PendingCount:   row.PendingCount,
		OverdueCount:   row.OverdueCount,
	}
}

func divisionFromAssignment(row assignmentRow) model.Division {
	return model.Division{
		UUIDModel: model.UUIDModel{UUID: row.DivisionUUID},
		Code:      row.DivisionCode,
		Name:      row.DivisionName,
		ShortName: row.DivisionShort,
		SortOrder: row.DivisionOrder,
	}
}

func submittedStatus(submittedAt, dueDate time.Time) string {
	if afterDueDate(submittedAt, dueDate) {
		return StatusLate
	}
	return StatusSubmitted
}

func afterDueDate(value, dueDate time.Time) bool {
	local := value.In(bangkokLocation)
	valueNumber := local.Year()*10000 + int(local.Month())*100 + local.Day()
	dueNumber := dueDate.Year()*10000 + int(dueDate.Month())*100 + dueDate.Day()
	return valueNumber > dueNumber
}

func (s *Service) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}
