package api

import (
	"log"
	"net/http"

	"github.com/niski84/ble-watch/web"
)

func (s *Server) handlePipeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	stats := s.processor.Stats()
	data := web.PipelineData{Submitted: stats.Submitted, Processed: stats.Processed, Rejected: stats.Rejected}
	if err := s.store.DB().QueryRow("SELECT count(*) FROM observations").Scan(&data.Stored); err != nil {
		log.Printf("pipeline observation count: %v", err)
		http.Error(w, "Database status unavailable", http.StatusServiceUnavailable)
		return
	}
	data.Source = "Receiver unavailable"
	if s.cfg.DemoMode {
		data.Source = "Synthetic scenario; no radio reception"
	} else if s.scanner != nil {
		health := s.scanner.Health()
		data.Source = "Receiver: " + health.State + "; activity: " + health.Activity
	}
	if err := web.Pipeline(data).Render(r.Context(), w); err != nil {
		log.Printf("pipeline render: %v", err)
	}
}
