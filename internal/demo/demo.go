// Package demo runs a synthetic scenario through the production task processor.
package demo

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/config"
	"github.com/niski84/ble-watch/internal/ingest"
	"github.com/niski84/ble-watch/internal/scanner"
	"github.com/niski84/ble-watch/internal/store"
)

const MAC = "02:00:00:00:00:01"

// Config deliberately ignores environment settings and real seed files.
func Config(dir string) config.Config {
	return config.Config{
		DemoMode: true, DataDir: dir, SeedFile: filepath.Join(dir, "no-seeds.json"),
		MinSamples: 6, ZThreshold: 3, MinDeltaDBM: 20,
		GoneAfter: 4 * time.Second, ObservationInterval: time.Second,
	}
}

// Run leaves the completed scenario available for inspection. It does not
// emulate RF reception or guarantee speed-independent event timestamps.
func Run(ctx context.Context, p *ingest.Processor, det *anomaly.Detector, st *store.Store, report func(string)) error {
	report("Building the signal baseline")
	for i, rssi := range []float64{-72, -71, -73, -72, -71, -73, -72} {
		if err := submit(ctx, p, rssi, 0); err != nil {
			return err
		}
		if i == 0 {
			if err := det.Recognize(MAC, "synthetic-demo"); err != nil {
				return err
			}
			if err := st.SetGroup(MAC, "Demo sensors"); err != nil {
				return err
			}
		}
		if err := wait(ctx, time.Second); err != nil {
			return err
		}
	}
	report("Signal surge queued with a two-second delay")
	if err := submit(ctx, p, -38, 2*time.Second); err != nil {
		return err
	}
	report("Sensor silent; waiting for the presence timeout")
	if err := wait(ctx, 6*time.Second); err != nil {
		return err
	}
	det.Reap(time.Now())
	report("Disappearance recorded; sensor returns in three seconds")
	if err := wait(ctx, 3*time.Second); err != nil {
		return err
	}
	if err := submit(ctx, p, -71, 0); err != nil {
		return err
	}
	report("Scenario complete; inspect the signal history and events")
	return nil
}

func submit(ctx context.Context, p *ingest.Processor, rssi float64, delay time.Duration) error {
	before := p.Stats().Processed
	o := scanner.Observation{Mac: MAC, Name: "Synthetic workshop sensor", RSSI: rssi,
		AddressType: "random", Ts: time.Now().Add(delay).Unix()}
	var err error
	if delay > 0 {
		err = p.SubmitAfter(ctx, o, delay)
	} else {
		err = p.Submit(ctx, o)
	}
	if err != nil {
		return err
	}
	deadline := time.NewTimer(delay + 5*time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("demo observation processing timed out")
		case <-tick.C:
			if p.Stats().Processed > before {
				return nil
			}
		}
	}
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// KeepAlive maintains a visible, healthy synthetic device after the scenario.
func KeepAlive(ctx context.Context, p *ingest.Processor) error {
	for {
		if err := wait(ctx, 2*time.Second); err != nil {
			return err
		}
		if err := submit(ctx, p, -72, 0); err != nil {
			return err
		}
	}
}
