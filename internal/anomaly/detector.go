// Package anomaly consumes BLE observations and raises presence, RSSI-surge,
// and rogue-device events.
package anomaly

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/niski84/ble-watch/internal/alert"
	"github.com/niski84/ble-watch/internal/config"
	"github.com/niski84/ble-watch/internal/scanner"
	"github.com/niski84/ble-watch/internal/store"
)

// Detector maintains per-device state and emits typed events via the hub/store.
type Detector struct {
	cfg  config.Config
	st   *store.Store
	hub  *alert.Hub
	sink *alert.WebhookSink

	mu              sync.Mutex
	devices         map[string]*devState
	known           map[string]bool   // recognized MACs
	names           map[string]string // lowercased recognized name -> MAC
	cooldown        map[string]time.Time
	lastLive        map[string]int64 // throttle live device_seen to 1/sec/device
	lastObservation map[string]int64 // throttle persisted samples; detection remains full-rate
	rotating        []rotationEntry
}

type devState struct {
	mac          string
	name         string
	addressType  string
	manufacturer string
	lastSeenAt   int64
	firstSeenAt  int64
	recognized   bool
	baseline     baseline
	hours        map[int64]bool // distinct hour buckets (auto-learn)
}

type rotationEntry struct {
	mac string
	ts  int64
}

type seedDevice struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
}

// NewDetector loads the seed list + previously recognized devices and returns a Detector.
func NewDetector(cfg config.Config, st *store.Store, hub *alert.Hub, sink *alert.WebhookSink) (*Detector, error) {
	d := &Detector{
		cfg:             cfg,
		st:              st,
		hub:             hub,
		sink:            sink,
		devices:         make(map[string]*devState),
		known:           make(map[string]bool),
		names:           make(map[string]string),
		cooldown:        make(map[string]time.Time),
		lastLive:        make(map[string]int64),
		lastObservation: make(map[string]int64),
	}

	seeds, err := loadSeed(cfg.SeedFile)
	if err != nil {
		return nil, err
	}
	for _, s := range seeds {
		mac := normalizeMAC(s.MAC)
		if mac == "" {
			continue
		}
		d.known[mac] = true
		if s.Name != "" {
			d.names[strings.ToLower(s.Name)] = mac
		}
		if err := st.SetRecognized(mac, "seed", true); err != nil {
			log.Printf("[ble-watch] seed recognize %s: %v", mac, err)
		}
	}

	if devs, err := st.ListRecognized(); err == nil {
		for _, dev := range devs {
			d.known[dev.Mac] = true
			if dev.Name != "" {
				d.names[strings.ToLower(dev.Name)] = dev.Mac
			}
		}
	}
	return d, nil
}

