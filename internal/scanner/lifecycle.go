package scanner

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

type managedObjects = map[dbus.ObjectPath]map[string]map[string]dbus.Variant

type busConnection interface {
	Object(string, dbus.ObjectPath) dbus.BusObject
	AddMatchSignalContext(context.Context, ...dbus.MatchOption) error
	Signal(chan<- *dbus.Signal)
	RemoveSignal(chan<- *dbus.Signal)
	Context() context.Context
	Close() error
}

// Run requests LE discovery and streams cached seeds followed by D-Bus updates.
// It owns a private bus connection and stops only its own discovery session.
func (s *Scanner) Run(ctx context.Context, out chan<- Observation) error {
	if ctx.Err() != nil {
		s.updateHealth(func(h *Health) { h.State = "stopped" })
		return nil
	}
	s.updateHealth(func(h *Health) { *h = Health{State: "starting", StartedAt: time.Now()} })
	if !strings.HasPrefix(s.adapter, "hci") || !s.adapterPath().IsValid() || strings.Contains(s.adapter, "/") {
		return s.fail("invalid_adapter")
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return s.fail("system_bus_unavailable")
	}
	return s.run(ctx, conn, out)
}

func (s *Scanner) fail(code string) error {
	s.updateHealth(func(h *Health) { h.State, h.Error = "failed", code })
	return errors.New(code)
}

func (s *Scanner) run(parent context.Context, conn busConnection, out chan<- Observation) (result error) {
	ctx, cancel := context.WithCancel(parent)
	stopWatch := context.AfterFunc(conn.Context(), cancel)
	defer func() {
		stopWatch()
		cancel()
		if err := conn.Close(); err != nil {
			s.cleanupError("bus_close_failed")
		}
		if parent.Err() != nil {
			result = nil
			s.updateHealth(func(h *Health) { h.State = "stopped" })
		} else if result != nil {
			// Every error returned below is a fixed classification.
			s.fail(result.Error())
		}
	}()
	s.updateHealth(func(h *Health) { *h = Health{State: "starting", StartedAt: time.Now()} })

	signals := make(chan *dbus.Signal, 256)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	// Register the consumer and all matches before triggering discovery.
	matches := [][]dbus.MatchOption{
		{dbus.WithMatchSender(bluezService), dbus.WithMatchInterface(propsIface), dbus.WithMatchMember("PropertiesChanged"), dbus.WithMatchPathNamespace(s.adapterPath())},
		{dbus.WithMatchSender(bluezService), dbus.WithMatchInterface(objMgrIface)},
		{dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, bluezService)},
	}
	for _, match := range matches {
		matchCtx, done := context.WithTimeout(ctx, 10*time.Second)
		err := conn.AddMatchSignalContext(matchCtx, match...)
		done()
		if err != nil {
			return errors.New("signal_subscription_failed")
		}
	}
	filter := map[string]dbus.Variant{
		"Transport":     dbus.MakeVariant("le"),
		"RSSI":          dbus.MakeVariant(int16(-100)),
		"DuplicateData": dbus.MakeVariant(true),
	}
	if call(conn, ctx, s.adapterPath(), adapterIface+".SetDiscoveryFilter", filter).Err != nil {
		return errors.New("discovery_filter_failed")
	}
	if call(conn, ctx, s.adapterPath(), adapterIface+".StartDiscovery").Err != nil {
		return errors.New("discovery_start_failed")
	}
	defer func() {
		s.updateHealth(func(h *Health) { h.State = "stopping" })
		cleanup, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		if call(conn, cleanup, s.adapterPath(), adapterIface+".StopDiscovery").Err != nil {
			s.cleanupError("discovery_stop_failed")
		}
	}()
	var managed managedObjects
	if call(conn, ctx, "/", objMgrIface+".GetManagedObjects").Store(&managed) != nil {
		return errors.New("seed_read_failed")
	}
	adapter, ok := managed[s.adapterPath()][adapterIface]
	if !ok {
		return errors.New("adapter_missing")
	}
	s.adapterProperties(adapter, nil)
	c := newCache()
	if err := s.seed(ctx, managed, c, out); err != nil {
		return errors.New("seed_interrupted")
	}
	s.updateHealth(func(h *Health) { h.State = "running" })
	for {
		select {
		case <-ctx.Done():
			return errors.New("bus_disconnected")
		case sig, ok := <-signals:
			if !ok || sig == nil {
				return errors.New("signal_stream_closed")
			}
			if err := s.handleSignal(ctx, sig, c, out); err != nil {
				if ctx.Err() != nil {
					return errors.New("bus_disconnected")
				}
				return err
			}
		}
	}
}

func call(conn busConnection, ctx context.Context, path dbus.ObjectPath, method string, args ...interface{}) *dbus.Call {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Object(bluezService, path).CallWithContext(ctx, method, 0, args...)
}

func (s *Scanner) cleanupError(code string) {
	s.updateHealth(func(h *Health) { h.CleanupError = code })
	log.Printf("scanner cleanup: %s", code)
}

func (s *Scanner) adapterProperties(props map[string]dbus.Variant, invalidated []string) {
	s.updateHealth(func(h *Health) {
		for _, key := range invalidated {
			switch key {
			case "Powered":
				h.Powered = false
			case "Discovering":
				h.Discovering = false
			}
		}
		if value, ok := props["Powered"]; ok {
			h.Powered, _ = value.Value().(bool)
		}
		if value, ok := props["Discovering"]; ok {
			h.Discovering, _ = value.Value().(bool)
		}
	})
}
