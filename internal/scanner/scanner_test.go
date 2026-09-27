package scanner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestBlueZSignalProducesObservation(t *testing.T) {
	const mac = "02:00:00:00:00:01"
	out := make(chan Observation, 2)
	c := newCache()
	s := New("hci0")
	path := dbus.ObjectPath("/org/bluez/hci0/dev_02_00_00_00_00_01")
	s.handleSignal(context.Background(), &dbus.Signal{
		Name: objMgrIface + ".InterfacesAdded",
		Body: []any{path, map[string]map[string]dbus.Variant{
			deviceIface: {
				"Alias":       dbus.MakeVariant("Synthetic beacon"),
				"RSSI":        dbus.MakeVariant(int16(-60)),
				"AddressType": dbus.MakeVariant("random"),
			},
		}},
	}, c, out)
	if len(out) != 1 {
		t.Fatal("device-added signal did not produce an observation")
	}
	first := <-out
	if first.Mac != mac || first.Name != "Synthetic beacon" || first.RSSI != -60 || first.Ts == 0 {
		t.Fatalf("unexpected decoded observation: %+v", first)
	}
	s.handleSignal(context.Background(), &dbus.Signal{
		Name: propsIface + ".PropertiesChanged", Path: path,
		Body: []any{deviceIface, map[string]dbus.Variant{"RSSI": dbus.MakeVariant(int16(-70))}},
	}, c, out)
	if len(out) != 1 {
		t.Fatal("property signal did not produce an observation")
	}
	updated := <-out
	if updated.RSSI != -70 || updated.Name != first.Name || updated.AddressType != first.AddressType {
		t.Fatalf("partial update did not preserve device metadata: %+v", updated)
	}
}

const syntheticPath = dbus.ObjectPath("/org/bluez/hci0/dev_02_00_00_00_00_01")

func propertySignal(props map[string]dbus.Variant, invalidated ...string) *dbus.Signal {
	return &dbus.Signal{Name: propsIface + ".PropertiesChanged", Path: syntheticPath,
		Body: []any{deviceIface, props, invalidated}}
}

func TestSeedSharesMetadataWithoutClaimingLiveActivity(t *testing.T) {
	s := New("hci0")
	c := newCache()
	out := make(chan Observation, 4)
	props := map[string]dbus.Variant{
		"Alias": dbus.MakeVariant("Synthetic beacon"), "AddressType": dbus.MakeVariant("random"),
		"RSSI": dbus.MakeVariant(int16(-65)), "UUIDs": dbus.MakeVariant([]string{"synthetic-service"}),
		"ManufacturerData": dbus.MakeVariant(map[uint16]dbus.Variant{42: dbus.MakeVariant([]byte{1})}),
	}
	if err := s.seed(context.Background(), managedObjects{syntheticPath: {deviceIface: props}}, c, out); err != nil {
		t.Fatal(err)
	}
	<-out
	if h := s.Health(); h.Seeded != 1 || h.LiveUpdates != 0 || !h.LastLiveUpdate.IsZero() {
		t.Fatalf("seed reported live activity: %+v", h)
	}
	s.handleSignal(context.Background(), propertySignal(map[string]dbus.Variant{"RSSI": dbus.MakeVariant(int16(-70))}), c, out)
	o := <-out
	if o.Name != "Synthetic beacon" || o.AddressType != "random" || o.MfgID != 42 || o.ServiceUUIDs != "synthetic-service" || o.RSSI != -70 {
		t.Fatalf("seed metadata lost: %+v", o)
	}
	// Alias updates are cached but are not sightings.
	s.handleSignal(context.Background(), propertySignal(map[string]dbus.Variant{"Alias": dbus.MakeVariant("Updated synthetic beacon")}), c, out)
	if len(out) != 0 {
		t.Fatal("metadata update emitted a sighting")
	}
	s.handleSignal(context.Background(), propertySignal(map[string]dbus.Variant{
		"ServiceData": dbus.MakeVariant(map[string]dbus.Variant{"synthetic-service": dbus.MakeVariant([]byte{2})}),
	}, "ManufacturerData", "UUIDs"), c, out)
	o = <-out
	if o.RSSI != -70 || o.Name != "Updated synthetic beacon" || o.MfgID != 0 || o.ServiceUUIDs != "" {
		t.Fatalf("partial data or invalidation lost: %+v", o)
	}
	if h := s.Health(); h.Seeded != 1 || h.LiveUpdates != 2 || h.DeviceSignals != 3 || h.LastLiveUpdate.IsZero() {
		t.Fatalf("incorrect counters: %+v", h)
	}
}

