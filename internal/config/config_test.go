package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	keys := []string{
		"APP_ENV", "APP_PORT", "DB_HOST", "DB_PORT", "DB_USER",
		"DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "DB_TIMEZONE",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}

	cfg := Load()

	if cfg.App.Env != "development" || cfg.App.Port != "8080" {
		t.Errorf("App config = %+v", cfg.App)
	}
	if cfg.Database.Host != "localhost" || cfg.Database.Port != "5432" {
		t.Errorf("database address = %s:%s", cfg.Database.Host, cfg.Database.Port)
	}
	if cfg.Database.User != "sdms" || cfg.Database.Password != "" || cfg.Database.Name != "sdms_db" {
		t.Errorf("database credentials/name = %+v", cfg.Database)
	}
	if cfg.Database.SSLMode != "disable" || cfg.Database.TimeZone != "Asia/Bangkok" {
		t.Errorf("database options = %+v", cfg.Database)
	}
}

func TestLoadUsesEnvironmentValues(t *testing.T) {
	values := map[string]string{
		"APP_ENV": "test", "APP_PORT": "9090", "DB_HOST": "db",
		"DB_PORT": "6432", "DB_USER": "tester", "DB_PASSWORD": "secret",
		"DB_NAME": "sdms_test", "DB_SSLMODE": "require", "DB_TIMEZONE": "UTC",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}

	cfg := Load()

	if cfg.App.Env != "test" || cfg.App.Port != "9090" {
		t.Errorf("App config = %+v", cfg.App)
	}
	if cfg.Database.Host != "db" || cfg.Database.Port != "6432" || cfg.Database.User != "tester" ||
		cfg.Database.Password != "secret" || cfg.Database.Name != "sdms_test" ||
		cfg.Database.SSLMode != "require" || cfg.Database.TimeZone != "UTC" {
		t.Errorf("Database config = %+v", cfg.Database)
	}
}

func TestDatabaseConfigDSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host: "db", Port: "6432", User: "tester", Password: "secret",
		Name: "sdms_test", SSLMode: "require", TimeZone: "UTC",
	}
	want := "host=db user=tester password=secret dbname=sdms_test port=6432 sslmode=require TimeZone=UTC"

	if got := cfg.DSN(); got != want {
		t.Fatalf("DSN() = %q, want %q", got, want)
	}
}

func TestGetEnvTreatsEmptyValueAsMissing(t *testing.T) {
	t.Setenv("SDMS_TEST_VALUE", "")
	if got := getEnv("SDMS_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("getEnv() = %q, want fallback", got)
	}

	t.Setenv("SDMS_TEST_VALUE", "configured")
	if got := getEnv("SDMS_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("getEnv() = %q, want configured", got)
	}
}
