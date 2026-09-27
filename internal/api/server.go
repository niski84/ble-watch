// Package api provides the HTTP layer for ble-watch.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/config"
	"github.com/niski84/ble-watch/internal/ingest"
	"github.com/niski84/ble-watch/internal/store"
	"github.com/niski84/ble-watch/web"
)

// Server holds HTTP handlers and their dependencies.
type Server struct {
	cfg       config.Config
	store     *store.Store
	hub       *alert.Hub
	detector  *anomaly.Detector
	processor *ingest.Processor
}

// NewServer builds an http.Handler with all routes registered.
func NewServer(cfg config.Config, st *store.Store, hub *alert.Hub, det *anomaly.Detector, processor *ingest.Processor) http.Handler {
	s := &Server{cfg: cfg, store: st, hub: hub, detector: det, processor: processor}

	mux := http.NewServeMux()

	// Health reports service liveness, not scanner or task health.
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/ingestion", s.handleIngestion)

	// Devices
	mux.HandleFunc("GET /api/devices", s.handleListDevices)
	mux.HandleFunc("GET /api/devices/{mac}", s.handleGetDevice)
	mux.HandleFunc("GET /api/devices/{mac}/series", s.handleSeries)
	mux.HandleFunc("GET /api/devices/{mac}/chart", s.handleChart)
	mux.HandleFunc("POST /api/devices/{mac}/recognize", s.handleRecognize)
	mux.HandleFunc("POST /api/devices/{mac}/unrecognize", s.handleUnrecognize)
	mux.HandleFunc("POST /api/devices/{mac}/rename", s.handleRename)
	mux.HandleFunc("POST /api/devices/{mac}/group", s.handleGroup)

	// Events
	mux.HandleFunc("GET /api/events", s.handleListEvents)
	mux.HandleFunc("POST /api/events/{id}/ack", s.handleAckEvent)

	// SSE live stream
	mux.HandleFunc("GET /api/stream", s.handleStream)

	// Web UI
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /devices", s.handleDevicesPage)
	mux.HandleFunc("GET /devices/{mac}", s.handleDevicePage)
	mux.HandleFunc("GET /events", s.handleEventsPage)

	// Live fragments (SSE-triggered partial refreshes)
	mux.HandleFunc("GET /partials/devices", s.handlePartialsDevices)
	mux.HandleFunc("GET /partials/events", s.handlePartialsEvents)

	// Favicon from the embedded web/ tree.
	mux.HandleFunc("GET /favicon.svg", s.handleFavicon)

	return withLogging(mux)
}

func (s *Server) handleIngestion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.processor.Stats())
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ble-watch"})
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	b, err := web.FS.ReadFile("favicon.svg")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(b)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := s.hub.Subscribe("global")
	defer s.hub.Unsubscribe("global", ch)

	fmt.Fprintf(w, "event: connected\ndata: {\"stream\":\"global\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, strings.ReplaceAll(msg, "\n", " "))
			flusher.Flush()
		}
	}
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("respondJSON encode error: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}
