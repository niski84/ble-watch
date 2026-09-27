#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${BLE_WATCH_DB_PATH:-$PROJECT_DIR/data/ble-watch.db}"
BACKUP_DIR="${BLE_WATCH_BACKUP_DIR:-$PROJECT_DIR/../ble-watch-backups}"
KEEP_COUNT="${BLE_WATCH_BACKUP_KEEP:-14}"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required" >&2
  exit 1
fi
if [[ ! -f "$DB_PATH" ]]; then
  echo "database not found: $DB_PATH" >&2
  exit 1
fi
if ! [[ "$KEEP_COUNT" =~ ^[0-9]+$ ]] || (( KEEP_COUNT < 1 )); then
  echo "BLE_WATCH_BACKUP_KEEP must be a positive integer" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"
timestamp="$(date +%Y%m%d-%H%M%S)"
backup_path="$BACKUP_DIR/ble-watch-$timestamp.db"

# SQLite's online backup is consistent while the service continues writing.
sqlite3 "$DB_PATH" ".timeout 10000" ".backup '$backup_path'"
sqlite3 "$backup_path" "PRAGMA integrity_check;" | grep -qx 'ok'

# Keep the newest snapshots and remove only older backups made by this script.
mapfile -t backups < <(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'ble-watch-*.db' -printf '%T@ %p\n' | sort -nr | awk '{sub(/^[^ ]+ /, ""); print}')
if (( ${#backups[@]} > KEEP_COUNT )); then
  for old_backup in "${backups[@]:KEEP_COUNT}"; do
    rm -f -- "$old_backup"
  done
fi

echo "Created verified backup: $backup_path"
du -h "$backup_path" | awk '{print "Backup size: " $1}'
