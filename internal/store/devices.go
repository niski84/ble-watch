package store

import (
	"database/sql"
	"time"
)

// Device is a tracked BLE device.
type Device struct {
	Mac              string  `json:"mac"`
	Source           string  `json:"source"`
	Name             string  `json:"name"`
	Group            string  `json:"group"`
	AddressType      string  `json:"address_type"`
	Manufacturer     string  `json:"manufacturer"`
	FirstSeenAt      int64   `json:"first_seen_at"`
	LastSeenAt       int64   `json:"last_seen_at"`
	SeenCount        int64   `json:"seen_count"`
	LastRSSI         float64 `json:"last_rssi"`
	BaselineMean     float64 `json:"baseline_mean"`
	BaselineStd      float64 `json:"baseline_std"`
	BaselineN        int     `json:"baseline_n"`
	Recognized       bool    `json:"recognized"`
	RecognizedAt     int64   `json:"recognized_at"`
	RecognizedSource string  `json:"recognized_source"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

// UpsertDevice refreshes a device's live fields from a scan. On first sight it
// creates the row; on subsequent sights it bumps seen_count and last_seen_at.
func (s *Store) UpsertDevice(mac, name, addressType, manufacturer string, ts int64, rssi float64) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
INSERT INTO devices(mac,source,name,group_name,address_type,manufacturer,first_seen_at,last_seen_at,seen_count,last_rssi,created_at,updated_at)
VALUES(?,'ble',?,'',?,?,?,?,1,?,?,?)
ON CONFLICT(mac) DO UPDATE SET
  name=excluded.name,
  address_type=excluded.address_type,
  manufacturer=excluded.manufacturer,
  last_seen_at=excluded.last_seen_at,
  seen_count=devices.seen_count+1,
  last_rssi=excluded.last_rssi,
  updated_at=excluded.updated_at`,
		mac, name, addressType, manufacturer, ts, ts, rssi, now, now)
	return err
}

// GetDevice returns a device by MAC, or sql.ErrNoRows.
func (s *Store) GetDevice(mac string) (*Device, error) {
	row := s.db.QueryRow(`
SELECT mac,source,name,group_name,address_type,manufacturer,first_seen_at,last_seen_at,seen_count,last_rssi,
       COALESCE(baseline_mean,0), COALESCE(baseline_std,0), baseline_n,
       recognized, COALESCE(recognized_at,0), recognized_source, created_at, updated_at
FROM devices WHERE mac=?`, mac)
	return scanDevice(row)
}

// ListDevices returns all devices, most recently seen first.
func (s *Store) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(`
SELECT mac,source,name,group_name,address_type,manufacturer,first_seen_at,last_seen_at,seen_count,last_rssi,
       COALESCE(baseline_mean,0), COALESCE(baseline_std,0), baseline_n,
       recognized, COALESCE(recognized_at,0), recognized_source, created_at, updated_at
FROM devices ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

// ListRecognized returns only recognized devices.
func (s *Store) ListRecognized() ([]Device, error) {
	rows, err := s.db.Query(`
SELECT mac,source,name,group_name,address_type,manufacturer,first_seen_at,last_seen_at,seen_count,last_rssi,
       COALESCE(baseline_mean,0), COALESCE(baseline_std,0), baseline_n,
       recognized, COALESCE(recognized_at,0), recognized_source, created_at, updated_at
FROM devices WHERE recognized=1 ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

// SetRecognized marks (or unmarks) a device as recognized.
func (s *Store) SetRecognized(mac, source string, recognized bool) error {
	now := time.Now().Unix()
	var err error
	if recognized {
		_, err = s.db.Exec(`UPDATE devices SET recognized=1, recognized_source=?, recognized_at=COALESCE(recognized_at,?), updated_at=? WHERE mac=?`,
			source, now, now, mac)
	} else {
		_, err = s.db.Exec(`UPDATE devices SET recognized=0, recognized_source='', recognized_at=NULL, updated_at=? WHERE mac=?`,
			now, mac)
	}
	return err
}

// SetAlias renames a device.
func (s *Store) SetAlias(mac, name string) error {
	_, err := s.db.Exec(`UPDATE devices SET name=?, updated_at=? WHERE mac=?`, name, time.Now().Unix(), mac)
	return err
}

// SetGroup assigns a human-friendly grouping such as "Family", "My devices",
// or "Neighbors". An empty group returns the device to the ungrouped view.
func (s *Store) SetGroup(mac, group string) error {
	_, err := s.db.Exec(`UPDATE devices SET group_name=?, updated_at=? WHERE mac=?`, group, time.Now().Unix(), mac)
	return err
}

// SetBaseline persists the rolling RSSI baseline for a device.
func (s *Store) SetBaseline(mac string, mean, std float64, n int) error {
	_, err := s.db.Exec(`UPDATE devices SET baseline_mean=?, baseline_std=?, baseline_n=?, updated_at=? WHERE mac=?`,
		mean, std, n, time.Now().Unix(), mac)
	return err
}

type rowScanner interface{ Scan(dest ...any) error }

func scanDevice(rs rowScanner) (*Device, error) {
	d := &Device{}
	var recognized int
	err := rs.Scan(&d.Mac, &d.Source, &d.Name, &d.Group, &d.AddressType, &d.Manufacturer, &d.FirstSeenAt, &d.LastSeenAt,
		&d.SeenCount, &d.LastRSSI, &d.BaselineMean, &d.BaselineStd, &d.BaselineN,
		&recognized, &d.RecognizedAt, &d.RecognizedSource, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	d.Recognized = recognized != 0
	return d, nil
}

func scanDevices(rows *sql.Rows) ([]Device, error) {
	out := make([]Device, 0)
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}
