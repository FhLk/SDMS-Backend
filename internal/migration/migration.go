// Package migration เตรียมโครงสร้างฐานข้อมูลว่างและข้อมูลตั้งต้นสำหรับ SDMS.
package migration

import (
	"fmt"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var bangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

// DivisionSeed คือข้อมูลฝ่ายพื้นฐานที่ทุก environment ต้องมี.
type DivisionSeed struct {
	Code      string
	Name      string
	ShortName string
	SortOrder int16
}

// DefaultDivisions คืนข้อมูลตั้งต้นของฝ่ายทั้ง 4 ฝ่าย.
func DefaultDivisions() []DivisionSeed {
	return []DivisionSeed{
		{Code: "aca", Name: "ฝ่ายวิชาการ", ShortName: "วิชาการ", SortOrder: 1},
		{Code: "bud", Name: "ฝ่ายงบประมาณ", ShortName: "งบประมาณ", SortOrder: 2},
		{Code: "per", Name: "ฝ่ายบุคคล", ShortName: "บุคคล", SortOrder: 3},
		{Code: "gen", Name: "ฝ่ายทั่วไป", ShortName: "ทั่วไป", SortOrder: 4},
	}
}

// CurrentAcademicPeriod คำนวณปีการศึกษา พ.ศ. และภาคเรียนด้วยเวลาไทย.
// พฤษภาคมถึงตุลาคมเป็นภาคเรียน 1 ส่วนพฤศจิกายนถึงเมษายนเป็นภาคเรียน 2.
func CurrentAcademicPeriod(now time.Time) (year int16, semester int16) {
	localTime := now.In(bangkokLocation)
	year = int16(localTime.Year() + 543)

	switch {
	case localTime.Month() < time.May:
		year--
		semester = 2
	case localTime.Month() >= time.November:
		semester = 2
	default:
		semester = 1
	}

	return year, semester
}

// Up สร้างตารางทั้งหมดจาก GORM models และเติมข้อมูลตั้งต้น.
// initializer ชุดนี้ออกแบบสำหรับฐานข้อมูลใหม่ จึงไม่มีขั้นตอนแปลง schema รุ่นเก่า.
// การรันซ้ำทำได้โดยไม่สร้างฝ่ายหรือปีการศึกษาซ้ำ.
func Up(db *gorm.DB, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// กำหนด join model เองเพื่อให้ user_divisions มี UUID primary key ตามตารางอื่น.
		if err := tx.SetupJoinTable(&model.User{}, "Divisions", &model.UserDivision{}); err != nil {
			return fmt.Errorf("configure user division join table: %w", err)
		}

		if err := tx.AutoMigrate(SchemaModels()...); err != nil {
			return fmt.Errorf("create database schema: %w", err)
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

// SchemaModels รวมเฉพาะตารางที่ระบบปัจจุบันใช้งานจริงตามลำดับการสร้าง.
func SchemaModels() []any {
	return []any{
		&model.User{},
		&model.Division{},
		&model.UserDivision{},
		&model.AcademicYear{},
		&model.Topic{},
		&model.TopicField{},
		&model.TopicAssignee{},
		&model.Submission{},
		&model.File{},
		&model.Session{},
	}
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

	// ไม่เขียนทับชื่อฝ่ายหากแอดมินแก้ไขไปแล้วและสั่ง initializer ซ้ำ.
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&divisions).Error; err != nil {
		return fmt.Errorf("seed divisions: %w", err)
	}
	return nil
}

func seedCurrentAcademicYear(db *gorm.DB, now time.Time) error {
	var currentCount int64
	if err := db.Model(&model.AcademicYear{}).
		Where("is_current = ?", true).
		Count(&currentCount).Error; err != nil {
		return fmt.Errorf("check current academic year: %w", err)
	}
	if currentCount > 0 {
		return nil
	}

	year, semester := CurrentAcademicPeriod(now)
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
