// ble-demo serves an isolated synthetic BLE Watch instance on loopback only.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/api"
	"github.com/niski84/ble-watch/internal/demo"
	"github.com/niski84/ble-watch/internal/ingest"
	"github.com/niski84/ble-watch/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	port := flag.Int("port", 0, "loopback port (0 selects an available port)")
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return err
	}
	defer listener.Close()
	dir, err := os.MkdirTemp("", "ble-watch-demo-")
	if err != nil {
		return err
	}
	// Keep this synthetic-only database available after exit for inspection.
	log.Printf("Synthetic database: %s", filepath.Join(dir, "ble-watch.db"))
	st, err := store.Open(filepath.Join(dir, "ble-watch.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	cfg := demo.Config(dir)
	hub := alert.NewHub()
	det, err := anomaly.NewDetector(cfg, st, hub, nil)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	p := ingest.New(ctx, det)
	server := &http.Server{Handler: api.NewServer(cfg, st, hub, det, p), ReadHeaderTimeout: 5 * time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()
	defer server.Close()
	log.Printf("Skitchcraft / BLE Watch demo: http://%s", listener.Addr())
	log.Print("Synthetic input only. Bluetooth and webhooks are disabled. Scenario starts in five seconds.")
	scenarioDone := make(chan error, 1)
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			scenarioDone <- ctx.Err()
		case <-timer.C:
			if err := demo.Run(ctx, p, det, st, func(phase string) { log.Print(phase) }); err != nil {
				scenarioDone <- err
				return
			}
			log.Print("Sensor continues baseline readings until the demo is stopped.")
			scenarioDone <- demo.KeepAlive(ctx, p)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			if scenarioDone != nil {
				<-scenarioDone
			}
			return nil
		case err := <-serverDone:
			stop()
			if scenarioDone != nil {
				<-scenarioDone
			}
			return err
		case err := <-scenarioDone:
			scenarioDone = nil
			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("demo: %w", err)
			}
		}
	}
}
