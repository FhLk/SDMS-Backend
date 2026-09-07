#!/usr/bin/env bash
set -euo pipefail

# SDMS Phase-1 backup: PostgreSQL + local evidence files.
# Requires pg_dump and the same DB_* variables used by the API.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"

UPLOAD_DIR="${UPLOAD_DIR:-uploads}"
BACKUP_DIR="${BACKUP_DIR:-backups}"
STAMP="$(date +%Y%m%d-%H%M%S)"
TARGET="$ROOT_DIR/$BACKUP_DIR/$STAMP"
mkdir -p "$TARGET"

export PGPASSWORD="${DB_PASSWORD:-}"
pg_dump \
  --host "$DB_HOST" \
  --port "$DB_PORT" \
  --username "$DB_USER" \
  --format=custom \
  --file "$TARGET/database.dump" \
  "$DB_NAME"

if [[ -d "$ROOT_DIR/$UPLOAD_DIR" ]]; then
  tar -C "$ROOT_DIR" -czf "$TARGET/uploads.tar.gz" "$UPLOAD_DIR"
else
  echo "warning: upload directory '$ROOT_DIR/$UPLOAD_DIR' does not exist; database backup only" >&2
fi

cat > "$TARGET/manifest.txt" <<MANIFEST
created_at=$(date -Iseconds)
database=$DB_NAME
upload_dir=$UPLOAD_DIR
MANIFEST

echo "backup created: $TARGET"
