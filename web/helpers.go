package web

import (
	"fmt"
	"sort"
)

// BoolLabel provides stable data attributes for client-side device filters.
func BoolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// SeverityBadge returns a DaisyUI badge class for an event severity.
func SeverityBadge(sev string) string {
	switch sev {
	case "critical":
		return "badge-error"
	case "warn":
		return "badge-warning"
	default:
		return "badge-info"
	}
}

// KindLabel returns a human-readable label for an event kind.
func KindLabel(kind string) string {
	switch kind {
	case "appeared":
		return "Appeared"
	case "disappeared":
		return "Disappeared"
	case "rssi_anomaly":
		return "Signal surge"
	case "rogue_device":
		return "Name match"
	case "mac_rotation":
		return "Address churn"
	case "device_recognized":
		return "Recognized"
	case "device_seen":
		return "Seen"
	}
	return kind
}

// KindEmoji returns a compact glyph for an event kind.
func KindEmoji(kind string) string {
	switch kind {
	case "appeared":
		return "+"
	case "disappeared":
		return "-"
	case "rssi_anomaly":
		return "RSSI"
	case "rogue_device":
		return "!"
	case "mac_rotation":
		return "R"
	case "device_recognized":
		return "*"
	}
	return "•"
}

// rssiToProximity maps RSSI in dBm (-100..-30) to a 0..100 percentage.
func rssiToProximity(rssi float64) int {
	if rssi >= -30 {
		return 100
	}
	if rssi <= -100 {
		return 0
	}
	return int((rssi + 100) / 70 * 100)
}

// RSSIBarWidth returns a CSS width for a signal-strength bar.
func RSSIBarWidth(rssi float64) string {
	return fmt.Sprintf("%d%%", rssiToProximity(rssi))
}

// RSSIBarColor returns a DaisyUI color class for a signal bar.
func RSSIBarColor(rssi float64) string {
	p := rssiToProximity(rssi)
	switch {
	case p >= 70:
		return "bg-success"
	case p >= 40:
		return "bg-warning"
	default:
		return "bg-error"
	}
}

// deviceLabel returns a device's display name, falling back to its MAC.
func deviceLabel(d DeviceCard) string {
	if d.Name != "" {
		return d.Name
	}
	return d.Mac
}

// recognizedDevices counts recognized devices in a list.
func recognizedDevices(devs []DeviceCard) int {
	n := 0
	for _, d := range devs {
		if d.Recognized {
			n++
		}
	}
	return n
}

// unknownDevices counts unrecognized devices in a list.
func unknownDevices(devs []DeviceCard) int {
	n := 0
	for _, d := range devs {
		if !d.Recognized {
			n++
		}
	}
	return n
}

// DeviceGroups returns unique non-empty groups in stable alphabetical order.
func DeviceGroups(devs []DeviceCard) []string {
	seen := map[string]bool{}
	for _, d := range devs {
		if d.Group != "" {
			seen[d.Group] = true
		}
	}
	out := make([]string, 0, len(seen))
	for group := range seen {
		out = append(out, group)
	}
	sort.Strings(out)
	return out
}

// PresenceDotClass returns the presence indicator dot classes.
func PresenceDotClass(present bool) string {
	if present {
		return "h-2 w-2 rounded-full shrink-0 bg-cyan-400"
	}
	return "h-2 w-2 rounded-full shrink-0 bg-base-content/25"
}

// NavItemClass returns sidebar link classes, highlighting the active section.
func NavItemClass(section, current string) string {
	if section == current {
		return "rounded-lg active"
	}
	return "rounded-lg"
}

// HourBtnClass returns the device-page time-window button classes.
func HourBtnClass(active bool) string {
	if active {
		return "btn btn-xs btn-primary"
	}
	return "btn btn-xs btn-ghost"
}

// hourOption describes a time-window selector button on the device page.
type hourOption struct {
	Val    string
	Label  string
	Active bool
}

// HourOptions returns the device-page time-window selector options.
func HourOptions() []hourOption {
	return []hourOption{
		{Val: "1", Label: "1h"},
		{Val: "6", Label: "6h", Active: true},
		{Val: "24", Label: "24h"},
		{Val: "168", Label: "7d"},
	}
}

// RSSILabel formats an RSSI value for display.
func RSSILabel(rssi float64) string {
	if rssi == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%.0f dBm", rssi)
}
