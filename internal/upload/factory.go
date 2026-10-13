package upload

import (
	"context"
	"fmt"

	"github.com/FhLk/SDMS-Backend/internal/config"
)

// NewObjectStore สร้าง adapter ตามค่าเดียวใน .env เพื่อให้ service ใช้ flow เดิมทุก environment.
func NewObjectStore(ctx context.Context, cfg config.UploadConfig) (ObjectStore, error) {
	switch cfg.Driver {
	case "local":
		return NewLocalStore(cfg.LocalDirectory)
	case "r2":
		return NewR2Store(ctx, cfg.R2)
	default:
		return nil, fmt.Errorf("unsupported file storage driver %q", cfg.Driver)
	}
}
