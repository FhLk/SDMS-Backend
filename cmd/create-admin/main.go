package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/FhLk/SDMS-Backend/internal/database"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	// รองรับทั้ง flags และ environment variable เพื่อไม่บังคับให้ใส่รหัสผ่านใน command history.
	username := flag.String("username", os.Getenv("INITIAL_ADMIN_USERNAME"), "admin username")
	password := flag.String("password", os.Getenv("INITIAL_ADMIN_PASSWORD"), "admin password (prefer INITIAL_ADMIN_PASSWORD)")
	prefix := flag.String("prefix", os.Getenv("INITIAL_ADMIN_PREFIX"), "admin name prefix")
	firstName := flag.String("first-name", os.Getenv("INITIAL_ADMIN_FIRST_NAME"), "admin first name")
	lastName := flag.String("last-name", os.Getenv("INITIAL_ADMIN_LAST_NAME"), "admin last name")
	fullName := flag.String("name", os.Getenv("INITIAL_ADMIN_NAME"), "legacy full name fallback")
	position := flag.String("position", os.Getenv("INITIAL_ADMIN_POSITION"), "admin position")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("create admin failed: load configuration: %v", err)
	}

	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("create admin failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("create admin failed: access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	created, err := admin.Create(context.Background(), db, admin.CreateInput{
		Username:  *username,
		Password:  *password,
		Prefix:    *prefix,
		FirstName: *firstName,
		LastName:  *lastName,
		FullName:  *fullName,
		Position:  *position,
	})
	if err != nil {
		log.Fatalf("create admin failed: %v", err)
	}

	log.Printf("admin %q created successfully (uuid=%s)", created.Username, created.UUID)
}
