package topic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StatusSubmitted    = "submitted"
	StatusLate         = "late"
	StatusPending      = "pending"
	StatusOverdue      = "overdue"
	StatusNotSubmitted = "not_submitted"
)

var ErrTopicAcademicYearNotFound = errors.New("topic academic year not found")

var topicBangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

// ListItem คือข้อมูลสรุปสำหรับหน้ารายการ ไม่โหลด fields และ assignees ทั้งชุด
// แต่คืนจำนวนการส่งกับสถานะของผู้ใช้ที่คำนวณจาก snapshot ใน topic_assignees.
type ListItem struct {
	UUID           uuid.UUID
	Semester       int16
	Title          string
	DueDate        time.Time
	IsOpen         bool
	IsSecret       bool
	CreatedByUUID  uuid.UUID
	SubmittedCount int64
	AssigneeCount  int64
	IsAssigned     bool
	MyStatus       *string
	MySubmittedAt  *time.Time
	ClosedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DivisionTopics struct {
	Division     model.Division
	AcademicYear model.AcademicYear
	Topics       []ListItem
}

type topicListRow struct {
	UUID           uuid.UUID
	Semester       int16
	Title          string
	DueDate        time.Time
	IsOpen         bool
	IsSecret       bool
	CreatedByUUID  uuid.UUID
	SubmittedCount int64
	AssigneeCount  int64
	IsAssigned     bool
	MySubmittedAt  *time.Time
	ClosedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ListDivisions คืนฝ่ายตาม sort_order เพื่อให้ทุกหน้าของระบบแสดงลำดับเดียวกัน.
func (s *Service) ListDivisions(ctx context.Context) ([]model.Division, error) {
	divisions := make([]model.Division, 0)
	if err := s.db.WithContext(ctx).Order("sort_order, code").Find(&divisions).Error; err != nil {
		return nil, fmt.Errorf("list divisions: %w", err)
	}
	return divisions, nil
}

// ListDivisionTopics ใช้ปีปัจจุบันเมื่อไม่ส่ง year และสรุป x/y ใน query เดียว.
// x นับเฉพาะ submission ของคนที่อยู่ใน snapshot เพื่อไม่ให้ข้อมูลผิดปกติเพิ่มยอด.
func (s *Service) ListDivisionTopics(ctx context.Context, divisionCode string, year *int16, userUUID uuid.UUID) (DivisionTopics, error) {
	divisionCode = strings.ToLower(strings.TrimSpace(divisionCode))
	if divisionCode == "" {
		return DivisionTopics{}, validationError("code", "division code is required")
	}
	if userUUID == uuid.Nil {
		return DivisionTopics{}, validationError("user_uuid", "user UUID is required")
	}
	if year != nil && (*year < 2400 || *year > 3000) {
		return DivisionTopics{}, validationError("year", "academic year must be between 2400 and 3000")
	}

	db := s.db.WithContext(ctx)
	division, err := findDivisionByCode(db, divisionCode)
	if err != nil {
		return DivisionTopics{}, err
	}
	academicYear, err := findListAcademicYear(db, year)
	if err != nil {
		return DivisionTopics{}, err
	}

	rows := make([]topicListRow, 0)
	if err := db.Table("topics").
		Select(`
			topics.uuid,
			topics.semester,
			topics.title,
			topics.due_date,
			topics.is_open,
			topics.is_secret,
			topics.created_by_uuid,
			topics.closed_at,
			topics.created_at,
			topics.updated_at,
			(SELECT COUNT(*) FROM topic_assignees AS ta
			 WHERE ta.topic_uuid = topics.uuid) AS assignee_count,
			(SELECT COUNT(*) FROM submissions AS submitted
			 INNER JOIN topic_assignees AS ta
			   ON ta.topic_uuid = submitted.topic_uuid AND ta.user_uuid = submitted.user_uuid
			 WHERE submitted.topic_uuid = topics.uuid) AS submitted_count,
			EXISTS (
			 SELECT 1 FROM topic_assignees AS mine
			 WHERE mine.topic_uuid = topics.uuid AND mine.user_uuid = ?
			) AS is_assigned,
			(SELECT mine_submission.submitted_at FROM submissions AS mine_submission
			 WHERE mine_submission.topic_uuid = topics.uuid AND mine_submission.user_uuid = ?
			 LIMIT 1) AS my_submitted_at`, userUUID, userUUID).
		Where("topics.division_uuid = ?", division.UUID).
		Where("topics.academic_year_uuid = ?", academicYear.UUID).
		Order("topics.is_open DESC").
		Order("topics.due_date ASC").
		Order("topics.created_at DESC").
		Order("topics.uuid ASC").
		Scan(&rows).Error; err != nil {
		return DivisionTopics{}, fmt.Errorf("list division topics: %w", err)
	}

	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	topics := make([]ListItem, 0, len(rows))
	for _, row := range rows {
		topics = append(topics, newListItem(row, now))
	}
	return DivisionTopics{Division: division, AcademicYear: academicYear, Topics: topics}, nil
}

func findDivisionByCode(db *gorm.DB, code string) (model.Division, error) {
	var division model.Division
	query := db.Where("code = ?", code).Limit(1).Find(&division)
	if query.Error != nil {
		return model.Division{}, fmt.Errorf("find division by code: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return model.Division{}, admin.ErrDivisionNotFound
	}
	return division, nil
}

func findListAcademicYear(db *gorm.DB, year *int16) (model.AcademicYear, error) {
	var academicYear model.AcademicYear
	query := db
	if year == nil {
		query = query.Where("is_current = ?", true)
	} else {
		query = query.Where("year = ?", *year)
	}
	result := query.Limit(1).Find(&academicYear)
	if result.Error != nil {
		return model.AcademicYear{}, fmt.Errorf("find topic academic year: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if year == nil {
			return model.AcademicYear{}, ErrCurrentAcademicYearNotSet
		}
		return model.AcademicYear{}, ErrTopicAcademicYearNotFound
	}
	return academicYear, nil
}

func newListItem(row topicListRow, now time.Time) ListItem {
	return ListItem{
		UUID:           row.UUID,
		Semester:       row.Semester,
		Title:          row.Title,
		DueDate:        row.DueDate,
		IsOpen:         row.IsOpen,
		IsSecret:       row.IsSecret,
		CreatedByUUID:  row.CreatedByUUID,
		SubmittedCount: row.SubmittedCount,
		AssigneeCount:  row.AssigneeCount,
		IsAssigned:     row.IsAssigned,
		MyStatus:       topicStatus(row.IsAssigned, row.MySubmittedAt, row.DueDate, row.IsOpen, now),
		MySubmittedAt:  row.MySubmittedAt,
		ClosedAt:       row.ClosedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

// topicStatus เปรียบเทียบเฉพาะวันที่ตามเวลาไทย: ส่งภายในวัน due_date ยังถือว่าทัน.
// ผู้ใช้ที่ไม่อยู่ใน snapshot ไม่มีหน้าที่ส่ง จึงคืน nil แทนการสร้างสถานะหลอก.
func topicStatus(assigned bool, submittedAt *time.Time, dueDate time.Time, open bool, now time.Time) *string {
	if !assigned {
		return nil
	}
	if submittedAt != nil {
		status := StatusSubmitted
		if dateNumber(submittedAt.In(topicBangkokLocation)) > dateNumber(dueDate) {
			status = StatusLate
		}
		return &status
	}
	if !open {
		status := StatusNotSubmitted
		return &status
	}
	status := StatusPending
	if dateNumber(now.In(topicBangkokLocation)) > dateNumber(dueDate) {
		status = StatusOverdue
	}
	return &status
}

func dateNumber(value time.Time) int {
	year, month, day := value.Date()
	return year*10000 + int(month)*100 + day
}
