package api

import (
	"net/http"
	"strconv"
	"time"
)

// handleSeries returns a device's RSSI samples over a time window.
// Query: ?hours=N (default 24, capped at 168 = 7 days).
func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	hours := 24
	if v := r.URL.Query().Get("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	if hours > 168 {
		hours = 168
	}
	until := time.Now().Unix()
	since := until - int64(hours*3600)
	series, err := s.store.Series(mac, since, until)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load series")
		return
	}
	respondJSON(w, http.StatusOK, series)
}
