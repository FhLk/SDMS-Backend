package migration

import (
	"os"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSeedCurrentAcademicYearPreservesAdminSelection(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	// จำลองปีที่แอดมินเลือกเอง แล้วรัน seed ด้วยวันที่ซึ่งคำนวณได้คนละปี.
	// seed ต้องไม่เขียนทับการตั้งค่าจาก BE-07 เมื่อ migration ถูกรันซ้ำ.
	if err := tx.Model(&model.AcademicYear{}).
		Where("is_current = ?", true).
		Update("is_current", false).Error; err != nil {
		t.Fatalf("clear current academic year: %v", err)
	}
	manual := model.AcademicYear{Year: 3000}
	if err := tx.Where("year = ?", manual.Year).
		FirstOrCreate(&manual, model.AcademicYear{CurrentSemester: 2}).Error; err != nil {
		t.Fatalf("find or create manually selected year: %v", err)
	}
	if err := tx.Model(&model.AcademicYear{}).
		Where("uuid = ?", manual.UUID).
		Updates(map[string]any{"is_current": true, "current_semester": 2}).Error; err != nil {
		t.Fatalf("select manual academic year: %v", err)
	}

	seedDate := time.Date(2026, time.June, 1, 12, 0, 0, 0, bangkokLocation)
	if err := seedCurrentAcademicYear(tx, seedDate); err != nil {
		t.Fatalf("seedCurrentAcademicYear() error = %v", err)
	}

	var current model.AcademicYear
	if err := tx.Where("is_current = ?", true).Take(&current).Error; err != nil {
		t.Fatalf("find current academic year: %v", err)
	}
	if current.Year != 3000 || current.CurrentSemester != 2 {
		t.Fatalf("current year = %+v, want preserved admin selection 3000 semester 2", current)
	}
}
