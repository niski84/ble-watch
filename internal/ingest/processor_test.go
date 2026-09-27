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

func TestObservationPersistsThroughTaskQueue(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Config{SeedFile: filepath.Join(dir, "no-seeds.json"), ObservationInterval: time.Second}
	det, err := anomaly.NewDetector(cfg, st, alert.NewHub(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := New(ctx, det)
	obs := scanner.Observation{Mac: "02:00:00:00:00:01", Name: "Synthetic beacon", RSSI: -62, AddressType: "random", Ts: 1700000000}
	if err := p.Submit(ctx, obs); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for p.Stats().Processed != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := p.Stats(); got.Processed != 1 || got.Submitted != 1 || got.Rejected != 0 {
		t.Fatalf("unexpected task counters: %+v", got)
	}
	samples, err := st.Series(obs.Mac, obs.Ts, obs.Ts)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].RSSI != obs.RSSI || samples[0].Name != obs.Name {
		t.Fatalf("observation not persisted correctly: %+v", samples)
	}
	device, err := st.GetDevice(obs.Mac)
	if err != nil || device == nil || device.LastSeenAt != obs.Ts {
		t.Fatalf("device state not persisted: device=%+v err=%v", device, err)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if err := p.Submit(cancelled, obs); !errors.Is(err, executor.ErrItemSubmitterContext) {
		t.Fatalf("expected caller cancellation, got %v", err)
	}
	if p.Stats().Rejected != 1 || p.Stats().Submitted != 1 {
		t.Fatalf("rejected submission counted as accepted: %+v", p.Stats())
	}
}
