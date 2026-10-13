package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadWithDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/sdms?sslmode=disable&TimeZone=Asia%2FBangkok")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("DB_CONNECT_TIMEOUT", "3s")
	t.Setenv("DB_MAX_IDLE_CONNS", "2")
	t.Setenv("DB_MAX_OPEN_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFETIME", "30m")
	t.Setenv("SESSION_TTL", "12h")
	t.Setenv("SESSION_COOKIE_NAME", "school_session")
	t.Setenv("SESSION_COOKIE_SECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Server.Address(), "127.0.0.1:9000"; got != want {
		t.Fatalf("Server.Address() = %q, want %q", got, want)
	}
	if got, want := cfg.Database.ConnectTimeout, 3*time.Second; got != want {
		t.Fatalf("ConnectTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.Database.ConnMaxLifetime, 30*time.Minute; got != want {
		t.Fatalf("ConnMaxLifetime = %v, want %v", got, want)
	}
	if got, want := cfg.Auth.SessionTTL, 12*time.Hour; got != want {
		t.Fatalf("Auth.SessionTTL = %v, want %v", got, want)
	}
	if got, want := cfg.Auth.CookieName, "school_session"; got != want {
		t.Fatalf("Auth.CookieName = %q, want %q", got, want)
	}
	if !cfg.Auth.CookieSecure {
		t.Fatal("Auth.CookieSecure = false, want true")
	}
	if strings.Contains(cfg.Database.DSN, "Asia%2FBangkok") || !strings.Contains(cfg.Database.DSN, "TimeZone=Asia/Bangkok") {
		t.Fatalf("Database.DSN = %q, want an unescaped IANA timezone", cfg.Database.DSN)
	}
}

func TestLoadBuildsDatabaseURLFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "sdms user")
	t.Setenv("DB_PASSWORD", "p@ss word")
	t.Setenv("DB_NAME", "sdms")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DB_TIMEZONE", "Asia/Bangkok")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, part := range []string{
		"postgres://sdms%20user:p%40ss%20word@localhost:5432/sdms?",
		"TimeZone=Asia/Bangkok",
		"sslmode=require",
	} {
		if !strings.Contains(cfg.Database.DSN, part) {
			t.Errorf("Database.DSN = %q, want it to contain %q", cfg.Database.DSN, part)
		}
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/sdms")
	t.Setenv("SERVER_PORT", "70000")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SERVER_PORT") {
		t.Fatalf("Load() error = %v, want invalid SERVER_PORT error", err)
	}
}

func TestLoadRejectsInvalidAuthConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/sdms")
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("SESSION_COOKIE_NAME", "not a cookie")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SESSION_COOKIE_NAME") {
		t.Fatalf("Load() error = %v, want invalid SESSION_COOKIE_NAME error", err)
	}
}

func TestLoadUsesLocalUploadStorageByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/sdms")
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("FILE_STORAGE_DRIVER", "local")
	t.Setenv("LOCAL_UPLOAD_DIR", "test-uploads")
	t.Setenv("STAGED_UPLOAD_TTL", "24h")
	t.Setenv("UPLOAD_CLEANUP_INTERVAL", "30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Uploads.Driver != "local" || cfg.Uploads.LocalDirectory != "test-uploads" {
		t.Fatalf("Uploads = %+v", cfg.Uploads)
	}
	if cfg.Uploads.StagedTTL != 24*time.Hour || cfg.Uploads.CleanupInterval != 30*time.Minute {
		t.Fatalf("upload durations = %+v", cfg.Uploads)
	}
}

func TestLoadRequiresR2Settings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/sdms")
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("FILE_STORAGE_DRIVER", "r2")
	t.Setenv("R2_ACCOUNT_ID", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_BUCKET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "R2_ACCOUNT_ID") {
		t.Fatalf("Load() error = %v, want missing R2 account error", err)
	}
}

func TestLoadR2UploadStorage(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/sdms")
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("FILE_STORAGE_DRIVER", "r2")
	t.Setenv("R2_ACCOUNT_ID", "account")
	t.Setenv("R2_ACCESS_KEY_ID", "access")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	t.Setenv("R2_SESSION_TOKEN", "temporary")
	t.Setenv("R2_BUCKET", "sdms-files")
	t.Setenv("R2_PREFIX", "school/sdms")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Uploads.Driver != "r2" || cfg.Uploads.R2.Bucket != "sdms-files" || cfg.Uploads.R2.Prefix != "school/sdms" {
		t.Fatalf("Uploads.R2 = %+v", cfg.Uploads.R2)
	}
	if cfg.Uploads.R2.SessionToken != "temporary" {
		t.Fatalf("R2.SessionToken = %q, want temporary", cfg.Uploads.R2.SessionToken)
	}
}
