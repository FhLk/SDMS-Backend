package migration

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateObjectStorageColumns เปลี่ยนชื่อ reference จาก Google Drive เป็น object key
// แบบ in-place จึงไม่คัดลอกหรือลบข้อมูลไฟล์เดิม และรันซ้ำได้อย่างปลอดภัย.
func migrateObjectStorageColumns(db *gorm.DB) error {
	if !db.Migrator().HasTable("files") {
		return nil
	}
	hasLegacy := db.Migrator().HasColumn("files", "drive_file_id")
	hasObjectKey := db.Migrator().HasColumn("files", "object_key")
	switch {
	case hasLegacy && !hasObjectKey:
		if err := db.Migrator().RenameColumn("files", "drive_file_id", "object_key"); err != nil {
			return fmt.Errorf("rename file storage column: %w", err)
		}
		return nil
	case !hasLegacy && hasObjectKey:
		return nil
	case hasLegacy && hasObjectKey:
		return fmt.Errorf("migrate object storage: files has both drive_file_id and object_key")
	default:
		return fmt.Errorf("migrate object storage: files has neither drive_file_id nor object_key")
	}
}
