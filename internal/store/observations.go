package store

// Observation is one RSSI sample for a device at a point in time.
type Observation struct {
	Mac          string  `json:"mac"`
	Ts           int64   `json:"ts"`
	RSSI         float64 `json:"rssi"`
	Name         string  `json:"name"`
	AddressType  string  `json:"address_type"`
	MfgID        int     `json:"mfg_id"`
	ServiceUUIDs string  `json:"service_uuids"`
}

// InsertObservation upserts a sample (one per device per second).
func (s *Store) InsertObservation(o *Observation) error {
	_, err := s.db.Exec(`
INSERT INTO observations(mac,ts,rssi,name,address_type,mfg_id,service_uuids)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(mac,ts) DO UPDATE SET
  rssi=excluded.rssi,
  name=excluded.name,
  address_type=excluded.address_type,
  mfg_id=excluded.mfg_id,
  service_uuids=excluded.service_uuids`,
		o.Mac, o.Ts, o.RSSI, o.Name, o.AddressType, o.MfgID, o.ServiceUUIDs)
	return err
}

// Series returns a device's RSSI samples in [since, until], ascending by time.
func (s *Store) Series(mac string, since, until int64) ([]Observation, error) {
	rows, err := s.db.Query(`
SELECT mac,ts,rssi,name,address_type,mfg_id,service_uuids
FROM observations WHERE mac=? AND ts>=? AND ts<=? ORDER BY ts ASC`, mac, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Observation, 0)
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.Mac, &o.Ts, &o.RSSI, &o.Name, &o.AddressType, &o.MfgID, &o.ServiceUUIDs); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// PurgeObservations deletes samples older than beforeTs (retention).
func (s *Store) PurgeObservations(beforeTs int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM observations WHERE ts < ?`, beforeTs)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
