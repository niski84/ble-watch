package demo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/ingest"
	"github.com/niski84/ble-watch/internal/store"
)

func TestScenarioPersistsEventsThroughWitchcraft(t *testing.T) {
	if testing.Short() {
		t.Skip("real-timer demo scenario")
	}
	t.Setenv("DATA_DIR", "/invalid/private-data")
	t.Setenv("SEED_FILE", "/invalid/private-seeds")
	t.Setenv("ALERT_WEBHOOK_URL", "http://invalid.invalid")
	dir := t.TempDir()
	cfg := Config(dir)
	if cfg.DataDir != dir || cfg.AlertWebhookURL != "" || !cfg.DemoMode || cfg.SeedFile != filepath.Join(dir, "no-seeds.json") {
		t.Fatal("demo inherited live configuration")
	}
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	det, err := anomaly.NewDetector(cfg, st, alert.NewHub(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	p := ingest.New(ctx, det)
	if err := Run(ctx, p, det, st, func(string) {}); err != nil {
		t.Fatal(err)
	}
	stats := p.Stats()
	if stats.Submitted != 9 || stats.Processed != 9 || stats.Rejected != 0 {
		t.Fatalf("unexpected queue counters: %+v", stats)
	}
	samples, err := st.Series(MAC, 0, time.Now().Unix()+1)
	if err != nil || len(samples) != 9 {
		t.Fatalf("samples=%d err=%v", len(samples), err)
	}
	events, err := st.ListEvents(20)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"appeared", "disappeared", "rssi_anomaly", "appeared"}
	if len(events) != len(want) {
		t.Fatalf("events=%d want=%d", len(events), len(want))
	}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Fatalf("event %d: got %s want %s", i, events[i].Kind, kind)
		}
	}
	devices, err := st.ListRecognized()
	if err != nil || len(devices) != 1 || devices[0].Group != "Demo sensors" {
		t.Fatalf("recognized-device projection failed: count=%d err=%v", len(devices), err)
	}
}

func TestCancelledScenarioDoesNotSubmit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("wait cancellation: %v", err)
	}
}
