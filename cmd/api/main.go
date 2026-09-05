package main

import (
	"log"

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
