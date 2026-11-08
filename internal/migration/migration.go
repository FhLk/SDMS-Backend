// Package migration เตรียมโครงสร้างฐานข้อมูลว่างและข้อมูลตั้งต้นสำหรับ SDMS.
package migration

import (
	"errors"
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
		if err := migrateAccountsTable(tx); err != nil {
			return err
		}
		if err := dropLegacyUserDivisionsTable(tx); err != nil {
			return err
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

// dropLegacyUserDivisionsTable removes the obsolete per-user membership
// relation. Division access is universal; only divisions.head_user_uuid keeps
// a person-to-division relationship for authorization and administration.
func dropLegacyUserDivisionsTable(db *gorm.DB) error {
	if !db.Migrator().HasTable("user_divisions") {
		return nil
	}
	if err := db.Migrator().DropTable("user_divisions"); err != nil {
		return fmt.Errorf("drop obsolete user divisions table: %w", err)
	}
	return nil
}

// migrateAccountsTable preserves existing installations when the personnel
// table changes from users to accounts. AutoMigrate alone would create a new,
// empty accounts table and leave the old data behind, and it does not remove
// the obsolete position column.
func migrateAccountsTable(db *gorm.DB) error {
	migrator := db.Migrator()
	hasUsers := migrator.HasTable("users")
	hasAccounts := migrator.HasTable("accounts")
	if hasUsers && hasAccounts {
		return errors.New("both users and accounts tables exist; resolve the duplicate personnel tables before migrating")
	}
	if hasUsers {
		if err := migrator.RenameTable("users", "accounts"); err != nil {
			return fmt.Errorf("rename users table to accounts: %w", err)
		}
		hasAccounts = true
	}
	if !hasAccounts {
		return nil
	}

	for _, index := range []struct{ old, new string }{
		{old: "idx_users_username", new: "idx_accounts_username"},
		{old: "idx_users_deactivated_at", new: "idx_accounts_deactivated_at"},
	} {
		if migrator.HasIndex("accounts", index.old) && !migrator.HasIndex("accounts", index.new) {
			if err := migrator.RenameIndex("accounts", index.old, index.new); err != nil {
				return fmt.Errorf("rename accounts index %q: %w", index.old, err)
			}
		}
	}
	for _, constraint := range []struct{ old, new string }{
		{old: "users_pkey", new: "accounts_pkey"},
		{old: "chk_users_username", new: "chk_accounts_username"},
	} {
		if migrator.HasConstraint("accounts", constraint.old) && !migrator.HasConstraint("accounts", constraint.new) {
			statement := fmt.Sprintf("ALTER TABLE accounts RENAME CONSTRAINT %s TO %s", constraint.old, constraint.new)
			if err := db.Exec(statement).Error; err != nil {
				return fmt.Errorf("rename accounts constraint %q: %w", constraint.old, err)
			}
		}
	}
	if migrator.HasColumn("accounts", "position") {
		if err := migrator.DropColumn("accounts", "position"); err != nil {
			return fmt.Errorf("drop accounts position column: %w", err)
		}
	}
	return nil
}

// SchemaModels รวมเฉพาะตารางที่ระบบปัจจุบันใช้งานจริงตามลำดับการสร้าง.
func SchemaModels() []any {
	return []any{
		&model.User{},
		&model.Division{},
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
