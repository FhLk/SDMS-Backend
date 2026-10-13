// Package portfolio จัดการข้อมูลย้อนหลังและไฟล์รวมในหน้า "แฟ้มของฉัน".
package portfolio

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const maxSearchRunes = 200

const (
	StatusSubmitted = "submitted"
	StatusLate      = "late"
)

var (
	ErrInvalidUser        = errors.New("portfolio user is invalid")
	ErrStorageUnavailable = errors.New("portfolio object storage is unavailable")
	bangkokLocation       = time.FixedZone("Asia/Bangkok", 7*60*60)
)

// ValidationError ระบุ query parameter ที่ไม่ผ่านกติกาเพื่อให้ HTTP layer ตอบ 400 ได้ตรงจุด.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type ListInput struct {
	UserUUID uuid.UUID
	Year     *int16
	Query    string
}

type ArchiveInput struct {
	UserUUID uuid.UUID
	Year     *int16
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

type Submission struct {
	UUID        uuid.UUID
	TopicUUID   uuid.UUID
	TopicTitle  string
	Division    model.Division
	Semester    int16
	DueDate     time.Time
	IsOpen      bool
	IsSecret    bool
	Values      json.RawMessage
	Status      string
	SubmittedAt time.Time
	UpdatedAt   time.Time
	Files       []File
}

type YearGroup struct {
	AcademicYear int16
	Submissions  []Submission
}

type Portfolio struct {
	Years []YearGroup
}

type Archive struct {
	Filename string
	Content  io.ReadCloser
}

// objectReader เป็น interface แคบที่ทำให้ service ใช้ได้ทั้ง local storage และ Cloudflare R2
// โดยไม่ต้องรับความสามารถอัปโหลดหรือลบไฟล์ซึ่งไม่เกี่ยวกับ use case นี้.
type objectReader interface {
	Open(context.Context, string) (io.ReadCloser, error)
}

type Service struct {
	db    *gorm.DB
	store objectReader
}

func NewService(db *gorm.DB, store objectReader) *Service {
	return &Service{db: db, store: store}
}

type submissionRow struct {
	SubmissionUUID uuid.UUID
	TopicUUID      uuid.UUID
	Values         datatypes.JSON
	SubmittedAt    time.Time
	UpdatedAt      time.Time
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
}

type submissionFileRow struct {
	SubmissionUUID uuid.UUID
	FileUUID       uuid.UUID
	FieldUUID      uuid.UUID
	OriginalName   string
	MimeType       string
	SizeBytes      int64
	Position       int16
	CreatedAt      time.Time
}

type archiveFileRow struct {
	FileUUID     uuid.UUID
	ObjectKey    string
	OriginalName string
	CreatedAt    time.Time
	AcademicYear int16
	DivisionCode string
	TopicTitle   string
}

// List คืน submission ของผู้ใช้เองทั้งหมด รวมทั้งหัวข้อเก่าหรือหัวข้อที่ปิดรับแล้ว
// และอ่านข้อมูลกับรายการไฟล์จาก database snapshot เดียวกัน.
func (s *Service) List(ctx context.Context, input ListInput) (Portfolio, error) {
	query, err := validateListInput(input)
	if err != nil {
		return Portfolio{}, err
	}

	portfolio := Portfolio{Years: make([]YearGroup, 0)}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := listSubmissionRows(tx, input.UserUUID, input.Year, query)
		if err != nil {
			return err
		}
		files, err := listSubmissionFiles(tx, submissionUUIDs(rows))
		if err != nil {
			return err
		}
		portfolio = buildPortfolio(rows, files)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Portfolio{}, err
	}
	return portfolio, nil
}

// OpenArchive อ่านเฉพาะ metadata ที่เป็นของผู้ใช้ก่อน แล้วคืน pipe ซึ่งสร้าง ZIP ใน goroutine.
// ตัว writer เปิด object จาก storage ครั้งละหนึ่งไฟล์ จึงไม่ใช้หน่วยความจำตามขนาดไฟล์รวม.
func (s *Service) OpenArchive(ctx context.Context, input ArchiveInput) (Archive, error) {
	if err := validateArchiveInput(input); err != nil {
		return Archive{}, err
	}
	if s.store == nil {
		return Archive{}, ErrStorageUnavailable
	}

	files, err := listArchiveFiles(s.db.WithContext(ctx), input.UserUUID, input.Year)
	if err != nil {
		return Archive{}, err
	}

	filename := "sdms-files-all.zip"
	if input.Year != nil {
		filename = fmt.Sprintf("sdms-files-%d.zip", *input.Year)
	}
	return Archive{
		Filename: filename,
		Content:  newArchiveStream(s.store, files),
	}, nil
}

