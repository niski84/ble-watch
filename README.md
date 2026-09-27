# BLE Watch

I built BLE Watch to see what Bluetooth devices are doing around my own
infrastructure. It collects observations through BlueZ, keeps device and signal
history in SQLite, and serves a live dashboard with presence and anomaly events.

While evaluating how to run background work, I tried Palantir's
[Witchcraft Go Tasks](https://github.com/palantir/witchcraft-go-tasks). I integrated
it into the observation path and added submission errors to a fork so a caller
can tell when cancellation or shutdown prevents work from being accepted.

The [fork](https://github.com/niski84/witchcraft-go-tasks/tree/submission-errors)
is pinned in `go.mod`, so builds do not depend on a local sibling checkout.

I chose it for three reasons:

- It provides a worker pool and a collapsing queue for repeated work.
- It supports retries with backoff and delayed submission, which are useful for
  work that needs another attempt or a later state check.
- It provides task health reporting around the processing that runs my anomaly
  rules. The detection rules themselves belong to BLE Watch.

## What works

The current path is BlueZ discovery, an observation channel, a Witchcraft
submitter, one detector worker, then SQLite and live events. The dashboard shows
devices, groups, signal history, and appearance/disappearance events. Existing
rules check signal surges, reused names, and bursts of random addresses.

The fork's `TrySubmit` method is used by the application. Rejected submissions
are logged, counted, and published as live events. `GET /api/ingestion` exposes
submitted, processed, and rejected counters.

This is an initial integration. One worker limits concurrency, but queue storage
is not bounded. The detector logs database errors rather than returning them to
the task library, so those errors are not retried today. The processing counter
does not certify a database commit. Task health is not yet exposed through the
health endpoint. Delayed submission is implemented in the fork but is not yet
used by BLE Watch.

## Build and run

Use Go 1.27 or later. Live discovery requires Linux, BlueZ, a Bluetooth adapter,
and permission to use the system D-Bus Bluetooth service.

```sh
go test ./...
go build -o ble-watch ./cmd/ble-watch
./ble-watch
```

Open `http://localhost:8128`. `PORT`, `DATA_DIR`, and `BLUEZ_ADAPTER` override the
default port, `data` directory, and `hci0` adapter. A `.env` file is loaded only
by `scripts/reload.sh`; direct execution reads the process environment.

Device seeds are optional. Keep any real seed file at `data/known-devices.json`
or set `SEED_FILE`. Runtime records, seeds, logs, and screenshots are excluded
from this repository. See [ENDPOINTS.md](ENDPOINTS.md) and
[BACKUPS.md](BACKUPS.md) for API and backup details.

The service has no authentication and listens on the configured port on all
interfaces. Run it on a trusted network or behind access controls. Publishing
the source does not require publishing a live receiver or its database.

## Verification and next work

[Verification](docs/VERIFICATION.md) records what was tested and separates
synthetic integration tests from hardware observations. The shared test-agent
suite checks the running API and dashboard. [Automation](docs/TESTING.md)
describes the commands and schedule.

[The implementation plan](INTELLIGENCE_PLAN.md) covers replayable observations,
durable work, explained anomalies, and tentative device relationships.
[The task board](docs/TASKS.md) assigns file ownership and acceptance criteria
so agents can work without editing the same code at the same time.

The relationship graph and person-presence inference are planned. They are not
features of this release. Device co-presence would indicate an association,
not prove ownership, identity, or a Bluetooth connection.

[The article draft](docs/BLOG_DRAFT.md) describes the integration and its current
limits. Screenshots will be added from an isolated demo with synthetic devices.
