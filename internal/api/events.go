package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/niski84/ble-watch/internal/store"
	"github.com/niski84/ble-watch/web"
)

// handleListEvents returns recent events as JSON.
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}
	events, err := s.store.ListEvents(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	out := make([]web.EventCard, len(events))
	for i, e := range events {
		out[i] = eventToCard(e)
	}
	respondJSON(w, http.StatusOK, out)
}

// handleAckEvent marks an event acknowledged.
func (s *Server) handleAckEvent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid event id")
		return
	}
	if err := s.store.AcknowledgeEvent(id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to acknowledge")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": strconv.FormatInt(id, 10)})
}

// eventToCard converts a store event to a view card.
func eventToCard(e store.Event) web.EventCard {
	return web.EventCard{
		ID:           e.ID,
		Ts:           e.Ts,
		Time:         time.Unix(e.Ts, 0).Format("2006-01-02 15:04:05"),
		Kind:         e.Kind,
		Mac:          e.Mac,
		Name:         e.Name,
		RSSI:         e.RSSI,
		Severity:     e.Severity,
		Details:      e.Details,
		Acknowledged: e.Acknowledged,
	}
}
