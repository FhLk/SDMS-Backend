#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/restore.sh backups/20260907-120000
# This overwrites database objects contained in the dump and restores uploads.

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <backup-directory>" >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

BACKUP_PATH="$1"
if [[ "$BACKUP_PATH" != /* ]]; then
  BACKUP_PATH="$ROOT_DIR/$BACKUP_PATH"
fi

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
UPLOAD_DIR="${UPLOAD_DIR:-uploads}"

[[ -f "$BACKUP_PATH/database.dump" ]] || { echo "database.dump not found" >&2; exit 1; }

export PGPASSWORD="${DB_PASSWORD:-}"
pg_restore \
  --host "$DB_HOST" \
  --port "$DB_PORT" \
  --username "$DB_USER" \
  --dbname "$DB_NAME" \
  --clean --if-exists --no-owner \
  "$BACKUP_PATH/database.dump"

if [[ -f "$BACKUP_PATH/uploads.tar.gz" ]]; then
  rm -rf "$ROOT_DIR/$UPLOAD_DIR"
  tar -C "$ROOT_DIR" -xzf "$BACKUP_PATH/uploads.tar.gz"
fi

echo "restore completed from: $BACKUP_PATH"
