# ble-watch API Endpoints

Base URL: `http://localhost:8128`

## Health

### GET /api/health
Service liveness check.

```json
{"status": "ok", "service": "ble-watch"}
```

## Devices

### GET /api/ingestion
Process-local counters: `submitted`, `rejected`, and `processed`. These counters
reset on restart. Processed means the detector returned, not that every database
write succeeded. Counters are read independently and may reflect concurrent work.

### GET /api/devices
All tracked devices (newest-seen first), each as a `DeviceCard` (mac, name,
address_type, manufacturer, last_seen, seen_count, last_rssi, recognized,
present, baseline_mean/std/n).

### GET /api/devices/{mac}
Single device, same shape.

### GET /api/devices/{mac}/series?hours=N
Sampled RSSI time-series (default one sample per 15 seconds per device).
`OBSERVATION_INTERVAL` controls sampling. `hours` default 24, cap 168.

### GET /api/devices/{mac}/chart?hours=N
Chart-ready aggregate: downsampled `series` (~240 pts), RSSI `histogram`
(20 bins over -100..-20 dBm), `present` spans (online gaps split on `GONE_AFTER`),
and the device's `mean`/`std` baseline.

### POST /api/devices/{mac}/recognize
Mark recognized (source `manual`).

### POST /api/devices/{mac}/unrecognize
Remove from recognized set.

### POST /api/devices/{mac}/rename
Body `{"name": "..."}`. Sets the display alias.

## Events

### GET /api/events?limit=N
Recent events, newest first. Kinds: `appeared`, `disappeared`, `rssi_anomaly`,
`rogue_device`, `mac_rotation`, `device_recognized`, `device_seen`.

### POST /api/events/{id}/ack
Mark an event acknowledged.

## Live

### GET /api/stream
SSE stream `global`. Named events: `device_seen`, `appeared`, `disappeared`,
`rssi_anomaly`, `rogue_device`, `mac_rotation`, `device_recognized`.

### GET /partials/devices
HTML fragment (stats + device grid) for SSE-triggered refresh.

### GET /partials/events
HTML fragment (event feed) for SSE-triggered refresh.

## Pages

| Path | Page |
|------|------|
| `/` | Live dashboard |
| `/devices` | Device registry (recognize / alias) |
| `/devices/{mac}` | Device detail (charts) |
| `/events` | Full event log |

## Recognized-device seed

`data/known-devices.json` is a JSON array of `{"mac": "...", "name": "..."}` entries
loaded on startup (`recognized` source `seed`). Devices are also auto-promoted
(`learned`, seen in ≥3 distinct hours over 24h) and manually via the UI (`manual`).
