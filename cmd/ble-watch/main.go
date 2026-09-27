package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/api"
	"github.com/niski84/ble-watch/internal/config"
	"github.com/niski84/ble-watch/internal/ingest"
	"github.com/niski84/ble-watch/internal/scanner"
	"github.com/niski84/ble-watch/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DataDir + "/ble-watch.db")
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	hub := alert.NewHub()
	sink := alert.NewWebhookSink(cfg.AlertWebhookURL)

	det, err := anomaly.NewDetector(cfg, st, hub, sink)
	if err != nil {
		log.Fatalf("detector: %v", err)
	}
	// Apply retention immediately at startup, not only after the first ticker.
	purge(st, cfg.RetentionDays)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	obsCh := make(chan scanner.Observation, 256)
	processor := ingest.New(ctx, det)

	sc := scanner.New(cfg.Adapter)
	go func() {
		if err := sc.Run(ctx, obsCh); err != nil {
			log.Printf("scanner: %v", err)
		}
	}()

	// Submit observations to the task queue with one detector worker.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case o := <-obsCh:
				if err := processor.Submit(ctx, o); err != nil {
					log.Printf("observation submit: %v", err)
					hub.Broadcast("global", "observation_submit_error", map[string]string{"error": err.Error()})
				}
			}
		}
	}()

	// Housekeeping: disappearance reap + retention purge.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				det.Reap(time.Now())
				purge(st, cfg.RetentionDays)
			}
		}
	}()

	handler := api.NewServer(cfg, st, hub, det, processor)

	addr := ":" + cfg.Port
	log.Printf("[ble-watch] listening on http://localhost:%s (adapter %s)", cfg.Port, cfg.Adapter)

	go func() {
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Printf("http: %v", err)
			cancel()
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Println("[ble-watch] shutting down")
	cancel()
}

func purge(st *store.Store, retentionDays int) {
	before := time.Now().AddDate(0, 0, -retentionDays).Unix()
	if n, err := st.PurgeObservations(before); err != nil {
		log.Printf("purge: %v", err)
	} else if n > 0 {
		log.Printf("purged %d observations", n)
	}
	if n, err := st.PurgeEvents(before); err != nil {
		log.Printf("purge events: %v", err)
	} else if n > 0 {
		log.Printf("purged %d events", n)
	}
	if err := st.CapEvents(5000); err != nil {
		log.Printf("cap events: %v", err)
	}
}
