package store

// Event is a persisted detection/alert record.
type Event struct {
	ID           int64   `json:"id"`
	Ts           int64   `json:"ts"`
	Kind         string  `json:"kind"`
	Mac          string  `json:"mac"`
	Name         string  `json:"name"`
	RSSI         float64 `json:"rssi"`
	Severity     string  `json:"severity"`
	Details      string  `json:"details"`
	Acknowledged bool    `json:"acknowledged"`
}

// InsertEvent persists an event, returning its assigned ID.
func (s *Store) InsertEvent(e *Event) (int64, error) {
	res, err := s.db.Exec(`
INSERT INTO events(ts,kind,mac,name,rssi,severity,details,acknowledged)
VALUES(?,?,?,?,?,?,?,?)`,
		e.Ts, e.Kind, e.Mac, e.Name, nullFloat(e.RSSI), e.Severity, e.Details, boolInt(e.Acknowledged))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListEvents returns the most recent events, newest first.
func (s *Store) ListEvents(limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`
SELECT id,ts,kind,mac,name,COALESCE(rssi,0),severity,details,acknowledged
FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Event, 0)
	for rows.Next() {
		var e Event
		var ack int
		if err := rows.Scan(&e.ID, &e.Ts, &e.Kind, &e.Mac, &e.Name, &e.RSSI, &e.Severity, &e.Details, &ack); err != nil {
			return nil, err
		}
		e.Acknowledged = ack != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// AcknowledgeEvent marks an event acknowledged.
func (s *Store) AcknowledgeEvent(id int64) error {
	_, err := s.db.Exec(`UPDATE events SET acknowledged=1 WHERE id=?`, id)
	return err
}

// CapEvents keeps only the newest `limit` events.
func (s *Store) CapEvents(limit int) error {
	_, err := s.db.Exec(`
DELETE FROM events WHERE id NOT IN (SELECT id FROM events ORDER BY id DESC LIMIT ?)`, limit)
	return err
}

// PurgeEvents removes events older than beforeTs. The count cap remains as a
// second guard for unusually noisy detection periods.
func (s *Store) PurgeEvents(beforeTs int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM events WHERE ts < ?`, beforeTs)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func nullFloat(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
