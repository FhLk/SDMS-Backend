package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/config"
)

func integrationConfig(t *testing.T, schema string) config.DatabaseConfig {
	t.Helper()
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}
	return config.DatabaseConfig{
		DSN:             dsn,
		Schema:          schema,
		ConnectTimeout:  5 * time.Second,
		MaxIdleConns:    1,
		MaxOpenConns:    2,
		ConnMaxLifetime: time.Minute,
	}
}

func TestOpenVerifiesSchemaWithPostgreSQL(t *testing.T) {
	// อ่าน schema ปัจจุบันของ role ทดสอบก่อน (ปกติคือ sdms) แล้วใช้เป็นค่าที่ถูกต้อง.
	probe, err := Open(context.Background(), integrationConfig(t, ""))
	if err != nil {
		t.Fatalf("Open() without schema check error = %v", err)
	}
	sqlDB, _ := probe.DB()
	var current string
	if err := sqlDB.QueryRow("SELECT current_schema()").Scan(&current); err != nil {
		t.Fatalf("read current schema: %v", err)
	}
	if err := EnsureSchema(context.Background(), sqlDB, current); err != nil {
		t.Fatalf("EnsureSchema() on existing schema error = %v, want no-op", err)
	}
	_ = sqlDB.Close()

	db, err := Open(context.Background(), integrationConfig(t, current))
	if err != nil {
		t.Fatalf("Open() with schema %q error = %v", current, err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	tests := []struct {
		schema string
		want   string
	}{
		{schema: "sdms_schema_that_does_not_exist", want: "does not exist"},
		{schema: "information_schema", want: "resolves tables to"},
	}
	for _, test := range tests {
		t.Run(test.schema, func(t *testing.T) {
			_, err := Open(context.Background(), integrationConfig(t, test.schema))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Open() error = %v, want %q", err, test.want)
			}
		})
	}
}
