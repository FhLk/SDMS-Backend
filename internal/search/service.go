// Package search ค้นคำตอบที่ผู้ใช้มีสิทธิ์เห็นจากหัวข้อ บุคลากร ค่าที่กรอก และไฟล์แนบ.
package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	maxSearchRunes  = 200
	maxPage         = 1_000_000

	StatusSubmitted = "submitted"
	StatusLate      = "late"
)

var (
	ErrInvalidProfile = errors.New("search profile is invalid")
	bangkokLocation   = time.FixedZone("Asia/Bangkok", 7*60*60)
)

// ValidationError ระบุ query parameter ที่ผิดเพื่อให้ HTTP layer คืนรายละเอียดได้ตรงช่อง.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type Input struct {
	Profile  auth.Profile
	Query    string
	Division string
	Year     *int16
	Semester *int16
	By       *uuid.UUID
	Page     int
	PageSize int
}

type File struct {
	UUID         uuid.UUID
	FieldUUID    uuid.UUID
	OriginalName string
	MimeType     string
	SizeBytes    int64
	Position     int16
	CreatedAt    time.Time
}

type Topic struct {
	UUID         uuid.UUID
	Title        string
	Division     model.Division
	AcademicYear int16
	Semester     int16
	DueDate      time.Time
	IsOpen       bool
	IsSecret     bool
}

type Item struct {
	SubmissionUUID uuid.UUID
	Topic          Topic
	SubmittedBy    model.User
	Values         json.RawMessage
	Files          []File
	Status         string
	SubmittedAt    time.Time
	UpdatedAt      time.Time
}

type Result struct {
	Items      []Item
	Page       int
	PageSize   int
	Total      int64
	TotalPages int64
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type itemRow struct {
	SubmissionUUID uuid.UUID
	Values         datatypes.JSON
	SubmittedAt    time.Time
	UpdatedAt      time.Time
	TopicUUID      uuid.UUID
	TopicTitle     string
	DueDate        time.Time
	Semester       int16
	IsOpen         bool
	IsSecret       bool
	DivisionUUID   uuid.UUID
	DivisionCode   string
	DivisionName   string
	DivisionShort  string
	DivisionOrder  int16
	AcademicYear   int16
	UserUUID       uuid.UUID
	Username       string
	Prefix         string
	FirstName      string
	LastName       string
	Position       string
}

type fileRow struct {
	SubmissionUUID uuid.UUID
	FileUUID       uuid.UUID
	FieldUUID      uuid.UUID
	OriginalName   string
	MimeType       string
	SizeBytes      int64
	Position       int16
	CreatedAt      time.Time
}

// Search นับจำนวนและอ่านหน้าผลลัพธ์ใน repeatable-read snapshot เดียวกัน
// เพื่อให้ total, items และไฟล์แนบสอดคล้องกันตลอดหนึ่ง request.
func (s *Service) Search(ctx context.Context, input Input) (Result, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Items:    make([]Item, 0),
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := searchableSubmissions(tx, normalized).Count(&result.Total).Error; err != nil {
			return fmt.Errorf("count search results: %w", err)
		}

		rows := make([]itemRow, 0)
		offset := (normalized.Page - 1) * normalized.PageSize
		err := searchableSubmissions(tx, normalized).
			Select(`
				submissions.uuid AS submission_uuid,
				submissions.values,
				submissions.submitted_at,
				submissions.updated_at,
				topics.uuid AS topic_uuid,
				topics.title AS topic_title,
				topics.due_date,
				topics.semester,
				topics.is_open,
				topics.is_secret,
				divisions.uuid AS division_uuid,
				divisions.code AS division_code,
				divisions.name AS division_name,
				divisions.short_name AS division_short,
				divisions.sort_order AS division_order,
				academic_years.year AS academic_year,
				users.uuid AS user_uuid,
				users.username,
				users.prefix,
				users.first_name,
				users.last_name,
				users.position`).
			Order("submissions.submitted_at DESC, submissions.uuid DESC").
			Limit(normalized.PageSize).
			Offset(offset).
			Scan(&rows).Error
		if err != nil {
			return fmt.Errorf("list search results: %w", err)
		}

		files, err := listResultFiles(tx, resultSubmissionUUIDs(rows))
		if err != nil {
			return err
		}
		result.Items = buildItems(rows, files)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Result{}, err
	}
	if result.Total > 0 {
		result.TotalPages = (result.Total + int64(result.PageSize) - 1) / int64(result.PageSize)
	}
	return result, nil
}

