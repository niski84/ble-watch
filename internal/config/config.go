// Package config loads ble-watch configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds runtime configuration.
type Config struct {
	Port    string
	DataDir string
	AppURL  string

	// Bluetooth adapter (BlueZ) to scan with, e.g. "hci0".
	Adapter string

	// Seed file for recognized devices (JSON array of {mac,name}).
	SeedFile string

	// RSSI anomaly detection.
	ZThreshold  float64 // z-score above which a surge is flagged
	MinDeltaDBM float64 // minimum absolute RSSI delta (dBm) before flagging
	MinSamples  int     // baseline samples required before flagging

	// Presence.
	GoneAfter time.Duration // unseen duration before a device is "disappeared"

	// Rogue detection.
	RotationN      int // distinct unknown MACs in window -> mac_rotation
	RotationWindow time.Duration

	// Auto-learn recognized devices.
	AutoLearnHours  int // distinct hours seen to auto-promote
	AutoLearnWindow time.Duration

	// Retention.
	RetentionDays       int
	ObservationInterval time.Duration

	// Alerting.
	AlertWebhookURL string
}

// Load reads configuration from the environment, applying sane defaults.
func Load() (Config, error) {
	port := getenv("PORT", "8128")
	dataDir := getenv("DATA_DIR", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("mkdir %s: %w", dataDir, err)
	}
	return Config{
		Port:                port,
		DataDir:             dataDir,
		AppURL:              getenv("APP_URL", "http://localhost:"+port),
		Adapter:             getenv("BLUEZ_ADAPTER", "hci0"),
		SeedFile:            getenv("SEED_FILE", "data/known-devices.json"),
		ZThreshold:          getenvFloat("Z_THRESHOLD", 3.0),
		MinDeltaDBM:         getenvFloat("MIN_DELTA_DBM", 20.0),
		MinSamples:          getenvInt("MIN_SAMPLES", 30),
		GoneAfter:           getenvDuration("GONE_AFTER", 60*time.Second),
		RotationN:           getenvInt("ROTATION_N", 5),
		RotationWindow:      getenvDuration("ROTATION_WINDOW", 60*time.Second),
		AutoLearnHours:      getenvInt("AUTOLEARN_HOURS", 3),
		AutoLearnWindow:     getenvDuration("AUTOLEARN_WINDOW", 24*time.Hour),
		RetentionDays:       getenvInt("RETENTION_DAYS", 30),
		ObservationInterval: getenvDuration("OBSERVATION_INTERVAL", 15*time.Second),
		AlertWebhookURL:     getenv("ALERT_WEBHOOK_URL", ""),
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getenvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
		return fallback
	}
	return f
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}
