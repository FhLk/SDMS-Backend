// Package config loads and validates runtime configuration for the API.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultServerHost         = "0.0.0.0"
	defaultServerPort         = "8080"
	defaultDatabasePort       = "5432"
	defaultDatabaseSSLMode    = "disable"
	defaultDatabaseTimeZone   = "Asia/Bangkok"
	defaultConnectTimeout     = 5 * time.Second
	defaultMaxIdleConnections = 5
	defaultMaxOpenConnections = 20
	defaultConnectionLifetime = time.Hour
)

// Config contains all settings required to start the application.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

// ServerConfig controls which network address the HTTP server listens on.
type ServerConfig struct {
	Host string
	Port string
}

// Address returns a host:port pair and correctly handles IPv6 hosts.
func (c ServerConfig) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}

// DatabaseConfig contains the PostgreSQL DSN and database/sql pool settings.
type DatabaseConfig struct {
	DSN             string
	ConnectTimeout  time.Duration
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// Load reads .env when present and then resolves the process environment.
// Existing environment variables take precedence over values in .env.
func Load() (Config, error) {
	// godotenv does not overwrite variables already present in the process
	// environment, so deployment secrets take precedence over local .env values.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read .env: %w", err)
	}

	// PORT is supported as a fallback for hosting platforms that inject it.
	serverPort := firstNonEmpty(os.Getenv("SERVER_PORT"), os.Getenv("PORT"), defaultServerPort)
	if err := validatePort("SERVER_PORT", serverPort); err != nil {
		return Config{}, err
	}

	dsn, err := databaseDSN()
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := durationFromEnv("DB_CONNECT_TIMEOUT", defaultConnectTimeout)
	if err != nil {
		return Config{}, err
	}

	maxIdleConns, err := positiveIntFromEnv("DB_MAX_IDLE_CONNS", defaultMaxIdleConnections, true)
	if err != nil {
		return Config{}, err
	}

	maxOpenConns, err := positiveIntFromEnv("DB_MAX_OPEN_CONNS", defaultMaxOpenConnections, false)
	if err != nil {
		return Config{}, err
	}
	if maxIdleConns > maxOpenConns {
		return Config{}, errors.New("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}

	connMaxLifetime, err := durationFromEnv("DB_CONN_MAX_LIFETIME", defaultConnectionLifetime)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Server: ServerConfig{
			Host: firstNonEmpty(os.Getenv("SERVER_HOST"), defaultServerHost),
			Port: serverPort,
		},
		Database: DatabaseConfig{
			DSN:             dsn,
			ConnectTimeout:  connectTimeout,
			MaxIdleConns:    maxIdleConns,
			MaxOpenConns:    maxOpenConns,
			ConnMaxLifetime: connMaxLifetime,
		},
	}, nil
}

// databaseDSN prefers a complete DATABASE_URL, which is common in production.
// For local development it safely builds the same URL from the individual DB_*
// variables and escapes credentials that contain special characters.
func databaseDSN() (string, error) {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		parsed, err := url.Parse(dsn)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", errors.New("DATABASE_URL must be a valid PostgreSQL URL")
		}
		if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
			return "", errors.New("DATABASE_URL scheme must be postgres or postgresql")
		}

		// Normalize an encoded timezone for the same PostgreSQL driver behavior as
		// the DB_* form below, while preserving every other query parameter.
		query := parsed.Query()
		if timeZone := query.Get("TimeZone"); timeZone != "" {
			if _, err := time.LoadLocation(timeZone); err != nil {
				return "", fmt.Errorf("DATABASE_URL TimeZone must be a valid IANA time zone: %w", err)
			}
			query.Del("TimeZone")
			parsed.RawQuery = query.Encode()
			if parsed.RawQuery != "" {
				parsed.RawQuery += "&"
			}
			parsed.RawQuery += "TimeZone=" + timeZone
			dsn = parsed.String()
		}

		return dsn, nil
	}

	host, err := requiredEnv("DB_HOST")
	if err != nil {
		return "", err
	}
	user, err := requiredEnv("DB_USER")
	if err != nil {
		return "", err
	}
	databaseName, err := requiredEnv("DB_NAME")
	if err != nil {
		return "", err
	}

	port := firstNonEmpty(os.Getenv("DB_PORT"), defaultDatabasePort)
	if err := validatePort("DB_PORT", port); err != nil {
		return "", err
	}

	password := os.Getenv("DB_PASSWORD")
	credentials := url.User(user)
	if password != "" {
		credentials = url.UserPassword(user, password)
	}

	sslMode := firstNonEmpty(os.Getenv("DB_SSLMODE"), defaultDatabaseSSLMode)
	timeZone := firstNonEmpty(os.Getenv("DB_TIMEZONE"), defaultDatabaseTimeZone)
	if _, err := time.LoadLocation(timeZone); err != nil {
		return "", fmt.Errorf("DB_TIMEZONE must be a valid IANA time zone: %w", err)
	}

	// Keep the slash in an IANA timezone such as Asia/Bangkok. PostgreSQL's DSN
	// parser treats an encoded slash (%2F) as literal text instead of decoding it.
	query := "sslmode=" + url.QueryEscape(sslMode) + "&TimeZone=" + timeZone

	return (&url.URL{
		Scheme:   "postgres",
		User:     credentials,
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + databaseName,
		RawQuery: query,
	}).String(), nil
}

// requiredEnv reads a value that is mandatory when using individual DB_* settings.
func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required when DATABASE_URL is not set", key)
	}
	return value, nil
}

// validatePort rejects values that cannot be valid TCP ports before startup.
func validatePort(key, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a number between 1 and 65535", key)
	}
	return nil
}

// durationFromEnv parses Go duration strings such as "500ms", "5s", or "1h".
func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return duration, nil
}

// positiveIntFromEnv parses connection-pool sizes and optionally permits zero.
func positiveIntFromEnv(key string, fallback int, allowZero bool) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	number, err := strconv.Atoi(value)
	minimum := 1
	if allowZero {
		minimum = 0
	}
	if err != nil || number < minimum {
		if allowZero {
			return 0, fmt.Errorf("%s must be zero or a positive integer", key)
		}
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return number, nil
}

// firstNonEmpty returns the first usable value, allowing simple precedence chains.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