func validateListInput(input ListInput) (string, error) {
	if input.UserUUID == uuid.Nil {
		return "", ErrInvalidUser
	}
	if err := validateYear(input.Year); err != nil {
		return "", err
	}
	query := strings.TrimSpace(input.Query)
	if utf8.RuneCountInString(query) > maxSearchRunes {
		return "", &ValidationError{Field: "q", Message: "search query must not exceed 200 characters"}
	}
	return query, nil
}

func validateArchiveInput(input ArchiveInput) error {
	if input.UserUUID == uuid.Nil {
		return ErrInvalidUser
	}
	return validateYear(input.Year)
}

func validateYear(year *int16) error {
	if year != nil && (*year < 2400 || *year > 3000) {
		return &ValidationError{Field: "year", Message: "academic year must be between 2400 and 3000"}
	}
	return nil
}

func listSubmissionRows(db *gorm.DB, userUUID uuid.UUID, year *int16, search string) ([]submissionRow, error) {
	rows := make([]submissionRow, 0)
	query := db.Table("submissions").
		Select(`
			submissions.uuid AS submission_uuid,
			submissions.topic_uuid,
			submissions.values,
			submissions.submitted_at,
			submissions.updated_at,
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
			academic_years.year AS academic_year`).
		Joins("JOIN topics ON topics.uuid = submissions.topic_uuid").
		Joins("JOIN divisions ON divisions.uuid = topics.division_uuid").
		Joins("JOIN academic_years ON academic_years.uuid = topics.academic_year_uuid").
		Where("submissions.user_uuid = ?", userUUID)
	if year != nil {
		query = query.Where("academic_years.year = ?", *year)
	}
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query = query.Where(`(
			topics.title ILIKE ? ESCAPE E'\\' OR
			submissions.values::text ILIKE ? ESCAPE E'\\' OR
			EXISTS (
				SELECT 1 FROM files
				WHERE files.submission_uuid = submissions.uuid
					AND files.status = ?
					AND files.original_name ILIKE ? ESCAPE E'\\'
			)
		)`, pattern, pattern, model.FileAttached, pattern)
	}
	err := query.
		Order("academic_years.year DESC, submissions.submitted_at DESC, submissions.uuid DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list portfolio submissions: %w", err)
	}
	return rows, nil
}

func listSubmissionFiles(db *gorm.DB, submissionUUIDs []uuid.UUID) ([]submissionFileRow, error) {
	rows := make([]submissionFileRow, 0)
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
		Order("files.submission_uuid, files.position, files.created_at, files.uuid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list portfolio submission files: %w", err)
	}
	return rows, nil
}

func listArchiveFiles(db *gorm.DB, userUUID uuid.UUID, year *int16) ([]archiveFileRow, error) {
	rows := make([]archiveFileRow, 0)
	query := db.Table("files").
		Select(`
			files.uuid AS file_uuid,
			files.object_key,
			files.original_name,
			files.created_at,
			academic_years.year AS academic_year,
			divisions.code AS division_code,
			topics.title AS topic_title`).
		Joins("JOIN submissions ON submissions.uuid = files.submission_uuid").
		Joins("JOIN topics ON topics.uuid = submissions.topic_uuid AND topics.uuid = files.topic_uuid").
		Joins("JOIN divisions ON divisions.uuid = topics.division_uuid").
		Joins("JOIN academic_years ON academic_years.uuid = topics.academic_year_uuid").
		Where("submissions.user_uuid = ? AND files.status = ?", userUUID, model.FileAttached)
	if year != nil {
		query = query.Where("academic_years.year = ?", *year)
	}
	err := query.
		Order("academic_years.year DESC, divisions.sort_order, topics.title, submissions.submitted_at DESC, files.position, files.created_at, files.uuid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list portfolio archive files: %w", err)
	}
	return rows, nil
}

func submissionUUIDs(rows []submissionRow) []uuid.UUID {
	values := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.SubmissionUUID)
	}
	return values
}

