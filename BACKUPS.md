# BLE Watch database backups

The SQLite database is runtime state and is intentionally excluded from Git.
Use the online backup script while BLE Watch is running:

```sh
./scripts/backup-db.sh
```

Backups are written to `../ble-watch-backups` by default. Override that path
with `BLE_WATCH_BACKUP_DIR`, and control the number retained with
`BLE_WATCH_BACKUP_KEEP` (default: 14).

Each snapshot is integrity-checked before it is reported successful. To restore
one, stop BLE Watch, move the current `data/ble-watch.db` out of the way, and
run:

```sh
./scripts/restore-db.sh /path/to/ble-watch-YYYYMMDD-HHMMSS.db
```

For protection against losing this computer, point `BLE_WATCH_BACKUP_DIR` at
an external drive or a directory synchronized to another machine.
