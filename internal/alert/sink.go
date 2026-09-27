package alert

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// WebhookSink forwards alert payloads to an external HTTP endpoint (e.g. nag-deck).
type WebhookSink struct {
	URL    string
	client *http.Client
	queue  chan []byte
}

// NewWebhookSink returns a sink, or nil if url is empty.
func NewWebhookSink(url string) *WebhookSink {
	if url == "" {
		return nil
	}
	s := &WebhookSink{URL: url, client: &http.Client{Timeout: 10 * time.Second}, queue: make(chan []byte, 128)}
	go s.run()
	return s
}

// Send posts a JSON payload asynchronously. Safe to call on a nil sink.
func (s *WebhookSink) Send(payload any) {
	if s == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case s.queue <- b:
	default:
		log.Printf("[ble-watch] webhook queue full; dropping alert")
	}
}

func (s *WebhookSink) run() {
	for b := range s.queue {
		resp, err := s.client.Post(s.URL, "application/json", bytes.NewReader(b))
		if err != nil {
			log.Printf("[ble-watch] webhook: %v", err)
			continue
		}
		_ = resp.Body.Close()
	}
}
