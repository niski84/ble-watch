package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/niski84/ble-watch/internal/config"
	"github.com/niski84/ble-watch/internal/scanner"
)

func TestScannerHealthAndLiveness(t *testing.T) {
	sc := scanner.New("invalid/private-synthetic-value")
	if err := sc.Run(context.Background(), nil); err == nil {
		t.Fatal("invalid adapter accepted")
	}
	for name, source := range map[string]*scanner.Scanner{"missing": nil, "not_started": scanner.New("hci0"), "failed": sc} {
		t.Run(name, func(t *testing.T) {
			handler := NewServer(config.Config{}, nil, nil, nil, nil, source)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/scanner", nil))
			if resp.Code != http.StatusServiceUnavailable {
				t.Fatalf("scanner status: %d", resp.Code)
			}
			var health scanner.Health
			if err := json.Unmarshal(resp.Body.Bytes(), &health); err != nil {
				t.Fatal(err)
			}
			if health.Ready || health.Activity != "unknown" {
				t.Fatalf("false readiness: %+v", health)
			}
			if name == "failed" && health.Error != "invalid_adapter" {
				t.Fatalf("missing failure classification: %+v", health)
			}
			if strings.Contains(resp.Body.String(), "private-synthetic-value") {
				t.Fatal("private value leaked")
			}
			resp = httptest.NewRecorder()
			handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/health", nil))
			if resp.Code != http.StatusOK || strings.TrimSpace(resp.Body.String()) != `{"service":"ble-watch","status":"ok"}` {
				t.Fatalf("liveness changed: %d %s", resp.Code, resp.Body.String())
			}
		})
	}
}
