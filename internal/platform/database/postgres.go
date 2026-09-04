package database

import (
	"fmt"

	"sdms/internal/config"

	submissionpostgres "sdms/internal/modules/submission/repository/postgres"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	userpostgres "sdms/internal/modules/user/repository/postgres"

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
		&userpostgres.UserModel{},
		&topicpostgres.TopicFieldModel{},
		&submissionpostgres.SubmissionModel{},
		&submissionpostgres.SubmissionValueModel{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}
