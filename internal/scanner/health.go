package scanner

import (
	"sync"
	"time"
)

const silenceAfter = 45 * time.Second

// Health describes scanner setup and D-Bus activity, not RF coverage or writes.
type Health struct {
	State          string    `json:"state"`
	Ready          bool      `json:"ready"`
	Powered        bool      `json:"powered"`
	Discovering    bool      `json:"discovering"`
	Activity       string    `json:"activity"`
	SilenceSeconds int64     `json:"silence_after_seconds"`
	StartedAt      time.Time `json:"started_at"`
	LastLiveUpdate time.Time `json:"last_live_update"`
	Seeded         uint64    `json:"seeded"`
	LiveUpdates    uint64    `json:"live_updates"`
	DeviceSignals  uint64    `json:"device_signals"`
	Error          string    `json:"error,omitempty"`
	CleanupError   string    `json:"cleanup_error,omitempty"`
}

// Scanner discovers BLE devices via BlueZ D-Bus and emits observations.
type Scanner struct {
	adapter string
	mu      sync.Mutex
	health  Health
}

// New returns a Scanner for the given BlueZ adapter (e.g. "hci0").
func New(adapter string) *Scanner {
	return &Scanner{adapter: adapter, health: Health{State: "not_started"}}
}

// Health reports readiness for scanning separately from source silence.
// Live updates are advertising-related D-Bus updates, not proven RF packets.
func (s *Scanner) Health() Health {
	return s.healthAt(time.Now())
}

func (s *Scanner) healthAt(now time.Time) Health {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.health
	h.Ready = h.State == "running" && h.Powered && h.Discovering
	h.SilenceSeconds = int64(silenceAfter.Seconds())
	h.Activity = "unknown"
	if h.Ready {
		last := h.LastLiveUpdate
		h.Activity = "active"
		if last.IsZero() {
			last = h.StartedAt
			h.Activity = "waiting"
		}
		if now.Sub(last) >= silenceAfter {
			h.Activity = "silent"
		}
	}
	return h
}

func (s *Scanner) updateHealth(fn func(*Health)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.health)
}
