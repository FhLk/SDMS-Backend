package database

import (
	"fmt"
	"log"

	"sdms/internal/config"

	topicpostgres "sdms/internal/modules/topic/repository/postgres"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgres(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(
		postgres.Open(cfg.DSN()),
		&gorm.Config{},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := db.AutoMigrate(
		&topicpostgres.TopicModel{},
	); err != nil {
		log.Fatal(err)
	}

	return db, nil
}
