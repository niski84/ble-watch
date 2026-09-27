package scanner

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakeBus struct {
	dbus.BusObject
	ctx           context.Context
	steps         []string
	signals       chan<- *dbus.Signal
	managed       managedObjects
	failMethod    string
	onSeed        func()
	onStart       func()
	stopContextOK bool
}

func newFakeBus() *fakeBus {
	return &fakeBus{ctx: context.Background(), managed: managedObjects{
		"/org/bluez/hci0": {adapterIface: {"Powered": dbus.MakeVariant(true), "Discovering": dbus.MakeVariant(true)}},
		syntheticPath:     {deviceIface: {"Alias": dbus.MakeVariant("Synthetic beacon"), "RSSI": dbus.MakeVariant(int16(-60))}},
	}}
}

func (b *fakeBus) Object(string, dbus.ObjectPath) dbus.BusObject { return b }
func (b *fakeBus) Context() context.Context                      { return b.ctx }
func (b *fakeBus) Signal(ch chan<- *dbus.Signal)                 { b.steps = append(b.steps, "signal"); b.signals = ch }
func (b *fakeBus) RemoveSignal(chan<- *dbus.Signal)              { b.steps = append(b.steps, "remove_signal") }
func (b *fakeBus) Close() error                                  { b.steps = append(b.steps, "close"); return nil }
func (b *fakeBus) AddMatchSignalContext(ctx context.Context, _ ...dbus.MatchOption) error {
	b.steps = append(b.steps, "match")
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing deadline")
	}
	if b.failMethod == "match" {
		return errors.New("synthetic-private-error")
	}
	return nil
}
func (b *fakeBus) CallWithContext(ctx context.Context, method string, _ dbus.Flags, _ ...interface{}) *dbus.Call {
	method = method[strings.LastIndex(method, ".")+1:]
	b.steps = append(b.steps, method)
	if _, ok := ctx.Deadline(); !ok {
		return &dbus.Call{Err: errors.New("missing deadline")}
	}
	if method == "StopDiscovery" {
		b.stopContextOK = ctx.Err() == nil
	}
	if method == b.failMethod {
		return &dbus.Call{Err: errors.New("synthetic-private-error")}
	}
	if method == "StartDiscovery" && b.onStart != nil {
		b.onStart()
	}
	if method == "GetManagedObjects" {
		if b.onSeed != nil {
			b.onSeed()
		}
		return &dbus.Call{Body: []interface{}{b.managed}}
	}
	return &dbus.Call{}
}

func TestStartupOrderingAndQueuedSignal(t *testing.T) {
	b := newFakeBus()
	s := New("hci0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Observation, 2)
	b.onStart = func() {
		b.signals <- propertySignal(map[string]dbus.Variant{"RSSI": dbus.MakeVariant(int16(-75))})
	}
	done := make(chan error, 1)
	go func() { done <- s.run(ctx, b, out) }()
	for i := 0; i < 2; i++ {
		select {
		case o := <-out:
			if o.Name != "Synthetic beacon" || (i == 0 && o.RSSI != -60) || (i == 1 && o.RSSI != -75) {
				t.Fatalf("seed/signal inconsistency: %+v", o)
			}
		case <-time.After(time.Second):
			t.Fatal("startup update lost")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
	want := []string{"signal", "match", "match", "match", "SetDiscoveryFilter", "StartDiscovery", "GetManagedObjects", "StopDiscovery", "remove_signal", "close"}
	if !reflect.DeepEqual(b.steps, want) || !b.stopContextOK {
		t.Fatalf("lifecycle order or cleanup context: %v, %v", b.steps, b.stopContextOK)
	}
	if h := s.Health(); h.State != "stopped" || h.Ready || h.Seeded != 1 || h.LiveUpdates != 1 {
		t.Fatalf("shutdown health: %+v", h)
	}
}

func TestSetupFailuresAreSanitized(t *testing.T) {
	for method, code := range map[string]string{
		"match": "signal_subscription_failed", "SetDiscoveryFilter": "discovery_filter_failed",
		"StartDiscovery": "discovery_start_failed", "GetManagedObjects": "seed_read_failed",
	} {
		t.Run(method, func(t *testing.T) {
			b := newFakeBus()
			b.failMethod = method
			s := New("hci0")
			err := s.run(context.Background(), b, make(chan Observation, 2))
			if err == nil || err.Error() != code {
				t.Fatalf("error classification: %v", err)
			}
			if h := s.Health(); h.Ready || h.State != "failed" || h.Error != code || h.Seeded != 0 {
				t.Fatalf("failure health: %+v", h)
			}
			stopped := false
			for _, step := range b.steps {
				if step == "StopDiscovery" {
					stopped = true
				}
			}
			if stopped != (method == "GetManagedObjects") {
				t.Fatalf("incorrect discovery ownership cleanup: %v", b.steps)
			}
			if b.steps[len(b.steps)-1] != "close" {
				t.Fatal("connection not closed")
			}
		})
	}
}

func TestRunCancelsDuringSeedAndCleansUp(t *testing.T) {
	b := newFakeBus()
	s := New("hci0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Observation, 1)
	out <- Observation{}
	seeding := make(chan struct{})
	b.onSeed = func() { close(seeding) }
	b.failMethod = "StopDiscovery"
	done := make(chan error, 1)
	go func() { done <- s.run(ctx, b, out) }()
	<-seeding
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("seed prevented shutdown")
	}
	if h := s.Health(); h.State != "stopped" || h.Seeded != 0 || h.CleanupError != "discovery_stop_failed" {
		t.Fatalf("cleanup health: %+v", h)
	}
}

func TestBusDisconnectUnblocksOutput(t *testing.T) {
	b := newFakeBus()
	busCtx, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	b.ctx = busCtx
	b.onSeed = disconnect
	s := New("hci0")
	done := make(chan error, 1)
	go func() { done <- s.run(context.Background(), b, make(chan Observation)) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("disconnect treated as clean shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect did not unblock output")
	}
	if h := s.Health(); h.State != "failed" || h.Ready {
		t.Fatalf("disconnect health: %+v", h)
	}
}

func TestClosedSignalStreamIsFailure(t *testing.T) {
	b := newFakeBus()
	b.onSeed = func() { close(b.signals) }
	s := New("hci0")
	if err := s.run(context.Background(), b, make(chan Observation, 1)); err == nil || err.Error() != "signal_stream_closed" {
		t.Fatalf("stream closure: %v", err)
	}
}