func normalizeInput(input Input) (Input, error) {
	if input.Profile.User.UUID == uuid.Nil {
		return Input{}, ErrInvalidProfile
	}
	input.Query = strings.TrimSpace(input.Query)
	if utf8.RuneCountInString(input.Query) > maxSearchRunes {
		return Input{}, &ValidationError{Field: "q", Message: "search query must not exceed 200 characters"}
	}

	input.Division = strings.ToLower(strings.TrimSpace(input.Division))
	if input.Division != "" {
		if len(input.Division) > 8 {
			return Input{}, &ValidationError{Field: "division", Message: "division code must not exceed 8 characters"}
		}
		for _, character := range input.Division {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-') {
				return Input{}, &ValidationError{Field: "division", Message: "division must be a valid code"}
			}
		}
	}
	if input.Year != nil && (*input.Year < 2400 || *input.Year > 3000) {
		return Input{}, &ValidationError{Field: "year", Message: "academic year must be between 2400 and 3000"}
	}
	if input.Semester != nil && *input.Semester != 1 && *input.Semester != 2 {
		return Input{}, &ValidationError{Field: "semester", Message: "semester must be 1 or 2"}
	}
	if input.By != nil && *input.By == uuid.Nil {
		return Input{}, &ValidationError{Field: "by", Message: "submitter UUID must be valid"}
	}
	if input.Page == 0 {
		input.Page = 1
	}
	if input.Page < 1 || input.Page > maxPage {
		return Input{}, &ValidationError{Field: "page", Message: "page must be between 1 and 1000000"}
	}
	if input.PageSize == 0 {
		input.PageSize = DefaultPageSize
	}
	if input.PageSize < 1 || input.PageSize > MaxPageSize {
		return Input{}, &ValidationError{Field: "page_size", Message: "page_size must be between 1 and 100"}
	}
	return input, nil
}

func searchableSubmissions(db *gorm.DB, input Input) *gorm.DB {
	query := db.Table("submissions").
		Joins("JOIN topics ON topics.uuid = submissions.topic_uuid").
		Joins("JOIN divisions ON divisions.uuid = topics.division_uuid").
		Joins("JOIN academic_years ON academic_years.uuid = topics.academic_year_uuid").
		Joins("JOIN users ON users.uuid = submissions.user_uuid")
	query = applyVisibility(query, input.Profile)

	if input.Query != "" {
		pattern := "%" + escapeLike(input.Query) + "%"
		query = query.Where(`(
			topics.title ILIKE ? ESCAPE E'\\' OR
			divisions.name ILIKE ? ESCAPE E'\\' OR
			divisions.short_name ILIKE ? ESCAPE E'\\' OR
			divisions.code ILIKE ? ESCAPE E'\\' OR
			users.prefix ILIKE ? ESCAPE E'\\' OR
			users.first_name ILIKE ? ESCAPE E'\\' OR
			users.last_name ILIKE ? ESCAPE E'\\' OR
			CONCAT_WS(' ', NULLIF(users.prefix, ''), users.first_name, NULLIF(users.last_name, '')) ILIKE ? ESCAPE E'\\' OR
			(users.prefix || users.first_name || CASE WHEN users.last_name = '' THEN '' ELSE ' ' || users.last_name END) ILIKE ? ESCAPE E'\\' OR
			users.username ILIKE ? ESCAPE E'\\' OR
			submissions.values::text ILIKE ? ESCAPE E'\\' OR
			EXISTS (
				SELECT 1 FROM files
				WHERE files.submission_uuid = submissions.uuid
					AND files.status = ?
					AND files.original_name ILIKE ? ESCAPE E'\\'
			)
		)`, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, model.FileAttached, pattern)
	}
	if input.Division != "" {
		query = query.Where("divisions.code = ?", input.Division)
	}
	if input.Year != nil {
		query = query.Where("academic_years.year = ?", *input.Year)
	}
	if input.Semester != nil {
		query = query.Where("topics.semester = ?", *input.Semester)
	}
	if input.By != nil {
		query = query.Where("submissions.user_uuid = ?", *input.By)
	}
	return query
}

