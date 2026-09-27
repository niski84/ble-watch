package scanner

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestBlueZSignalProducesObservation(t *testing.T) {
	const mac = "02:00:00:00:00:01"
	out := make(chan Observation, 2)
	c := newCache()
	path := dbus.ObjectPath("/org/bluez/hci0/dev_02_00_00_00_00_01")
	handleSignal(&dbus.Signal{
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
	handleSignal(&dbus.Signal{
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
