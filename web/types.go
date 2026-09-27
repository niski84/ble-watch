package web

// DeviceCard is the view model for a single tracked device.
type DeviceCard struct {
	Mac          string  `json:"mac"`
	Source       string  `json:"source"`
	Name         string  `json:"name"`
	Group        string  `json:"group"`
	AddressType  string  `json:"address_type"`
	Manufacturer string  `json:"manufacturer"`
	LastSeenAt   string  `json:"last_seen_at"`
	LastSeenUnix int64   `json:"last_seen_unix"`
	FirstSeenAt  string  `json:"first_seen_at"`
	SeenCount    int64   `json:"seen_count"`
	LastRSSI     float64 `json:"last_rssi"`
	Recognized   bool    `json:"recognized"`
	Present      bool    `json:"present"`
	BaselineMean float64 `json:"baseline_mean"`
	BaselineStd  float64 `json:"baseline_std"`
	BaselineN    int     `json:"baseline_n"`
}

// EventCard is the view model for a single detection/alert event.
type EventCard struct {
	ID           int64   `json:"id"`
	Ts           int64   `json:"ts"`
	Time         string  `json:"time"`
	Kind         string  `json:"kind"`
	Mac          string  `json:"mac"`
	Name         string  `json:"name"`
	RSSI         float64 `json:"rssi"`
	Severity     string  `json:"severity"`
	Details      string  `json:"details"`
	Acknowledged bool    `json:"acknowledged"`
}

// DashboardData feeds the live dashboard page.
type DashboardData struct {
	Devices         []DeviceCard
	Events          []EventCard
	TotalDevices    int
	PresentCount    int
	RecognizedCount int
}

// DevicePageData feeds the per-device detail page.
type DevicePageData struct {
	Device DeviceCard
	Events []EventCard
}

// DevicesPageData feeds the recognized-device management page.
type DevicesPageData struct {
	Devices         []DeviceCard
	RecognizedCount int
	UnknownCount    int
}

// EventsPageData feeds the full event log page.
type EventsPageData struct {
	Events []EventCard
}
