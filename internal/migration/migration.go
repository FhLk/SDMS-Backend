// Package migration สร้าง schema และข้อมูลตั้งต้นที่ระบบต้องใช้ก่อนเปิด API.
package migration

import (
	"fmt"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var bangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

// DivisionSeed คือข้อมูลฝ่ายพื้นฐานที่ต้องมีในทุก environment.
type DivisionSeed struct {
	Code      string
	Name      string
	ShortName string
	SortOrder int16
}

// DefaultDivisions คืนข้อมูล 4 ฝ่ายตามข้อกำหนดของระบบ.
func DefaultDivisions() []DivisionSeed {
	return []DivisionSeed{
		{Code: "aca", Name: "ฝ่ายวิชาการ", ShortName: "วิชาการ", SortOrder: 1},
		{Code: "bud", Name: "ฝ่ายงบประมาณ", ShortName: "งบประมาณ", SortOrder: 2},
		{Code: "per", Name: "ฝ่ายบุคคล", ShortName: "บุคคล", SortOrder: 3},
		{Code: "gen", Name: "ฝ่ายทั่วไป", ShortName: "ทั่วไป", SortOrder: 4},
	}
}

// CurrentAcademicPeriod คำนวณปีการศึกษา พ.ศ. และภาคเรียนตามเวลาไทย:
// พฤษภาคม-ตุลาคมเป็นภาคเรียน 1 และพฤศจิกายน-เมษายนเป็นภาคเรียน 2.
func CurrentAcademicPeriod(now time.Time) (year int16, semester int16) {
	localTime := now.In(bangkokLocation)
	year = int16(localTime.Year() + 543)

	switch {
	case localTime.Month() < time.May:
		// มกราคม-เมษายนยังเป็นปีการศึกษาที่เริ่มในปีก่อนหน้า.
		year--
		semester = 2
	case localTime.Month() >= time.November:
		semester = 2
	default:
		semester = 1
	}

	return year, semester
}

// Up สร้างหรือปรับ schema ให้ตรงกับ GORM models และเติมข้อมูลตั้งต้น.
// ทุกขั้นตอนอยู่ใน transaction เดียว จึงไม่ทิ้ง schema ครึ่งหนึ่งเมื่อเกิดข้อผิดพลาด.
func Up(db *gorm.DB, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Existing installations used bigint and natural primary keys. Convert
		// them before AutoMigrate sees the UUID-based models so no data is lost.
		if err := migrateLegacyPrimaryKeys(tx); err != nil {
			return err
		}
		// รุ่น Google Drive ใช้ชื่อ drive_file_id; rename เป็น object_key เพื่อเก็บ
		// R2 key โดยคงค่าและความสัมพันธ์ของไฟล์เดิมทุกแถวไว้.
		if err := migrateObjectStorageColumns(tx); err != nil {
			return err
		}

		// Register the explicit join model so GORM uses its UUID primary key and
		// the UserUUID/DivisionUUID foreign-key columns.
		if err := tx.SetupJoinTable(&model.User{}, "Divisions", &model.UserDivision{}); err != nil {
			return fmt.Errorf("configure user division join table: %w", err)
		}

		if err := tx.AutoMigrate(
			&model.User{},
			&model.Division{},
			&model.UserDivision{},
			&model.AcademicYear{},
			&model.Topic{},
			&model.TopicField{},
			&model.Submission{},
			&model.File{},
			// ตารางต่อไปนี้รองรับความสัมพันธ์ที่ประกาศไว้ใน models และงานถัดไป.
			&model.TopicAssignee{},
			&model.Session{},
		); err != nil {
			return fmt.Errorf("migrate database schema: %w", err)
		}

		if err := seedDivisions(tx); err != nil {
			return err
		}
		if err := seedCurrentAcademicYear(tx, now); err != nil {
			return err
		}

		return nil
	})
}

func seedDivisions(db *gorm.DB) error {
	seeds := DefaultDivisions()
	divisions := make([]model.Division, 0, len(seeds))
	for _, seed := range seeds {
		divisions = append(divisions, model.Division{
			Code:      seed.Code,
			Name:      seed.Name,
			ShortName: seed.ShortName,
			SortOrder: seed.SortOrder,
		})
	}

	// DoNothing ทำให้ migration รันซ้ำได้โดยไม่เขียนทับชื่อฝ่ายที่ผู้ดูแลแก้ภายหลัง.
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&divisions).Error; err != nil {
		return fmt.Errorf("seed divisions: %w", err)
	}
	return nil
}

func seedCurrentAcademicYear(db *gorm.DB, now time.Time) error {
	year, semester := CurrentAcademicPeriod(now)

	// seed เฉพาะตอนระบบยังไม่มีปีปัจจุบัน เพื่อให้การรัน migration ซ้ำ
	// ไม่เขียนทับปี/ภาคเรียนที่แอดมินเลือกผ่าน BE-07 ไปแล้ว.
	var currentCount int64
	if err := db.Model(&model.AcademicYear{}).
		Where("is_current = ?", true).
		Count(&currentCount).Error; err != nil {
		return fmt.Errorf("check current academic year: %w", err)
	}
	if currentCount > 0 {
		return nil
	}

	current := model.AcademicYear{
		Year:            year,
		IsCurrent:       true,
		CurrentSemester: semester,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "year"}},
		DoUpdates: clause.Assignments(map[string]any{
			"is_current":       true,
			"current_semester": semester,
		}),
	}).Create(&current).Error; err != nil {
		return fmt.Errorf("seed current academic year: %w", err)
	}

	return nil
}
