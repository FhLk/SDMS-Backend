package main

import (
	"context"
	"log"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/FhLk/SDMS-Backend/internal/database"
	"github.com/FhLk/SDMS-Backend/internal/migration"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("initialize database failed: load configuration: %v", err)
	}

	// OpenForMigration สร้าง schema sdms ให้เองเมื่อใช้ PostgreSQL local.
	db, err := database.OpenForMigration(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("initialize database failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("initialize database failed: access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	// สร้างตารางและ seed ด้วยเวลาปัจจุบันเพื่อเลือกปีการศึกษาและภาคเรียน.
	if err := migration.Up(db, time.Now()); err != nil {
		log.Fatalf("initialize database failed: %v", err)
	}

	log.Printf("database initialized successfully (schema %q)", cfg.Database.Schema)
}
