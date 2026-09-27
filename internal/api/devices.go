package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/niski84/ble-watch/web"
)

// handleListDevices returns all tracked devices as JSON.
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devs, err := s.store.ListDevices()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	now := time.Now().Unix()
	out := make([]web.DeviceCard, len(devs))
	for i, d := range devs {
		out[i] = deviceToCard(d, now, s.cfg.GoneAfter)
	}
	respondJSON(w, http.StatusOK, out)
}

// handleGetDevice returns a single device as JSON.
func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	dev, err := s.store.GetDevice(mac)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load device")
		return
	}
	respondJSON(w, http.StatusOK, deviceToCard(*dev, time.Now().Unix(), s.cfg.GoneAfter))
}

// handleRecognize marks a device as recognized.
func (s *Server) handleRecognize(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	if err := s.detector.Recognize(mac, "manual"); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to recognize device")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "mac": mac, "recognized": "true"})
}

// handleUnrecognize removes a device from the recognized set.
func (s *Server) handleUnrecognize(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	if err := s.detector.Unrecognize(mac); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to unrecognize device")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "mac": mac, "recognized": "false"})
}

// handleRename updates a device's alias.
func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := s.detector.Rename(mac, body.Name); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to rename device")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "mac": mac, "name": body.Name})
}

// handleGroup assigns a human-friendly group used by the focused device view.
func (s *Server) handleGroup(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	var body struct {
		Group string `json:"group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.Group = strings.TrimSpace(body.Group)
	if len(body.Group) > 80 {
		respondError(w, http.StatusBadRequest, "group is too long")
		return
	}
	if err := s.store.SetGroup(mac, body.Group); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update group")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "mac": mac, "group": body.Group})
}
