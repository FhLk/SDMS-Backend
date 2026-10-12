package migration

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// legacyUUIDTable describes the stable legacy ordering used while assigning
// UUIDv7 values. The ordering keeps the backfilled keys monotonic and makes a
// migration repeatable to inspect even though UUID values themselves are new.
type legacyUUIDTable struct {
	name    string
	orderBy string
}

var legacyUUIDTables = []legacyUUIDTable{
	{name: "users", orderBy: "id"},
	{name: "divisions", orderBy: "code"},
	{name: "user_divisions", orderBy: "user_id, division_code"},
	{name: "academic_years", orderBy: "year"},
	{name: "drive_folders", orderBy: "path"},
	{name: "topics", orderBy: "id"},
	{name: "topic_fields", orderBy: "id"},
	{name: "topic_assignees", orderBy: "topic_id, user_id"},
	{name: "submissions", orderBy: "id"},
	{name: "files", orderBy: "id"},
	{name: "sessions", orderBy: "token_hash"},
}

// migrateLegacyPrimaryKeys upgrades the BE-02 schema in place. PostgreSQL DDL
// is transactional here, so a failure restores every old key and relationship.
func migrateLegacyPrimaryKeys(db *gorm.DB) error {
	if !db.Migrator().HasTable("users") {
		return nil
	}

	hasLegacyID := db.Migrator().HasColumn("users", "id")
	hasUUID := db.Migrator().HasColumn("users", "uuid")
	switch {
	case hasLegacyID && !hasUUID:
		// This is the complete legacy schema expected below.
	case !hasLegacyID && hasUUID:
		return nil
	default:
		return fmt.Errorf("migrate UUID primary keys: users table is in an unsupported partial state (id=%t, uuid=%t)", hasLegacyID, hasUUID)
	}

	for _, table := range legacyUUIDTables {
		if !db.Migrator().HasTable(table.name) {
			return fmt.Errorf("migrate UUID primary keys: required legacy table %q is missing", table.name)
		}
	}

	if err := executeMigrationStatements(db, addUUIDColumnStatements); err != nil {
		return err
	}
	for _, table := range legacyUUIDTables {
		if err := backfillUUIDv7(db, table); err != nil {
			return err
		}
	}
	if err := executeMigrationStatements(db, backfillForeignKeyStatements); err != nil {
		return err
	}
	if err := migrateSubmissionValueKeys(db); err != nil {
		return err
	}
	if err := executeMigrationStatements(db, requireUUIDStatements); err != nil {
		return err
	}
	if err := executeMigrationStatements(db, dropLegacyConstraintStatements); err != nil {
		return err
	}
	if err := executeMigrationStatements(db, dropLegacyColumnStatements); err != nil {
		return err
	}
	if err := executeMigrationStatements(db, addUUIDPrimaryKeyStatements); err != nil {
		return err
	}

	return nil
}

func executeMigrationStatements(db *gorm.DB, statements []string) error {
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("migrate UUID primary keys: %w", err)
		}
	}
	return nil
}

func backfillUUIDv7(db *gorm.DB, table legacyUUIDTable) error {
	query := fmt.Sprintf(`SELECT ctid::text FROM %s WHERE uuid IS NULL ORDER BY %s`, table.name, table.orderBy)
	rows, err := db.Raw(query).Rows()
	if err != nil {
		return fmt.Errorf("read legacy %s rows: %w", table.name, err)
	}

	var rowIDs []string
	for rows.Next() {
		var rowID string
		if err := rows.Scan(&rowID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read legacy %s row identifier: %w", table.name, err)
		}
		rowIDs = append(rowIDs, rowID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate legacy %s rows: %w", table.name, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy %s rows: %w", table.name, err)
	}

	update := fmt.Sprintf(`UPDATE %s SET uuid = ? WHERE ctid = ?::tid`, table.name)
	for _, rowID := range rowIDs {
		generated, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate UUIDv7 for %s: %w", table.name, err)
		}
		if err := db.Exec(update, generated, rowID).Error; err != nil {
			return fmt.Errorf("backfill UUIDv7 for %s: %w", table.name, err)
		}
	}
	return nil
}

// migrateSubmissionValueKeys changes JSON object keys from legacy numeric
// topic_fields.id values to the new topic_fields.uuid values. Unknown keys are
// retained verbatim so the migration never discards an existing answer.
func migrateSubmissionValueKeys(db *gorm.DB) error {
	const statement = `
		UPDATE submissions AS target
		SET values = migrated.values
		FROM (
			SELECT submission.id,
				COALESCE(
					jsonb_object_agg(COALESCE(field.uuid::text, entry.key), entry.value)
						FILTER (WHERE entry.key IS NOT NULL),
					'{}'::jsonb
				) AS values
			FROM submissions AS submission
			LEFT JOIN LATERAL jsonb_each(submission.values) AS entry ON true
			LEFT JOIN topic_fields AS field ON field.id::text = entry.key
			GROUP BY submission.id
		) AS migrated
		WHERE target.id = migrated.id`
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("migrate submission value UUID keys: %w", err)
	}
	return nil
}

