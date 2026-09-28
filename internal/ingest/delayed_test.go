package ingest

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/config"
	"github.com/niski84/ble-watch/internal/scanner"
	"github.com/niski84/ble-watch/internal/store"
	"github.com/palantir/witchcraft-go-tasks/executor"
)

func TestDelayedObservationAndCancelledSubmission(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	det, err := anomaly.NewDetector(config.Config{SeedFile: filepath.Join(dir, "none.json")}, st, alert.NewHub(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := New(ctx, det)
	o := scanner.Observation{Mac: "02:00:00:00:00:01", RSSI: -65, Ts: 1700000000}
	start := time.Now()
	if err := p.SubmitAfter(ctx, o, 150*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for p.Stats().Processed == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if p.Stats().Processed != 1 || time.Since(start) < 150*time.Millisecond {
		t.Fatal("delayed observation was early or never completed")
	}
	series, err := st.Series(o.Mac, o.Ts, o.Ts)
	if err != nil || len(series) != 1 {
		t.Fatalf("delayed persistence: count=%d err=%v", len(series), err)
	}
	caller, stop := context.WithCancel(ctx)
	stop()
	if err := p.SubmitAfter(caller, o, time.Hour); !errors.Is(err, executor.ErrItemSubmitterContext) {
		t.Fatalf("cancelled delay: %v", err)
	}
	if got := p.Stats(); got.Submitted != 1 || got.Rejected != 1 {
		t.Fatalf("delayed counters: %+v", got)
	}
}
