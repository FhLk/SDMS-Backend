package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/config"
	httpapi "github.com/FhLk/SDMS-Backend/internal/delivery/http"
	postgresrepo "github.com/FhLk/SDMS-Backend/internal/repository/postgres"
	"github.com/FhLk/SDMS-Backend/internal/usecase"
	"github.com/FhLk/SDMS-Backend/pkg/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database connection: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			slog.Error("close database", "error", closeErr)
		}
	}()

	if cfg.Database.AutoMigrate {
		if err := postgresrepo.Migrate(db); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
	}

	userRepository := postgresrepo.NewUserRepository(db)
	userUsecase := usecase.NewUserUsecase(userRepository)
	app := httpapi.NewRouter(userUsecase, sqlDB)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("HTTP server starting", "port", cfg.App.Port, "environment", cfg.App.Environment)
		serverErr <- app.Listen(":" + cfg.App.Port)
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-shutdownSignal.Done():
		slog.Info("shutting down HTTP server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("shutdown server: %w", err)
	}

	return nil
}
