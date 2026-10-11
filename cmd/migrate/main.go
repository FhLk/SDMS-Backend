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
		log.Fatalf("migration failed: load configuration: %v", err)
	}

	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("migration failed: access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	// ส่งเวลาปัจจุบันเข้า migration เพื่อเลือกปีการศึกษาและภาคเรียนตามเวลาไทย.
	if err := migration.Up(db, time.Now()); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("migration completed successfully")
}
