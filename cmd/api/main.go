package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/FhLk/SDMS-Backend/internal/database"
	"github.com/FhLk/SDMS-Backend/internal/server"
	"github.com/gofiber/fiber/v3"
)

func main() {
	// Keep timestamps consistent across machines and deployment environments.
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	// Load and validate every required setting before starting any dependency.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("application startup failed: load configuration: %v", err)
	}

	// database.Open also pings PostgreSQL. The API must not start when the
	// database is unavailable because every later feature depends on it.
	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("application startup failed: %v", err)
	}

	// GORM wraps database/sql. Keep the underlying pool so it can be closed
	// cleanly when the process exits.
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("application startup failed: access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	app := server.New()
	// Listen runs in a goroutine so the main goroutine can wait for either a
	// server failure or an operating-system shutdown signal.
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on %s", cfg.Server.Address())
		serverErrors <- app.Listen(cfg.Server.Address(), fiber.ListenConfig{
			DisableStartupMessage: true,
		})
	}()

	// SIGINT handles Ctrl+C, while SIGTERM is used by Docker and most process
	// managers when replacing or stopping an application instance.
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownSignals)

	select {
	case err := <-serverErrors:
		// A listener error is fatal because the API can no longer serve requests.
		if err != nil && !errors.Is(err, fiber.ErrServiceUnavailable) {
			log.Fatalf("HTTP server stopped unexpectedly: %v", err)
		}
	case sig := <-shutdownSignals:
		log.Printf("received %s; shutting down", sig)
		// Give in-flight requests time to finish, but do not let shutdown wait
		// forever for a stuck connection.
		if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
			log.Printf("HTTP server shutdown failed: %v", err)
		}
	}
}