func buildPortfolio(rows []submissionRow, fileRows []submissionFileRow) Portfolio {
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

	portfolio := Portfolio{Years: make([]YearGroup, 0)}
	for _, row := range rows {
		if len(portfolio.Years) == 0 || portfolio.Years[len(portfolio.Years)-1].AcademicYear != row.AcademicYear {
			portfolio.Years = append(portfolio.Years, YearGroup{
				AcademicYear: row.AcademicYear,
				Submissions:  make([]Submission, 0),
			})
		}
		files := filesBySubmission[row.SubmissionUUID]
		if files == nil {
			files = make([]File, 0)
		}
		portfolio.Years[len(portfolio.Years)-1].Submissions = append(
			portfolio.Years[len(portfolio.Years)-1].Submissions,
			Submission{
				UUID:       row.SubmissionUUID,
				TopicUUID:  row.TopicUUID,
				TopicTitle: row.TopicTitle,
				Division: model.Division{
					UUIDModel: model.UUIDModel{UUID: row.DivisionUUID},
					Code:      row.DivisionCode,
					Name:      row.DivisionName,
					ShortName: row.DivisionShort,
					SortOrder: row.DivisionOrder,
				},
				Semester:    row.Semester,
				DueDate:     row.DueDate,
				IsOpen:      row.IsOpen,
				IsSecret:    row.IsSecret,
				Values:      append(json.RawMessage(nil), row.Values...),
				Status:      submittedStatus(row.SubmittedAt, row.DueDate),
				SubmittedAt: row.SubmittedAt,
				UpdatedAt:   row.UpdatedAt,
				Files:       files,
			},
		)
	}
	return portfolio
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

// archiveReadCloser ยกเลิก context ของตัวสร้าง ZIP เมื่อ client อ่านจบหรือยกเลิกดาวน์โหลด.
type archiveReadCloser struct {
	*io.PipeReader
	cancel context.CancelFunc
	once   sync.Once
}

func (r *archiveReadCloser) Close() error {
	r.once.Do(r.cancel)
	return r.PipeReader.Close()
}

func newArchiveStream(store objectReader, files []archiveFileRow) io.ReadCloser {
	reader, writer := io.Pipe()
	streamContext, cancel := context.WithCancel(context.Background())
	stream := &archiveReadCloser{PipeReader: reader, cancel: cancel}

	go func() {
		zipWriter := zip.NewWriter(writer)
		err := writeArchive(streamContext, zipWriter, store, files)
		if closeErr := zipWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		cancel()
	}()
	return stream
}

func writeArchive(ctx context.Context, writer *zip.Writer, store objectReader, files []archiveFileRow) error {
	buffer := make([]byte, 32*1024)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		content, err := store.Open(ctx, file.ObjectKey)
		if err != nil {
			return fmt.Errorf("open archive file %s: %w", file.FileUUID, err)
		}

		header := &zip.FileHeader{
			Name:   archiveEntryName(file),
			Method: zip.Deflate,
		}
		header.SetMode(0o640)
		if !file.CreatedAt.IsZero() {
			header.SetModTime(file.CreatedAt)
		}
		entry, createErr := writer.CreateHeader(header)
		if createErr != nil {
			_ = content.Close()
			return fmt.Errorf("create ZIP entry for file %s: %w", file.FileUUID, createErr)
		}
		_, copyErr := io.CopyBuffer(entry, content, buffer)
		closeErr := content.Close()
		if copyErr != nil {
			return fmt.Errorf("stream archive file %s: %w", file.FileUUID, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close archive file %s: %w", file.FileUUID, closeErr)
		}
	}
	return nil
}

func archiveEntryName(file archiveFileRow) string {
	filename := file.FileUUID.String() + "_" + safePathSegment(file.OriginalName, "file")
	return strings.Join([]string{
		fmt.Sprintf("%d", file.AcademicYear),
		safePathSegment(file.DivisionCode, "division"),
		safePathSegment(file.TopicTitle, "topic"),
		filename,
	}, "/")
}

// safePathSegment ตัด path separator, control character และชื่อพิเศษออก
// เพื่อไม่ให้ชื่อไฟล์จากผู้ใช้ออกจากโฟลเดอร์ที่กำหนดเมื่อนำ ZIP ไปแตกไฟล์.
func safePathSegment(value, fallback string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(character rune) rune {
		if character < 32 || character == 127 || strings.ContainsRune(`<>:"/\|?*`, character) {
			return '_'
		}
		return character
	}, value)
	value = strings.Trim(value, ". ")
	if value == "" {
		return fallback
	}
	return value
}
