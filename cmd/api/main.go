package main

import (
	"log"
	"strings"

	"sdms/internal/config"
	"sdms/internal/platform/database"
	httpserver "sdms/internal/platform/http"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found")
	}

	cfg := config.Load()
	if strings.EqualFold(strings.TrimSpace(cfg.App.Env), "production") {
		secret := strings.TrimSpace(cfg.Auth.JWTSecret)
		if len(secret) < 32 || secret == "dev-only-change-this-secret" || secret == "change-this-to-a-long-random-secret" {
			log.Fatal("AUTH_JWT_SECRET must be a non-placeholder secret of at least 32 characters in production")
		}
	}

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("database connected")

	app := httpserver.NewRouter(db, cfg)

	address := ":" + cfg.App.Port

	log.Printf("server running on http://localhost%s", address)

	log.Fatal(app.Listen(address))
}
