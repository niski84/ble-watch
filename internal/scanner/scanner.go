package scanner

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	bluezService    = "org.bluez"
	adapterIface    = "org.bluez.Adapter1"
	deviceIface     = "org.bluez.Device1"
	propsIface      = "org.freedesktop.DBus.Properties"
	objMgrIface     = "org.freedesktop.DBus.ObjectManager"
	maxCacheEntries = 4096
	cacheMaxAge     = 15 * time.Minute
)

type cachedDevice struct {
	props map[string]dbus.Variant
	ts    int64
}

// The run loop owns the cache. Seeds and signals share partial metadata.
type cache struct {
	byMAC map[string]cachedDevice
}

func newCache() *cache { return &cache{byMAC: make(map[string]cachedDevice)} }

func (c *cache) merge(mac string, props map[string]dbus.Variant, invalidated []string) *Observation {
	now := time.Now().Unix()
	prev := c.byMAC[mac]
	if prev.props == nil {
		if len(c.byMAC) >= maxCacheEntries {
			oldestMAC := ""
			var oldest int64
			for key, entry := range c.byMAC {
				if entry.ts < now-int64(cacheMaxAge.Seconds()) {
					delete(c.byMAC, key)
					continue
				}
				if oldestMAC == "" || entry.ts < oldest {
					oldestMAC, oldest = key, entry.ts
				}
			}
			if len(c.byMAC) >= maxCacheEntries {
				delete(c.byMAC, oldestMAC)
			}
		}
		prev.props = make(map[string]dbus.Variant)
	}
	for _, key := range invalidated {
		delete(prev.props, key)
	}
	for _, key := range []string{"Alias", "Name", "AddressType", "RSSI", "ManufacturerData", "UUIDs"} {
		if value, ok := props[key]; ok {
			prev.props[key] = value
		}
	}
	prev.ts = now
	c.byMAC[mac] = prev
	return obsFromProps(mac, prev.props)
}

func (s *Scanner) handleSignal(ctx context.Context, sig *dbus.Signal, c *cache, out chan<- Observation) error {
	var path dbus.ObjectPath
	var props map[string]dbus.Variant
	var invalidated []string
	switch sig.Name {
	case "org.freedesktop.DBus.NameOwnerChanged":
		if len(sig.Body) == 3 && sig.Body[0] == bluezService && sig.Body[1] != "" {
			return errors.New("bluez_owner_changed")
		}
		return nil
	case propsIface + ".PropertiesChanged":
		if len(sig.Body) < 2 {
			return nil
		}
		iface, _ := sig.Body[0].(string)
		props, _ = sig.Body[1].(map[string]dbus.Variant)
		if len(sig.Body) > 2 {
			invalidated, _ = sig.Body[2].([]string)
		}
		if sig.Path == s.adapterPath() && iface == adapterIface {
			s.adapterProperties(props, invalidated)
			return nil
		}
		if iface != deviceIface {
			return nil
		}
		path = sig.Path
	case objMgrIface + ".InterfacesAdded":
		if len(sig.Body) < 2 {
			return nil
		}
		path, _ = sig.Body[0].(dbus.ObjectPath)
		ifaces, _ := sig.Body[1].(map[string]map[string]dbus.Variant)
		props = ifaces[deviceIface]
	case objMgrIface + ".InterfacesRemoved":
		if len(sig.Body) < 2 {
			return nil
		}
		path, _ = sig.Body[0].(dbus.ObjectPath)
		ifaces, _ := sig.Body[1].([]string)
		for _, iface := range ifaces {
			if path == s.adapterPath() && iface == adapterIface {
				return errors.New("adapter_removed")
			}
			if iface == deviceIface {
				delete(c.byMAC, s.deviceMAC(path))
			}
		}
		return nil
	default:
		return nil
	}
	mac := s.deviceMAC(path)
	if mac == "" || props == nil {
		return nil
	}
	s.updateHealth(func(h *Health) { h.DeviceSignals++ })
	o := c.merge(mac, props, invalidated)
	// Metadata-only changes do not constitute a fresh sighting.
	if !advertisingUpdate(props) {
		return nil
	}
	return s.emit(ctx, out, *o, false)
}

func advertisingUpdate(props map[string]dbus.Variant) bool {
	if v, ok := props["RSSI"]; ok {
		if _, ok := v.Value().(int16); ok {
			return true
		}
	}
	for _, key := range []string{"ManufacturerData", "ServiceData"} {
		if v, ok := props[key]; ok {
			switch value := v.Value().(type) {
			case map[uint16]dbus.Variant:
				if key == "ManufacturerData" && len(value) > 0 {
					return true
				}
			case map[string]dbus.Variant:
				if key == "ServiceData" && len(value) > 0 {
					return true
				}
			}
		}
	}
	return false
}

func (s *Scanner) seed(ctx context.Context, managed managedObjects, c *cache, out chan<- Observation) error {
	for path, ifaces := range managed {
		props, ok := ifaces[deviceIface]
		mac := s.deviceMAC(path)
		if !ok || mac == "" {
			continue
		}
		if err := s.emit(ctx, out, *c.merge(mac, props, nil), true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) emit(ctx context.Context, out chan<- Observation, o Observation, seeded bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- o:
		s.updateHealth(func(h *Health) {
			if seeded {
				h.Seeded++
			} else {
				h.LiveUpdates++
				h.LastLiveUpdate = time.Now()
			}
		})
		return nil
	}
}

func (s *Scanner) adapterPath() dbus.ObjectPath {
	return dbus.ObjectPath("/org/bluez/" + s.adapter)
}

func (s *Scanner) deviceMAC(path dbus.ObjectPath) string {
	prefix := string(s.adapterPath()) + "/dev_"
	if !strings.HasPrefix(string(path), prefix) {
		return ""
	}
	address := strings.TrimPrefix(string(path), prefix)
	if len(address) != 17 {
		return ""
	}
	for i, char := range address {
		if i%3 == 2 {
			if char != '_' {
				return ""
			}
		} else if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return ""
		}
	}
	return strings.ToUpper(strings.ReplaceAll(address, "_", ":"))
}

func obsFromProps(mac string, props map[string]dbus.Variant) *Observation {
	o := &Observation{Mac: mac, Ts: time.Now().Unix()}
	if v, ok := props["RSSI"].Value().(int16); ok {
		o.RSSI = float64(v)
	}
	if v, ok := props["Alias"]; ok {
		o.Name, _ = v.Value().(string)
	} else {
		o.Name, _ = props["Name"].Value().(string)
	}
	o.AddressType, _ = props["AddressType"].Value().(string)
	if m, ok := props["ManufacturerData"].Value().(map[uint16]dbus.Variant); ok {
		ids := make([]int, 0, len(m))
		for id := range m {
			ids = append(ids, int(id))
		}
		sort.Ints(ids)
		if len(ids) > 0 {
			o.MfgID = ids[0]
		}
	}
	if uuids, ok := props["UUIDs"].Value().([]string); ok {
		o.ServiceUUIDs = strings.Join(uuids, ",")
	}
	return o
}
