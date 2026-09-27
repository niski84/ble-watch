// Package store is the SQLite persistence layer for ble-watch.
package store

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database.
type Store struct{ db *sql.DB }

// DB exposes the underlying *sql.DB for packages that need raw access.
func (s *Store) DB() *sql.DB { return s.db }

// Open opens (and migrates) the SQLite database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS devices(
  mac               TEXT PRIMARY KEY,
  source            TEXT NOT NULL DEFAULT 'ble',
  name              TEXT NOT NULL DEFAULT '',
  group_name        TEXT NOT NULL DEFAULT '',
  address_type      TEXT NOT NULL DEFAULT '',
  manufacturer      TEXT NOT NULL DEFAULT '',
  first_seen_at     INTEGER NOT NULL,
  last_seen_at      INTEGER NOT NULL,
  seen_count        INTEGER NOT NULL DEFAULT 0,
  last_rssi         REAL NOT NULL DEFAULT 0,
  baseline_mean     REAL,
  baseline_std      REAL,
  baseline_n        INTEGER NOT NULL DEFAULT 0,
  recognized        INTEGER NOT NULL DEFAULT 0,
  recognized_at     INTEGER,
  recognized_source TEXT NOT NULL DEFAULT '',
  created_at        INTEGER NOT NULL,
  updated_at        INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS observations(
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  mac           TEXT NOT NULL,
  ts            INTEGER NOT NULL,
  rssi          REAL NOT NULL,
  name          TEXT NOT NULL DEFAULT '',
  address_type  TEXT NOT NULL DEFAULT '',
  mfg_id        INTEGER NOT NULL DEFAULT 0,
  service_uuids TEXT NOT NULL DEFAULT '',
  UNIQUE(mac, ts)
);
CREATE INDEX IF NOT EXISTS idx_obs_mac_ts ON observations(mac, ts);

CREATE TABLE IF NOT EXISTS events(
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  ts           INTEGER NOT NULL,
  kind         TEXT NOT NULL,
  mac          TEXT NOT NULL,
  name         TEXT NOT NULL DEFAULT '',
  rssi         REAL,
  severity     TEXT NOT NULL DEFAULT 'info',
  details      TEXT NOT NULL DEFAULT '',
  acknowledged INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
`)
	if err != nil {
		return err
	}
	// Older databases predate source/group metadata. Keep migrations additive
	// so existing device history survives upgrades.
	for _, stmt := range []string{
		`ALTER TABLE devices ADD COLUMN source TEXT NOT NULL DEFAULT 'ble'`,
		`ALTER TABLE devices ADD COLUMN group_name TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}
