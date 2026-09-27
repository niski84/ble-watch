// Package alert provides SSE pub/sub and optional outbound alert sinks.
package alert

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Hub is a pub/sub for SSE event streams.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan string]struct{}
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[string]map[chan string]struct{})}
}

// Subscribe registers a subscriber channel for a stream.
func (h *Hub) Subscribe(stream string) chan string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan string, 64)
	if h.subscribers[stream] == nil {
		h.subscribers[stream] = make(map[chan string]struct{})
	}
	h.subscribers[stream][ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber channel.
func (h *Hub) Unsubscribe(stream string, ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers[stream], ch)
}

// Broadcast sends a named event with a JSON payload to a stream.
func (h *Hub) Broadcast(stream, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[stream] {
		select {
		case ch <- fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(b)):
		default:
			// drop if a subscriber is slow
		}
	}
}
