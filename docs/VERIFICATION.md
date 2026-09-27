# Release verification

Checked on 2026-09-27. This report covers the initial Witchcraft integration,
publication cleanup, and smoke automation. It does not certify the planned
device-intelligence pipeline.

## Build and integration

- `go build ./...`: passed locally.
- `go test -race ./...`: passed locally.
- `TestBlueZSignalProducesObservation`: synthetic D-Bus device-added and
  property-change signals produced observations; partial updates preserved
  cached device metadata.
- `TestObservationPersistsThroughTaskQueue`: a synthetic observation passed
  through Witchcraft and the detector into a temporary SQLite database. Device
  state and the sample matched. A cancelled submission returned the fork's
  lifecycle error and incremented the rejection counter.
- Fork `go test -race ./executor ./internal/queue`: passed, including immediate
  and delayed acceptance errors during cancellation/shutdown.
- The module uses public fork revision `04039794046d`, with a checksum in
  `go.sum`. No local module replacement remains.

These tests validate specific behaviors. Other packages without test cases still
need coverage as the roadmap progresses. Future milestones include storage
failure recovery, durable work, classification, and relationship inference.

## Browser and automation

The shared test-agent suite passed 10 tests across desktop and mobile against
the real running application, without mocked API responses. It verifies the
health/counter contracts, dashboard filters, registry/events pages, and theme
persistence. It does not require radio observations to pass.

An hourly schedule is registered in the existing coverage-agent for
`ble-watch.spec.ts`. A manual invocation through coverage-agent's normal runner
completed successfully with exit code 0. That verifies the runner, not that an
hourly timer has already fired. See [TESTING.md](TESTING.md) for the exact API
distinction between schedule filenames and project IDs.

## Live Bluetooth and SQLite

A service restart read five device observations from BlueZ and processed five
submissions. The maximum SQLite observation ID increased by five and the newest
record was current immediately afterward. No synthetic data was inserted into
the live database for this check.

That establishes the BlueZ startup-read path through the application into
SQLite. Cached BlueZ state can produce these startup observations, so this is
not proof of fresh radio advertisements.

The separate 45-second streaming check reported:

```json
{
  "status": "blocked",
  "bluez_discovering": true,
  "sample_seconds": 45,
  "processed_delta": 0,
  "observation_id_advanced": false,
  "newest_observation_age_seconds": 45
}
```

Continuous live reception remains unverified. Diagnose it with a controlled
owned beacon and the receiver status before presenting a live radio demo.
HTTP liveness alone does not establish scanner health.

## Privacy review

The public application starts from a sanitized source snapshot. Private Git
history, remote configuration, and personal commit contact details are excluded.
Public commits use the GitHub account name and GitHub noreply address.

The seed list was empty at inspection, but it and the runtime PID file were
removed from tracking so later device entries cannot be published accidentally.
Their local files were preserved. Databases, environment files, logs, runtime
reports, and screenshots are excluded. The public source contains only an
explicit synthetic address and a formatting placeholder.

The tracked-content audit checks private filesystem paths, private IP addresses,
email addresses, common credential patterns, non-fixture device addresses, and
runtime artifacts. Findings show locations rather than matched secrets. The
release also reviews commit metadata and the publication file list. No scanner
can guarantee detection of every possible secret.

The dependency fork preserves upstream history and license notices. Only the
new submission-error change is authored with the public account identity; the
private development commits are not ancestors of the published branch.

Publishing under an existing GitHub account exposes that account's public
identity. This release does not claim anonymity.

## Writing review

The public introduction and article draft use a first-person project account.
They give three reasons for trying the library and identify its current limits.
Witchcraft is credited with execution primitives, not BLE classification or
Palantir's proprietary decision systems. The planned graph and person-presence
features are explicitly unfinished.

Emoji and em dashes were removed from owned application text and comments.
Generated templates were rebuilt from source. Name-reuse and address-churn UI
descriptions now explain that these heuristics do not establish spoofing or a
threat; existing event identifiers and rule severity behavior remain unchanged.

Screenshots and separate blog publication remain pending. Use synthetic data
for those assets; live device names and addresses are not part of this release.
