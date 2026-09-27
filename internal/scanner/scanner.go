package scanner

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
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

// Scanner discovers BLE devices via BlueZ D-Bus and emits observations.
type Scanner struct {
	adapter string
}

// New returns a Scanner for the given BlueZ adapter (e.g. "hci0").
func New(adapter string) *Scanner {
	return &Scanner{adapter: adapter}
}

// Run connects to BlueZ, starts passive LE discovery, and streams observations
// to out until ctx is cancelled. It returns nil on clean shutdown.
func (s *Scanner) Run(ctx context.Context, out chan<- Observation) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("system bus: %w", err)
	}
	defer conn.Close()

	adapterPath := dbus.ObjectPath("/org/bluez/" + s.adapter)
	adapter := conn.Object(bluezService, adapterPath)

	// Passive LE discovery, with duplicate data so RSSI updates stream continuously.
	filter := map[string]dbus.Variant{
		"Transport":     dbus.MakeVariant("le"),
		"RSSI":          dbus.MakeVariant(int16(-100)),
		"DuplicateData": dbus.MakeVariant(true),
	}
	if err := adapter.Call(adapterIface+".SetDiscoveryFilter", 0, filter).Err; err != nil {
		log.Printf("[ble-watch] SetDiscoveryFilter: %v (continuing)", err)
	}
	if err := adapter.Call(adapterIface+".StartDiscovery", 0).Err; err != nil {
		return fmt.Errorf("StartDiscovery: %w", err)
	}
	defer func() { _ = adapter.Call(adapterIface+".StopDiscovery", 0).Err }()

	if err := conn.AddMatchSignal(
		dbus.WithMatchSender(bluezService),
		dbus.WithMatchInterface(propsIface),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return fmt.Errorf("match PropertiesChanged: %w", err)
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender(bluezService),
		dbus.WithMatchInterface(objMgrIface),
	); err != nil {
		return fmt.Errorf("match ObjectManager: %w", err)
	}

	signals := make(chan *dbus.Signal, 256)
	conn.Signal(signals)

	seed(conn, out)

	cache := newCache()
	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-signals:
			if sig == nil {
				return nil
			}
			handleSignal(sig, cache, out)
		}
	}
}

// cache carries forward stable fields (name, address type, mfg) across partial
// PropertiesChanged updates that only include e.g. RSSI.
type cache struct {
	mu    sync.Mutex
	byMAC map[string]*Observation
}

func newCache() *cache { return &cache{byMAC: make(map[string]*Observation)} }

func (c *cache) merge(o *Observation) *Observation {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked(o.Ts)
	prev, ok := c.byMAC[o.Mac]
	if !ok {
		c.byMAC[o.Mac] = o
		return o
	}
	if o.Name == "" {
		o.Name = prev.Name
	}
	if o.AddressType == "" {
		o.AddressType = prev.AddressType
	}
	if o.MfgID == 0 {
		o.MfgID = prev.MfgID
	}
	if o.ServiceUUIDs == "" {
		o.ServiceUUIDs = prev.ServiceUUIDs
	}
	c.byMAC[o.Mac] = o
	return o
}

func (c *cache) pruneLocked(now int64) {
	if len(c.byMAC) <= maxCacheEntries {
		return
	}
	cutoff := now - int64(cacheMaxAge.Seconds())
	for mac, o := range c.byMAC {
		if o.Ts < cutoff {
			delete(c.byMAC, mac)
		}
	}
	// A scan can encounter more random addresses than the age window covers.
	// Keep a hard bound in that case by evicting the oldest entries.
	for len(c.byMAC) > maxCacheEntries {
		oldestMAC := ""
		var oldest int64
		for mac, o := range c.byMAC {
			if oldestMAC == "" || o.Ts < oldest {
				oldestMAC, oldest = mac, o.Ts
			}
		}
		delete(c.byMAC, oldestMAC)
	}
}

func handleSignal(sig *dbus.Signal, c *cache, out chan<- Observation) {
	switch sig.Name {
	case propsIface + ".PropertiesChanged":
		if len(sig.Body) < 2 {
			return
		}
		iface, _ := sig.Body[0].(string)
		if iface != deviceIface {
			return
		}
		changed, ok := sig.Body[1].(map[string]dbus.Variant)
		if !ok {
			return
		}
		mac := macFromPath(sig.Path)
		if mac == "" {
			return
		}
		o := obsFromProps(mac, changed)
		if o == nil {
			return
		}
		out <- *c.merge(o)

	case objMgrIface + ".InterfacesAdded":
		if len(sig.Body) < 2 {
			return
		}
		path, _ := sig.Body[0].(dbus.ObjectPath)
		ifaces, ok := sig.Body[1].(map[string]map[string]dbus.Variant)
		if !ok {
			return
		}
		props, ok := ifaces[deviceIface]
		if !ok {
			return
		}
		mac := macFromPath(path)
		if mac == "" {
			return
		}
		if o := obsFromProps(mac, props); o != nil {
			out <- *c.merge(o)
		}
	}
}

func seed(conn *dbus.Conn, out chan<- Observation) {
	obj := conn.Object(bluezService, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call(objMgrIface+".GetManagedObjects", 0).Store(&managed); err != nil {
		log.Printf("[ble-watch] GetManagedObjects: %v", err)
		return
	}
	for path, ifaces := range managed {
		props, ok := ifaces[deviceIface]
		if !ok {
			continue
		}
		mac := macFromPath(path)
		if mac == "" {
			continue
		}
		if o := obsFromProps(mac, props); o != nil {
			out <- *o
		}
	}
}

func macFromPath(p dbus.ObjectPath) string {
	s := string(p)
	idx := strings.LastIndex(s, "/")
	if idx < 0 {
		return ""
	}
	dev := s[idx+1:]
	if !strings.HasPrefix(dev, "dev_") {
		return ""
	}
	return strings.ReplaceAll(strings.TrimPrefix(dev, "dev_"), "_", ":")
}

func obsFromProps(mac string, props map[string]dbus.Variant) *Observation {
	o := &Observation{Mac: mac, Ts: time.Now().Unix()}
	if v, ok := props["RSSI"]; ok {
		o.RSSI = float64(int16Variant(v))
	}
	if v, ok := props["Alias"]; ok {
		o.Name = stringVariant(v)
	} else if v, ok := props["Name"]; ok {
		o.Name = stringVariant(v)
	}
	if v, ok := props["AddressType"]; ok {
		o.AddressType = stringVariant(v)
	}
	if v, ok := props["ManufacturerData"]; ok {
		o.MfgID = firstMfgID(v)
	}
	if v, ok := props["UUIDs"]; ok {
		o.ServiceUUIDs = uuidList(v)
	}
	return o
}

func stringVariant(v dbus.Variant) string {
	s, ok := v.Value().(string)
	if !ok {
		return ""
	}
	return s
}

func int16Variant(v dbus.Variant) int16 {
	switch x := v.Value().(type) {
	case int16:
		return x
	case int32:
		return int16(x)
	case int64:
		return int16(x)
	case uint16:
		return int16(x)
	}
	return 0
}

func firstMfgID(v dbus.Variant) int {
	m, ok := v.Value().(map[uint16]dbus.Variant)
	if !ok {
		return 0
	}
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, int(id))
	}
	if len(ids) == 0 {
		return 0
	}
	sort.Ints(ids)
	return ids[0]
}

func uuidList(v dbus.Variant) string {
	switch x := v.Value().(type) {
	case []string:
		return strings.Join(x, ",")
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	}
	return ""
}
