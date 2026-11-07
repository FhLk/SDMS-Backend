package server

import (
	_ "embed"

	"github.com/gofiber/contrib/v3/swaggerui"
	"github.com/gofiber/fiber/v3"
)

// openAPISpec is embedded so the API binary can serve its contract regardless
// of the process working directory used in development or production.
//
//go:embed openapi.yaml
var openAPISpec []byte

func swaggerDocumentation() fiber.Handler {
	return swaggerui.New(swaggerui.Config{
		BasePath:    "/api/",
		FilePath:    "openapi.yaml",
		FileContent: openAPISpec,
		Path:        "docs",
		Title:       "SDMS API Documentation",
		CacheAge:    300,
	})
}
