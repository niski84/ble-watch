package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/config"
)

func TestStreamPreservesEventFrames(t *testing.T) {
	hub := alert.NewHub()
	server := httptest.NewServer(NewServer(config.Config{}, nil, hub, nil, nil))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	readFrame := func() string {
		t.Helper()
		var frame strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("incomplete SSE frame: %v", err)
			}
			frame.WriteString(line)
			if line == "\n" {
				return frame.String()
			}
		}
	}
	if got := readFrame(); got != "event: connected\ndata: {\"stream\":\"global\"}\n\n" {
		t.Fatalf("connection frame: %q", got)
	}
	for _, kind := range []string{"device_seen", "rssi_anomaly"} {
		hub.Broadcast("global", kind, map[string]string{"name": "Synthetic\nsensor"})
		want := "event: " + kind + "\ndata: {\"name\":\"Synthetic\\nsensor\"}\n\n"
		if got := readFrame(); got != want {
			t.Fatalf("event framing: %q", got)
		}
	}
}
