// Package database creates and configures the application's database connection.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open creates and verifies a PostgreSQL connection before returning it.
// It also checks that unqualified table names resolve to cfg.Schema, so SDMS
// never reads or creates tables in another application's schema by accident.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	return open(ctx, cfg, false)
}

// OpenForMigration works like Open but first creates cfg.Schema when it does
// not exist yet and the database role is allowed to create schemas (for
// example the local docker user). On Supabase the schema is created by
// deploy/supabase/sdms-schema.sql because the application role cannot.
func OpenForMigration(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	return open(ctx, cfg, true)
}

func open(ctx context.Context, cfg config.DatabaseConfig, createSchema bool) (*gorm.DB, error) {
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

	if cfg.Schema != "" {
		if createSchema {
			if err := EnsureSchema(ctx, sqlDB, cfg.Schema); err != nil {
				_ = sqlDB.Close()
				return nil, err
			}
		}
		if err := VerifySchema(ctx, sqlDB, cfg.Schema); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}

	return db, nil
}

// EnsureSchema creates schema when it is missing. Existence is checked first
// because CREATE SCHEMA IF NOT EXISTS still requires CREATE on the database,
// which a least-privilege role such as sdms_app does not have.
func EnsureSchema(ctx context.Context, db *sql.DB, schema string) error {
	var exists bool
	if err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = $1)", schema,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check database schema %q: %w", schema, err)
	}
	if exists {
		return nil
	}

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		return fmt.Errorf("database schema %q does not exist and this role cannot create it (%v); "+
			"on Supabase run deploy/supabase/sdms-schema.sql as postgres first", schema, err)
	}
	return nil
}

// VerifySchema confirms that current_schema() is schema. Unqualified table
// names in GORM models and raw SQL resolve through search_path, so this one
// check guarantees every query and every AutoMigrate statement targets SDMS
// tables instead of the shared database's public schema.
func VerifySchema(ctx context.Context, db *sql.DB, schema string) error {
	var role string
	var current sql.NullString
	var exists bool
	if err := db.QueryRowContext(ctx,
		"SELECT current_user, current_schema(), EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = $1)", schema,
	).Scan(&role, &current, &exists); err != nil {
		return fmt.Errorf("check database schema: %w", err)
	}
	if current.Valid && current.String == schema {
		return nil
	}

	if !exists {
		return fmt.Errorf("database schema %q does not exist; run `go run ./cmd/migrate` "+
			"(on Supabase run deploy/supabase/sdms-schema.sql first, see sdms-shared-database.md)", schema)
	}
	got := "no existing schema"
	if current.Valid {
		got = fmt.Sprintf("schema %q", current.String)
	}
	// ไม่แนะนำให้ ALTER ROLE ของ role ที่ต่ออยู่ เพราะอาจเป็น postgres ที่ระบบ check-in ใช้
	// การเปลี่ยน search_path ของ role นั้นจะทำให้ check-in หาตารางของตัวเองไม่เจอ.
	return fmt.Errorf("database role %q resolves tables to %s instead of %q; connect as the dedicated SDMS role "+
		"created by deploy/supabase/sdms-schema.sql (its search_path is already %s) or add search_path=%s to DATABASE_URL. "+
		"Do not change the search_path of a role that other applications use (see sdms-shared-database.md)",
		role, got, schema, schema, schema)
}
