package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrExecutiveCannotHead     = errors.New("executive cannot be a division head")
	ErrInactiveUserCannotHead  = errors.New("inactive user cannot be a division head")
	ErrUserAlreadyDivisionHead = errors.New("user is already a division head")
	ErrAcademicYearExists      = errors.New("academic year already exists")
	ErrAcademicYearNotFound    = errors.New("academic year not found")
)

const academicYearSummarySelect = `
	academic_years.uuid,
	academic_years.year,
	academic_years.is_current,
	academic_years.current_semester,
	academic_years.created_at,
	(SELECT COUNT(*) FROM topics WHERE topics.academic_year_uuid = academic_years.uuid) AS topic_count`

// OrganizationService จัดการหัวหน้าฝ่ายและปีการศึกษา ซึ่งเป็นค่ากลาง
// ที่มีผลต่อสิทธิ์และหัวข้อใหม่ทั้งระบบ.
type OrganizationService struct {
	db *gorm.DB
}

func NewOrganizationService(db *gorm.DB) *OrganizationService {
	return &OrganizationService{db: db}
}

// AcademicYearSummary เพิ่มจำนวนหัวข้อให้ข้อมูลปีการศึกษา เพื่อให้หน้าแอดมิน
// แสดงผลได้ใน request เดียวโดยไม่ต้อง query แยกทีละปี.
type AcademicYearSummary struct {
	UUID            uuid.UUID `json:"uuid"`
	Year            int16     `json:"year"`
	IsCurrent       bool      `json:"is_current"`
	CurrentSemester int16     `json:"current_semester"`
	TopicCount      int64     `json:"topic_count"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateAcademicYearInput struct {
	Year            int16
	CurrentSemester *int16
}

type SetCurrentAcademicYearInput struct {
	Year            int16
	CurrentSemester *int16
}

// SetDivisionHead ตรวจผู้ใช้และฝ่ายภายใน transaction เดียว ก่อนแทนที่หัวหน้าคนเดิม.
// การ lock แถวฝ่ายและผู้ใช้ช่วยให้คำขอพร้อมกันไม่เขียนทับกันโดยไม่ตั้งใจ.
func (s *OrganizationService) SetDivisionHead(ctx context.Context, code string, userUUID uuid.UUID) (model.Division, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return model.Division{}, &ValidationError{Field: "code", Message: "division code is required"}
	}
	if userUUID == uuid.Nil {
		return model.Division{}, &ValidationError{Field: "user_uuid", Message: "user UUID is required"}
	}

	var result model.Division
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var division model.Division
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ?", code).
			Take(&division).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDivisionNotFound
			}
			return fmt.Errorf("find division: %w", err)
		}

		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uuid = ?", userUUID).
			Take(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("find division head candidate: %w", err)
		}
		if user.IsExec {
			return ErrExecutiveCannotHead
		}
		if user.DeactivatedAt != nil {
			return ErrInactiveUserCannotHead
		}

		// ตรวจล่วงหน้าเพื่อคืน conflict ที่อ่านง่าย ส่วน unique index ใน DB
		// ยังคงเป็นด่านสุดท้ายสำหรับ race condition.
		var assignedDivision model.Division
		err := tx.Where("head_user_uuid = ? AND uuid <> ?", userUUID, division.UUID).
			Take(&assignedDivision).Error
		switch {
		case err == nil:
			return ErrUserAlreadyDivisionHead
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return fmt.Errorf("check existing division head assignment: %w", err)
		}

		if err := tx.Model(&model.Division{}).
			Where("uuid = ?", division.UUID).
			Update("head_user_uuid", userUUID).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrUserAlreadyDivisionHead
			}
			return fmt.Errorf("set division head: %w", err)
		}

		if err := tx.Preload("HeadUser").Where("uuid = ?", division.UUID).Take(&result).Error; err != nil {
			return fmt.Errorf("load updated division head: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Division{}, err
	}
	return result, nil
}

// ListAcademicYears คืนปีล่าสุดก่อน พร้อมจำนวนหัวข้อที่ผูกกับแต่ละปี.
func (s *OrganizationService) ListAcademicYears(ctx context.Context) ([]AcademicYearSummary, error) {
	years := make([]AcademicYearSummary, 0)
	if err := s.db.WithContext(ctx).
		Table("academic_years").
		Select(academicYearSummarySelect).
		Order("academic_years.year DESC").
		Scan(&years).Error; err != nil {
		return nil, fmt.Errorf("list academic years: %w", err)
	}
	return years, nil
}

// CreateAcademicYear สร้างปีใหม่เป็นปีที่ยังไม่ใช่ปัจจุบันเสมอ
// เพื่อให้การสลับปีเกิดผ่าน SetCurrentAcademicYear เพียงจุดเดียว.
func (s *OrganizationService) CreateAcademicYear(ctx context.Context, input CreateAcademicYearInput) (AcademicYearSummary, error) {
	semester := int16(1)
	if input.CurrentSemester != nil {
		semester = *input.CurrentSemester
	}
	if err := validateAcademicYear(input.Year, semester); err != nil {
		return AcademicYearSummary{}, err
	}

	var result AcademicYearSummary
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		year := model.AcademicYear{
			Year:            input.Year,
			CurrentSemester: semester,
			IsCurrent:       false,
		}
		if err := tx.Create(&year).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrAcademicYearExists
			}
			return fmt.Errorf("create academic year: %w", err)
		}

		var err error
		result, err = findAcademicYearSummary(tx, input.Year)
		return err
	})
	if err != nil {
		return AcademicYearSummary{}, err
	}
	return result, nil
}

// SetCurrentAcademicYear lock ทุกแถวปีการศึกษาตามลำดับเดียวกันก่อนสลับสถานะ
// เพื่อ serialize คำขอพร้อมกัน และรักษากฎว่ามีปีปัจจุบันเพียงปีเดียว.
func (s *OrganizationService) SetCurrentAcademicYear(ctx context.Context, input SetCurrentAcademicYearInput) (AcademicYearSummary, error) {
	if err := validateAcademicYearNumber(input.Year); err != nil {
		return AcademicYearSummary{}, err
	}
	if input.CurrentSemester != nil {
		if err := validateSemester(*input.CurrentSemester); err != nil {
			return AcademicYearSummary{}, err
		}
	}

	var result AcademicYearSummary
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var years []model.AcademicYear
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("year").
			Find(&years).Error; err != nil {
			return fmt.Errorf("lock academic years: %w", err)
		}

		var target *model.AcademicYear
		for index := range years {
			if years[index].Year == input.Year {
				target = &years[index]
				break
			}
		}
		if target == nil {
			return ErrAcademicYearNotFound
		}

		semester := target.CurrentSemester
		if input.CurrentSemester != nil {
			semester = *input.CurrentSemester
		}
		if err := validateSemester(semester); err != nil {
			return err
		}

		if err := tx.Model(&model.AcademicYear{}).
			Where("is_current = ?", true).
			Update("is_current", false).Error; err != nil {
			return fmt.Errorf("clear current academic year: %w", err)
		}
		if err := tx.Model(&model.AcademicYear{}).
			Where("uuid = ?", target.UUID).
			Updates(map[string]any{
				"is_current":       true,
				"current_semester": semester,
			}).Error; err != nil {
			return fmt.Errorf("set current academic year: %w", err)
		}

		var err error
		result, err = findAcademicYearSummary(tx, input.Year)
		return err
	})
	if err != nil {
		return AcademicYearSummary{}, err
	}
	return result, nil
}

func findAcademicYearSummary(db *gorm.DB, year int16) (AcademicYearSummary, error) {
	var result AcademicYearSummary
	if err := db.Table("academic_years").
		Select(academicYearSummarySelect).
		Where("academic_years.year = ?", year).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AcademicYearSummary{}, ErrAcademicYearNotFound
		}
		return AcademicYearSummary{}, fmt.Errorf("find academic year summary: %w", err)
	}
	return result, nil
}

func validateAcademicYear(year, semester int16) error {
	if err := validateAcademicYearNumber(year); err != nil {
		return err
	}
	return validateSemester(semester)
}

func validateAcademicYearNumber(year int16) error {
	if year < 2400 || year > 3000 {
		return &ValidationError{Field: "year", Message: "academic year must be between 2400 and 3000"}
	}
	return nil
}

func validateSemester(semester int16) error {
	if semester != 1 && semester != 2 {
		return &ValidationError{Field: "current_semester", Message: "current semester must be 1 or 2"}
	}
	return nil
}