var addUUIDColumnStatements = []string{
	`ALTER TABLE users ADD COLUMN uuid uuid`,
	`ALTER TABLE divisions ADD COLUMN uuid uuid, ADD COLUMN head_user_uuid uuid`,
	`ALTER TABLE user_divisions ADD COLUMN uuid uuid, ADD COLUMN user_uuid uuid, ADD COLUMN division_uuid uuid`,
	`ALTER TABLE academic_years ADD COLUMN uuid uuid`,
	`ALTER TABLE drive_folders ADD COLUMN uuid uuid`,
	`ALTER TABLE topics ADD COLUMN uuid uuid, ADD COLUMN division_uuid uuid, ADD COLUMN academic_year_uuid uuid, ADD COLUMN created_by_uuid uuid`,
	`ALTER TABLE topic_fields ADD COLUMN uuid uuid, ADD COLUMN topic_uuid uuid`,
	`ALTER TABLE topic_assignees ADD COLUMN uuid uuid, ADD COLUMN topic_uuid uuid, ADD COLUMN user_uuid uuid`,
	`ALTER TABLE submissions ADD COLUMN uuid uuid, ADD COLUMN topic_uuid uuid, ADD COLUMN user_uuid uuid`,
	`ALTER TABLE files ADD COLUMN uuid uuid, ADD COLUMN topic_uuid uuid, ADD COLUMN field_uuid uuid, ADD COLUMN submission_uuid uuid, ADD COLUMN uploaded_by_uuid uuid`,
	`ALTER TABLE sessions ADD COLUMN uuid uuid, ADD COLUMN user_uuid uuid`,
}

var backfillForeignKeyStatements = []string{
	`UPDATE divisions AS target SET head_user_uuid = parent.uuid FROM users AS parent WHERE target.head_user_id = parent.id`,
	`UPDATE user_divisions AS target SET user_uuid = parent.uuid FROM users AS parent WHERE target.user_id = parent.id`,
	`UPDATE user_divisions AS target SET division_uuid = parent.uuid FROM divisions AS parent WHERE target.division_code = parent.code`,
	`UPDATE topics AS target SET division_uuid = parent.uuid FROM divisions AS parent WHERE target.division_code = parent.code`,
	`UPDATE topics AS target SET academic_year_uuid = parent.uuid FROM academic_years AS parent WHERE target.year = parent.year`,
	`UPDATE topics AS target SET created_by_uuid = parent.uuid FROM users AS parent WHERE target.created_by_id = parent.id`,
	`UPDATE topic_fields AS target SET topic_uuid = parent.uuid FROM topics AS parent WHERE target.topic_id = parent.id`,
	`UPDATE topic_assignees AS target SET topic_uuid = parent.uuid FROM topics AS parent WHERE target.topic_id = parent.id`,
	`UPDATE topic_assignees AS target SET user_uuid = parent.uuid FROM users AS parent WHERE target.user_id = parent.id`,
	`UPDATE submissions AS target SET topic_uuid = parent.uuid FROM topics AS parent WHERE target.topic_id = parent.id`,
	`UPDATE submissions AS target SET user_uuid = parent.uuid FROM users AS parent WHERE target.user_id = parent.id`,
	`UPDATE files AS target SET topic_uuid = parent.uuid FROM topics AS parent WHERE target.topic_id = parent.id`,
	`UPDATE files AS target SET field_uuid = parent.uuid FROM topic_fields AS parent WHERE target.field_id = parent.id`,
	`UPDATE files AS target SET submission_uuid = parent.uuid FROM submissions AS parent WHERE target.submission_id = parent.id`,
	`UPDATE files AS target SET uploaded_by_uuid = parent.uuid FROM users AS parent WHERE target.uploaded_by_id = parent.id`,
	`UPDATE sessions AS target SET user_uuid = parent.uuid FROM users AS parent WHERE target.user_id = parent.id`,
}

