// Command import-checkin-teachers สร้างบัญชี SDMS จากครูในระบบ check-in
// ที่ใช้ฐานข้อมูลเดียวกัน ค่าเริ่มต้นเป็นโหมดทดลอง ต้องใส่ -apply จึงบันทึกจริง.
//
//	go run ./cmd/import-checkin-teachers                    # ดูแผนก่อน
//	go run ./cmd/import-checkin-teachers -division aca      # ฝ่ายเริ่มต้นของครูที่จับคู่ฝ่ายไม่ได้
//	go run ./cmd/import-checkin-teachers -division aca -apply
//	go run ./cmd/import-checkin-teachers -sync-passwords -apply
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/FhLk/SDMS-Backend/internal/checkinimport"
	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/FhLk/SDMS-Backend/internal/database"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	apply := flag.Bool("apply", false, "save the changes (default is a dry run that only prints the plan)")
	divisions := flag.String("division", "", "comma-separated division codes for teachers whose department matches no division, e.g. aca or aca,per")
	position := flag.String("position", checkinimport.DefaultPosition, "position text placed before the check-in department")
	withPrefix := flag.Bool("with-prefix", false, "deprecated: prefixes are always stored separately")
	syncPasswords := flag.Bool("sync-passwords", false, "copy the current check-in password hash to previously imported users and sign them out")
	source := flag.String("source", checkinimport.DefaultSource, "view that exposes check-in teachers (see deploy/supabase/checkin-teacher-view.sql)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("import failed: load configuration: %v", err)
	}

	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("import failed: access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	report, err := checkinimport.Run(context.Background(), db, checkinimport.Options{
		Source:          *source,
		DefaultPosition: *position,
		DivisionCodes:   splitList(*divisions),
		WithPrefix:      *withPrefix,
		SyncPasswords:   *syncPasswords,
		Apply:           *apply,
	})
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}
	if err := report.Write(os.Stdout); err != nil {
		log.Fatalf("import failed: write report: %v", err)
	}
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}
