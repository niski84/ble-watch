#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${BLE_WATCH_DB_PATH:-$PROJECT_DIR/data/ble-watch.db}"

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 /path/to/ble-watch-YYYYMMDD-HHMMSS.db" >&2
  exit 2
fi
SOURCE_DB="$1"

if [[ ! -f "$SOURCE_DB" ]]; then
  echo "backup not found: $SOURCE_DB" >&2
  exit 1
fi
if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required" >&2
  exit 1
fi
if [[ -e "$DB_PATH" ]]; then
  echo "refusing to overwrite existing database: $DB_PATH" >&2
  echo "Stop BLE Watch, move the current database aside, then retry." >&2
  exit 1
fi

sqlite3 "$SOURCE_DB" "PRAGMA integrity_check;" | grep -qx 'ok'
mkdir -p "$(dirname "$DB_PATH")"
sqlite3 "$SOURCE_DB" ".backup '$DB_PATH'"
echo "Restored verified database to: $DB_PATH"
