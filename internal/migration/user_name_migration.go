package migration

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateUserNameColumns converts existing installations from users.full_name
// to prefix/first_name/last_name. Existing text is kept verbatim in first_name
// because an arbitrary display name cannot be split reliably without user
// input. Administrators can refine the components later through the API.
func migrateUserNameColumns(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable("users") {
		return nil
	}

	for _, statement := range []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS prefix varchar(30)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name varchar(120)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name varchar(120)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("add structured user name columns: %w", err)
		}
	}

	if migrator.HasColumn("users", "full_name") {
		if err := db.Exec(`
			UPDATE users
			SET first_name = COALESCE(NULLIF(BTRIM(first_name), ''), BTRIM(full_name)),
			    prefix = COALESCE(prefix, ''),
			    last_name = COALESCE(last_name, '')
		`).Error; err != nil {
			return fmt.Errorf("backfill structured user names: %w", err)
		}
	} else {
		if err := db.Exec(`
			UPDATE users
			SET first_name = COALESCE(first_name, ''),
			    prefix = COALESCE(prefix, ''),
			    last_name = COALESCE(last_name, '')
		`).Error; err != nil {
			return fmt.Errorf("normalize structured user names: %w", err)
		}
	}

	for _, statement := range []string{
		`ALTER TABLE users ALTER COLUMN prefix SET DEFAULT ''`,
		`ALTER TABLE users ALTER COLUMN prefix SET NOT NULL`,
		`ALTER TABLE users ALTER COLUMN first_name SET NOT NULL`,
		`ALTER TABLE users ALTER COLUMN last_name SET DEFAULT ''`,
		`ALTER TABLE users ALTER COLUMN last_name SET NOT NULL`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("constrain structured user name columns: %w", err)
		}
	}

	if migrator.HasColumn("users", "full_name") {
		if err := db.Exec(`ALTER TABLE users DROP COLUMN full_name`).Error; err != nil {
			return fmt.Errorf("drop legacy users.full_name: %w", err)
		}
	}
	return backfillCheckinUserNames(db)
}

// backfillCheckinUserNames restores the three authoritative components for
// accounts imported before structured names were introduced. It only touches
// rows that still exactly match one of the legacy importer display formats,
// so a name subsequently edited through the SDMS API is never overwritten.
func backfillCheckinUserNames(db *gorm.DB) error {
	var linksAvailable bool
	var sourceAvailable bool
	if err := db.Raw(`
		SELECT to_regclass(current_schema() || '.checkin_user_links') IS NOT NULL,
		       to_regclass(current_schema() || '.checkin_teacher_accounts') IS NOT NULL
	`).Row().Scan(&linksAvailable, &sourceAvailable); err != nil {
		return fmt.Errorf("locate check-in name source: %w", err)
	}
	if !linksAvailable || !sourceAvailable {
		return nil
	}

	if err := db.Exec(`
		UPDATE users AS target
		SET prefix = BTRIM(COALESCE(source.prefix, '')),
		    first_name = BTRIM(COALESCE(source.first_name, '')),
		    last_name = BTRIM(COALESCE(source.last_name, ''))
		FROM checkin_user_links AS links
		JOIN checkin_teacher_accounts AS source
		  ON source.teacher_uid = links.checkin_teacher_uid
		WHERE target.uuid = links.user_uuid
		  AND BTRIM(COALESCE(source.first_name, '')) <> ''
		  AND target.prefix = ''
		  AND target.last_name = ''
		  AND (
		      target.first_name = CONCAT_WS(
		          ' ',
		          NULLIF(BTRIM(COALESCE(source.first_name, '')), ''),
		          NULLIF(BTRIM(COALESCE(source.last_name, '')), '')
		      )
		      OR target.first_name =
		          BTRIM(COALESCE(source.prefix, '')) ||
		          BTRIM(COALESCE(source.first_name, '')) ||
		          CASE
		              WHEN BTRIM(COALESCE(source.last_name, '')) = '' THEN ''
		              ELSE ' ' || BTRIM(source.last_name)
		          END
		  )
	`).Error; err != nil {
		return fmt.Errorf("backfill imported check-in user names: %w", err)
	}
	return nil
}
