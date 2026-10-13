// Package upload จัดการวงจรชีวิตไฟล์ตั้งแต่อัปโหลดแบบ staged จนถึงนำไปทิ้ง.
package upload

import (
	"context"
	"io"
)

// ObjectStore แยกรายละเอียดของ local filesystem และ Cloudflare R2 ออกจาก use case.
// location อาจเป็น directory จริงหรือ prefix ของ object key โดย service ไม่ต้องรู้ชนิดปลายทาง.
type ObjectStore interface {
	RootID() string
	EnsureFolder(context.Context, string, string) (string, error)
	Put(context.Context, string, string, string, int64, io.Reader) (string, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Trash(context.Context, string) error
}
