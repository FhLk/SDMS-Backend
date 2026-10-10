package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
}

type AppConfig struct {
	Environment string
	Port        string
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	TimeZone        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	AutoMigrate     bool
}

func Load() (Config, error) {
	maxIdleConns, err := envInt("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	maxOpenConns, err := envInt("DB_MAX_OPEN_CONNS", 100)
	if err != nil {
		return Config{}, err
	}
	connMaxLifetime, err := envDuration("DB_CONN_MAX_LIFETIME", time.Hour)
	if err != nil {
		return Config{}, err
	}
	autoMigrate, err := envBool("DB_AUTO_MIGRATE", true)
	if err != nil {
		return Config{}, err
	}

	return Config{
		App: AppConfig{
			Environment: env("APP_ENV", "development"),
			Port:        env("APP_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:            env("DB_HOST", "localhost"),
			Port:            env("DB_PORT", "5432"),
			User:            env("DB_USER", "postgres"),
			Password:        env("DB_PASSWORD", "postgres"),
			Name:            env("DB_NAME", "sdms"),
			SSLMode:         env("DB_SSLMODE", "disable"),
			TimeZone:        env("DB_TIMEZONE", "Asia/Bangkok"),
			MaxIdleConns:    maxIdleConns,
			MaxOpenConns:    maxOpenConns,
			ConnMaxLifetime: connMaxLifetime,
			AutoMigrate:     autoMigrate,
		},
	}, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must not be negative", key)
	}
	return parsed, nil
}

func envBool(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must not be negative", key)
	}
	return parsed, nil
}
