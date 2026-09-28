// Package scanner discovers BLE devices via BlueZ D-Bus.
package scanner

import "fmt"

// Observation is a cached device snapshot or an advertising-related D-Bus update.
type Observation struct {
	Mac          string  // upper-case "AA:BB:CC:DD:EE:FF"
	Name         string  // device alias/name (may be empty)
	RSSI         float64 // dBm, negative
	AddressType  string  // "public" or "random"
	MfgID        int     // primary Bluetooth SIG company ID (0 = none)
	ServiceUUIDs string  // comma-separated advertised UUIDs
	Ts           int64   // unix seconds
}

// String provides a task health key. Queue deduplication compares all fields.
func (o Observation) String() string {
	return fmt.Sprintf("%s@%d", o.Mac, o.Ts)
}