func TestAdapterFilteringAndMalformedSignals(t *testing.T) {
	s := New("hci0")
	c := newCache()
	out := make(chan Observation, 8)
	props := map[string]dbus.Variant{"RSSI": dbus.MakeVariant(int16(-70))}
	for _, path := range []dbus.ObjectPath{
		"/org/bluez/hci1/dev_02_00_00_00_00_01", "/org/bluez/hci00/dev_02_00_00_00_00_01",
		"/org/bluez/hci0/dev_02_00_00_00_00_01/service0001", "/org/bluez/hci0/dev_invalid",
	} {
		s.seed(context.Background(), managedObjects{path: {deviceIface: props}}, c, out)
		sig := propertySignal(props)
		sig.Path = path
		s.handleSignal(context.Background(), sig, c, out)
		s.handleSignal(context.Background(), &dbus.Signal{Name: objMgrIface + ".InterfacesAdded",
			Body: []any{path, map[string]map[string]dbus.Variant{deviceIface: props}}}, c, out)
	}
	for _, sig := range []*dbus.Signal{
		{}, {Name: propsIface + ".PropertiesChanged"},
		{Name: propsIface + ".PropertiesChanged", Path: syntheticPath, Body: []any{deviceIface, "invalid"}},
		{Name: objMgrIface + ".InterfacesAdded", Body: []any{syntheticPath, "invalid"}},
		propertySignal(map[string]dbus.Variant{"RSSI": dbus.MakeVariant("invalid")}),
		propertySignal(map[string]dbus.Variant{"Connected": dbus.MakeVariant(true)}),
	} {
		if err := s.handleSignal(context.Background(), sig, c, out); err != nil {
			t.Fatal(err)
		}
	}
	if len(out) != 0 || s.Health().LiveUpdates != 0 || s.Health().Seeded != 0 {
		t.Fatal("foreign or malformed input emitted observations")
	}
}

func TestCancellationWithFullOutput(t *testing.T) {
	for _, seeded := range []bool{false, true} {
		t.Run(fmt.Sprint(seeded), func(t *testing.T) {
			s := New("hci0")
			out := make(chan Observation, 1)
			out <- Observation{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			props := map[string]dbus.Variant{"RSSI": dbus.MakeVariant(int16(-70))}
			go func() {
				if seeded {
					done <- s.seed(ctx, managedObjects{syntheticPath: {deviceIface: props}}, newCache(), out)
				} else {
					done <- s.handleSignal(ctx, propertySignal(props), newCache(), out)
				}
			}()
			select {
			case err := <-done:
				t.Fatalf("full output returned before cancellation: %v", err)
			case <-time.After(10 * time.Millisecond):
			}
			cancel()
			select {
			case err := <-done:
				if err != context.Canceled {
					t.Fatalf("cancellation: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("output blocked shutdown")
			}
			if h := s.Health(); h.Seeded != 0 || h.LiveUpdates != 0 {
				t.Fatal("unsent observations counted")
			}
		})
	}
}

func TestHealthDistinguishesSilenceAndFailure(t *testing.T) {
	s := New("hci0")
	start := time.Now()
	s.updateHealth(func(h *Health) {
		*h = Health{State: "running", Powered: true, Discovering: true, StartedAt: start, Seeded: 5}
	})
	if h := s.healthAt(start); !h.Ready || h.Activity != "waiting" {
		t.Fatalf("initial activity: %+v", h)
	}
	if h := s.healthAt(start.Add(silenceAfter)); !h.Ready || h.Activity != "silent" || h.Error != "" {
		t.Fatalf("source silence: %+v", h)
	}
	s.updateHealth(func(h *Health) { h.LastLiveUpdate = start.Add(silenceAfter); h.LiveUpdates = 1 })
	if h := s.healthAt(start.Add(silenceAfter)); h.Activity != "active" {
		t.Fatalf("live update: %+v", h)
	}
	s.fail("discovery_start_failed")
	if h := s.Health(); h.Ready || h.State != "failed" || h.Activity != "unknown" || h.Error != "discovery_start_failed" {
		t.Fatalf("failure: %+v", h)
	}
}

func TestAdapterAndRemovalSignals(t *testing.T) {
	s := New("hci0")
	s.updateHealth(func(h *Health) { h.State = "running" })
	c := newCache()
	out := make(chan Observation, 2)
	s.handleSignal(context.Background(), &dbus.Signal{Name: propsIface + ".PropertiesChanged", Path: s.adapterPath(),
		Body: []any{adapterIface, map[string]dbus.Variant{"Powered": dbus.MakeVariant(true), "Discovering": dbus.MakeVariant(true)}}}, c, out)
	if !s.Health().Ready {
		t.Fatal("adapter state not reflected")
	}
	s.handleSignal(context.Background(), &dbus.Signal{Name: propsIface + ".PropertiesChanged", Path: s.adapterPath(),
		Body: []any{adapterIface, map[string]dbus.Variant{}, []string{"Discovering"}}}, c, out)
	if s.Health().Ready {
		t.Fatal("invalidated discovery still ready")
	}
	c.merge(s.deviceMAC(syntheticPath), map[string]dbus.Variant{"Alias": dbus.MakeVariant("Synthetic")}, nil)
	s.handleSignal(context.Background(), &dbus.Signal{Name: objMgrIface + ".InterfacesRemoved", Body: []any{syntheticPath, []string{deviceIface}}}, c, out)
	if len(c.byMAC) != 0 {
		t.Fatal("removed device retained")
	}
	for _, sig := range []*dbus.Signal{
		{Name: objMgrIface + ".InterfacesRemoved", Body: []any{s.adapterPath(), []string{adapterIface}}},
		{Name: "org.freedesktop.DBus.NameOwnerChanged", Body: []any{bluezService, ":1.2", ""}},
	} {
		if s.handleSignal(context.Background(), sig, c, out) == nil {
			t.Fatal("source loss not reported")
		}
	}
}

func TestCacheHasHardBound(t *testing.T) {
	c := newCache()
	for i := 0; i < maxCacheEntries+2; i++ {
		c.merge(fmt.Sprint(i), map[string]dbus.Variant{}, nil)
		if len(c.byMAC) > maxCacheEntries {
			t.Fatal("cache exceeded bound")
		}
	}
}