// applyVisibility วางเงื่อนไขสิทธิ์ไว้ใน SQL ก่อน count และ pagination
// เพื่อไม่ให้ทั้งข้อมูลและจำนวนคำตอบของหัวข้อลับรั่วผ่าน metadata.
func applyVisibility(query *gorm.DB, profile auth.Profile) *gorm.DB {
	if profile.User.IsExec {
		return query
	}

	headDivisionUUIDs := make([]uuid.UUID, 0, len(profile.HeadDivisions))
	for _, division := range profile.HeadDivisions {
		if division.UUID != uuid.Nil {
			headDivisionUUIDs = append(headDivisionUUIDs, division.UUID)
		}
	}
	if len(headDivisionUUIDs) > 0 {
		return query.Where(`(
			topics.is_secret = FALSE OR
			submissions.user_uuid = ? OR
			topics.created_by_uuid = ? OR
			topics.division_uuid IN ?
		)`, profile.User.UUID, profile.User.UUID, headDivisionUUIDs)
	}
	return query.Where(`(
		topics.is_secret = FALSE OR
		submissions.user_uuid = ? OR
		topics.created_by_uuid = ?
	)`, profile.User.UUID, profile.User.UUID)
}

func listResultFiles(db *gorm.DB, submissionUUIDs []uuid.UUID) ([]fileRow, error) {
	rows := make([]fileRow, 0)
	if len(submissionUUIDs) == 0 {
		return rows, nil
	}
	err := db.Table("files").
		Select(`
			files.submission_uuid,
			files.uuid AS file_uuid,
			files.field_uuid,
			files.original_name,
			files.mime_type,
			files.size_bytes,
			files.position,
			files.created_at`).
		Where("files.submission_uuid IN ? AND files.status = ?", submissionUUIDs, model.FileAttached).
		Order("files.submission_uuid, files.field_uuid, files.position, files.created_at, files.uuid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list search result files: %w", err)
	}
	return rows, nil
}

func resultSubmissionUUIDs(rows []itemRow) []uuid.UUID {
	values := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.SubmissionUUID)
	}
	return values
}

func buildItems(rows []itemRow, fileRows []fileRow) []Item {
	filesBySubmission := make(map[uuid.UUID][]File)
	for _, row := range fileRows {
		filesBySubmission[row.SubmissionUUID] = append(filesBySubmission[row.SubmissionUUID], File{
			UUID:         row.FileUUID,
			FieldUUID:    row.FieldUUID,
			OriginalName: row.OriginalName,
			MimeType:     row.MimeType,
			SizeBytes:    row.SizeBytes,
			Position:     row.Position,
			CreatedAt:    row.CreatedAt,
		})
	}

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		files := filesBySubmission[row.SubmissionUUID]
		if files == nil {
			files = make([]File, 0)
		}
		items = append(items, Item{
			SubmissionUUID: row.SubmissionUUID,
			Topic: Topic{
				UUID:  row.TopicUUID,
				Title: row.TopicTitle,
				Division: model.Division{
					UUIDModel: model.UUIDModel{UUID: row.DivisionUUID},
					Code:      row.DivisionCode,
					Name:      row.DivisionName,
					ShortName: row.DivisionShort,
					SortOrder: row.DivisionOrder,
				},
				AcademicYear: row.AcademicYear,
				Semester:     row.Semester,
				DueDate:      row.DueDate,
				IsOpen:       row.IsOpen,
				IsSecret:     row.IsSecret,
			},
			SubmittedBy: model.User{
				UUIDModel: model.UUIDModel{UUID: row.UserUUID},
				Username:  row.Username,
				Prefix:    row.Prefix,
				FirstName: row.FirstName,
				LastName:  row.LastName,
				Position:  row.Position,
			},
			Values:      append(json.RawMessage(nil), row.Values...),
			Files:       files,
			Status:      submittedStatus(row.SubmittedAt, row.DueDate),
			SubmittedAt: row.SubmittedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return items
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func submittedStatus(submittedAt, dueDate time.Time) string {
	local := submittedAt.In(bangkokLocation)
	submittedNumber := local.Year()*10000 + int(local.Month())*100 + local.Day()
	dueNumber := dueDate.Year()*10000 + int(dueDate.Month())*100 + dueDate.Day()
	if submittedNumber > dueNumber {
		return StatusLate
	}
	return StatusSubmitted
}