var requireUUIDStatements = []string{
	`ALTER TABLE users ALTER COLUMN uuid SET NOT NULL`,
	`ALTER TABLE divisions ALTER COLUMN uuid SET NOT NULL`,
	`ALTER TABLE user_divisions ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN user_uuid SET NOT NULL, ALTER COLUMN division_uuid SET NOT NULL`,
	`ALTER TABLE academic_years ALTER COLUMN uuid SET NOT NULL`,
	`ALTER TABLE drive_folders ALTER COLUMN uuid SET NOT NULL`,
	`ALTER TABLE topics ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN division_uuid SET NOT NULL, ALTER COLUMN academic_year_uuid SET NOT NULL, ALTER COLUMN created_by_uuid SET NOT NULL`,
	`ALTER TABLE topic_fields ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN topic_uuid SET NOT NULL`,
	`ALTER TABLE topic_assignees ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN topic_uuid SET NOT NULL, ALTER COLUMN user_uuid SET NOT NULL`,
	`ALTER TABLE submissions ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN topic_uuid SET NOT NULL, ALTER COLUMN user_uuid SET NOT NULL`,
	`ALTER TABLE files ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN topic_uuid SET NOT NULL, ALTER COLUMN field_uuid SET NOT NULL, ALTER COLUMN uploaded_by_uuid SET NOT NULL`,
	`ALTER TABLE sessions ALTER COLUMN uuid SET NOT NULL, ALTER COLUMN user_uuid SET NOT NULL`,
}

var dropLegacyConstraintStatements = []string{
	`ALTER TABLE divisions DROP CONSTRAINT IF EXISTS fk_divisions_head_user`,
	`ALTER TABLE user_divisions DROP CONSTRAINT IF EXISTS fk_user_divisions_division, DROP CONSTRAINT IF EXISTS fk_user_divisions_user`,
	`ALTER TABLE topics DROP CONSTRAINT IF EXISTS fk_academic_years_topics, DROP CONSTRAINT IF EXISTS fk_topics_created_by, DROP CONSTRAINT IF EXISTS fk_topics_division`,
	`ALTER TABLE topic_fields DROP CONSTRAINT IF EXISTS fk_topics_fields`,
	`ALTER TABLE topic_assignees DROP CONSTRAINT IF EXISTS fk_topic_assignees_user, DROP CONSTRAINT IF EXISTS fk_topics_assignees`,
	`ALTER TABLE submissions DROP CONSTRAINT IF EXISTS fk_submissions_topic, DROP CONSTRAINT IF EXISTS fk_submissions_user`,
	`ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_files_field, DROP CONSTRAINT IF EXISTS fk_files_topic, DROP CONSTRAINT IF EXISTS fk_files_uploaded_by, DROP CONSTRAINT IF EXISTS fk_submissions_files`,
	`ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_user`,
	`ALTER TABLE users DROP CONSTRAINT users_pkey`,
	`ALTER TABLE divisions DROP CONSTRAINT divisions_pkey`,
	`ALTER TABLE user_divisions DROP CONSTRAINT user_divisions_pkey`,
	`ALTER TABLE academic_years DROP CONSTRAINT academic_years_pkey`,
	`ALTER TABLE drive_folders DROP CONSTRAINT drive_folders_pkey`,
	`ALTER TABLE topics DROP CONSTRAINT topics_pkey`,
	`ALTER TABLE topic_fields DROP CONSTRAINT topic_fields_pkey`,
	`ALTER TABLE topic_assignees DROP CONSTRAINT topic_assignees_pkey`,
	`ALTER TABLE submissions DROP CONSTRAINT submissions_pkey`,
	`ALTER TABLE files DROP CONSTRAINT files_pkey`,
	`ALTER TABLE sessions DROP CONSTRAINT sessions_pkey`,
}

var dropLegacyColumnStatements = []string{
	`ALTER TABLE users DROP COLUMN id`,
	`ALTER TABLE divisions DROP COLUMN head_user_id`,
	`ALTER TABLE user_divisions DROP COLUMN user_id, DROP COLUMN division_code`,
	`ALTER TABLE topics DROP COLUMN id, DROP COLUMN division_code, DROP COLUMN year, DROP COLUMN created_by_id`,
	`ALTER TABLE topic_fields DROP COLUMN id, DROP COLUMN topic_id`,
	`ALTER TABLE topic_assignees DROP COLUMN topic_id, DROP COLUMN user_id`,
	`ALTER TABLE submissions DROP COLUMN id, DROP COLUMN topic_id, DROP COLUMN user_id`,
	`ALTER TABLE files DROP COLUMN id, DROP COLUMN topic_id, DROP COLUMN field_id, DROP COLUMN submission_id, DROP COLUMN uploaded_by_id`,
	`ALTER TABLE sessions DROP COLUMN user_id`,
}

var addUUIDPrimaryKeyStatements = []string{
	`ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE divisions ADD CONSTRAINT divisions_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE user_divisions ADD CONSTRAINT user_divisions_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE academic_years ADD CONSTRAINT academic_years_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE drive_folders ADD CONSTRAINT drive_folders_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE topics ADD CONSTRAINT topics_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE topic_fields ADD CONSTRAINT topic_fields_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE topic_assignees ADD CONSTRAINT topic_assignees_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE submissions ADD CONSTRAINT submissions_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE files ADD CONSTRAINT files_pkey PRIMARY KEY (uuid)`,
	`ALTER TABLE sessions ADD CONSTRAINT sessions_pkey PRIMARY KEY (uuid)`,
}
