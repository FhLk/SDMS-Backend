// Package database creates and configures the application's database connection.
package database

import (
	"context"
	"fmt"

	"github.com/FhLk/SDMS-Backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open creates and verifies a PostgreSQL connection before returning it.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	// The PostgreSQL dialector lets GORM use pgx through database/sql.
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		// แปลง error เฉพาะฐานข้อมูล เช่น unique violation เป็น error มาตรฐานของ GORM.
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access PostgreSQL connection: %w", err)
	}

	// Configure the shared database/sql pool once. All GORM operations reuse it.
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Opening a database/sql handle can be lazy, so an explicit ping proves that
	// PostgreSQL is reachable before the HTTP server starts accepting requests.
	pingContext, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingContext); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	return db, nil
}