// Process handles one observation.
func (d *Detector) Process(obs scanner.Observation) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := obs.Ts
	if now == 0 {
		now = time.Now().Unix()
	}

	recognized := d.known[obs.Mac]
	st, exists := d.devices[obs.Mac]
	if !exists {
		st = &devState{mac: obs.Mac, firstSeenAt: now, hours: map[int64]bool{}}
		d.devices[obs.Mac] = st

		// Recognized in store but not yet in memory (e.g. set while absent).
		if !recognized {
			if dev, err := d.st.GetDevice(obs.Mac); err == nil && dev.Recognized {
				recognized = true
				d.known[obs.Mac] = true
				if dev.Name != "" {
					d.names[strings.ToLower(dev.Name)] = obs.Mac
				}
			}
		}
		st.recognized = recognized

		sev := "info"
		if !recognized {
			sev = "warn"
		}
		d.emit("appeared", obs.Mac, obs.Name, obs.RSSI, sev, map[string]any{"new": !recognized})
	} else if recognized {
		st.recognized = true
	}

	st.lastSeenAt = now
	if obs.Name != "" {
		st.name = obs.Name
	}
	if obs.AddressType != "" {
		st.addressType = obs.AddressType
	}
	if mfg := scanner.CompanyName(obs.MfgID); mfg != "" {
		st.manufacturer = mfg
	}

	name := st.name
	if name == "" {
		name = obs.Name
	}

	// Auto-learn: track distinct hour buckets over the window.
	hour := now / 3600
	st.hours[hour] = true
	d.pruneHours(st, now)

	// Persist the live device on every scan, but sample observations at a lower
	// rate. Detection still processes every scan; the interval only controls the
	// historical RSSI series written to SQLite.
	if err := d.st.UpsertDevice(obs.Mac, name, st.addressType, st.manufacturer, now, obs.RSSI); err != nil {
		log.Printf("[ble-watch] upsert device: %v", err)
	}
	interval := int64(d.cfg.ObservationInterval / time.Second)
	if interval < 1 {
		interval = 1
	}
	if last, ok := d.lastObservation[obs.Mac]; !ok || now-last >= interval {
		if err := d.st.InsertObservation(&store.Observation{
			Mac: obs.Mac, Ts: now, RSSI: obs.RSSI, Name: name,
			AddressType: st.addressType, MfgID: obs.MfgID, ServiceUUIDs: obs.ServiceUUIDs,
		}); err != nil {
			log.Printf("[ble-watch] insert observation: %v", err)
		} else {
			d.lastObservation[obs.Mac] = now
		}
	}

	// RSSI surge detection (recognized devices only, once baseline is stable).
	if st.recognized && obs.RSSI < 0 {
		if st.baseline.n >= d.cfg.MinSamples {
			z := st.baseline.z(obs.RSSI)
			delta := obs.RSSI - st.baseline.mean
			if z >= d.cfg.ZThreshold && delta >= d.cfg.MinDeltaDBM {
				if d.cool("rssi_anomaly:" + obs.Mac) {
					d.emit("rssi_anomaly", obs.Mac, st.name, obs.RSSI, "critical", map[string]any{
						"z": z, "delta_dbm": delta,
						"baseline_mean": st.baseline.mean, "baseline_std": st.baseline.std(),
					})
				}
			}
		}
		st.baseline.add(obs.RSSI)
		_ = d.st.SetBaseline(obs.Mac, st.baseline.mean, st.baseline.std(), st.baseline.n)
	}

	// Auto-learn promotion.
	if !st.recognized && d.cfg.AutoLearnHours > 0 && len(st.hours) >= d.cfg.AutoLearnHours {
		d.recognize(obs.Mac, st, "learned")
	}

	// Rogue: a device advertising a recognized device's name on a different MAC.
	if !st.recognized && st.name != "" {
		if knownMAC, ok := d.names[strings.ToLower(st.name)]; ok && knownMAC != obs.Mac {
			if d.cool("rogue_device:" + obs.Mac) {
				d.emit("rogue_device", obs.Mac, st.name, obs.RSSI, "critical", map[string]any{
					"spoofed_name": st.name, "matches_mac": knownMAC,
				})
			}
		}
	}

	// Rogue: burst of distinct unknown random-MAC advertisers.
	if !st.recognized && st.addressType == "random" {
		d.rotating = append(d.rotating, rotationEntry{mac: obs.Mac, ts: now})
		cutoff := now - int64(d.cfg.RotationWindow.Seconds())
		kept := d.rotating[:0]
		distinct := map[string]bool{}
		for _, e := range d.rotating {
			if e.ts >= cutoff {
				kept = append(kept, e)
				distinct[e.mac] = true
			}
		}
		d.rotating = kept
		if d.cfg.RotationN > 0 && len(distinct) >= d.cfg.RotationN && d.cool("mac_rotation") {
			d.emit("mac_rotation", "", "", 0, "warn", map[string]any{
				"distinct_macs": len(distinct), "window_s": int(d.cfg.RotationWindow.Seconds()),
			})
		}
	}

	// Live feed (throttled to 1/sec/device).
	if now > d.lastLive[obs.Mac] {
		d.lastLive[obs.Mac] = now
		d.hub.Broadcast("global", "device_seen", map[string]any{
			"mac": obs.Mac, "name": st.name, "rssi": obs.RSSI,
			"address_type": st.addressType, "recognized": st.recognized,
		})
	}
}

// Reap emits "disappeared" for devices unseen longer than GoneAfter.
func (d *Detector) Reap(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cutoff := now.Unix() - int64(d.cfg.GoneAfter.Seconds())
	for mac, st := range d.devices {
		if st.lastSeenAt < cutoff {
			d.emit("disappeared", mac, st.name, 0, "info", nil)
			delete(d.devices, mac)
			delete(d.lastLive, mac)
		}
	}
	// Cooldowns are only useful for one minute. Without pruning, every unique
	// transient MAC can leave a permanent string key behind.
	for key, at := range d.cooldown {
		if now.Sub(at) >= time.Minute {
			delete(d.cooldown, key)
		}
	}
}

// Recognize marks a device as recognized (manual or seed).
func (d *Detector) Recognize(mac, source string) error {
	mac = normalizeMAC(mac)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.st.SetRecognized(mac, source, true); err != nil {
		return err
	}
	d.known[mac] = true
	if st, ok := d.devices[mac]; ok {
		st.recognized = true
		if st.name != "" {
			d.names[strings.ToLower(st.name)] = mac
		}
	} else if dev, err := d.st.GetDevice(mac); err == nil && dev.Name != "" {
		d.names[strings.ToLower(dev.Name)] = mac
	}
	return nil
}

// Unrecognize removes a device from the recognized set.
func (d *Detector) Unrecognize(mac string) error {
	mac = normalizeMAC(mac)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.st.SetRecognized(mac, "", false); err != nil {
		return err
	}
	delete(d.known, mac)
	if st, ok := d.devices[mac]; ok {
		st.recognized = false
	}
	return nil
}

// Rename updates a device's alias and the spoof name map.
func (d *Detector) Rename(mac, name string) error {
	mac = normalizeMAC(mac)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.st.SetAlias(mac, name); err != nil {
		return err
	}
	if st, ok := d.devices[mac]; ok {
		st.name = name
	}
	if d.known[mac] && name != "" {
		d.names[strings.ToLower(name)] = mac
	}
	return nil
}

func (d *Detector) recognize(mac string, st *devState, source string) {
	st.recognized = true
	d.known[mac] = true
	if st.name != "" {
		d.names[strings.ToLower(st.name)] = mac
	}
	if err := d.st.SetRecognized(mac, source, true); err != nil {
		log.Printf("[ble-watch] auto-learn recognize %s: %v", mac, err)
		return
	}
	d.hub.Broadcast("global", "device_recognized", map[string]any{"mac": mac, "name": st.name, "source": source})
}

func (d *Detector) emit(kind, mac, name string, rssi float64, severity string, details map[string]any) {
	detailJSON := ""
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailJSON = string(b)
		}
	}
	ev := &store.Event{Ts: time.Now().Unix(), Kind: kind, Mac: mac, Name: name, RSSI: rssi, Severity: severity, Details: detailJSON}
	id, err := d.st.InsertEvent(ev)
	if err != nil {
		log.Printf("[ble-watch] insert event: %v", err)
		return
	}
	ev.ID = id
	payload := map[string]any{
		"id": id, "ts": ev.Ts, "kind": kind, "mac": mac, "name": name,
		"rssi": rssi, "severity": severity, "details": detailJSON,
	}
	d.hub.Broadcast("global", kind, payload)
	d.sink.Send(payload)
}

func (d *Detector) cool(key string) bool {
	now := time.Now()
	if t, ok := d.cooldown[key]; ok && now.Sub(t) < 60*time.Second {
		return false
	}
	d.cooldown[key] = now
	return true
}

func (d *Detector) pruneHours(st *devState, now int64) {
	cutoffHour := (now - int64(d.cfg.AutoLearnWindow.Seconds())) / 3600
	for h := range st.hours {
		if h < cutoffHour {
			delete(st.hours, h)
		}
	}
}

func loadSeed(path string) ([]seedDevice, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var seeds []seedDevice
	if err := json.Unmarshal(b, &seeds); err != nil {
		return nil, err
	}
	return seeds, nil
}

func normalizeMAC(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "-", ":")
	return s
}
