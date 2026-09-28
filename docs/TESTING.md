# Tests and automation

## Build and database integration

```sh
go build ./...
go test -race ./...
```

`TestObservationPersistsThroughTaskQueue` sends a synthetic observation through
the actual Witchcraft processor and detector into a temporary SQLite database.
It checks the stored sample, device state, and cancellation rejection. It does
not require Bluetooth hardware and does not prove live radio reception.

`TestBlueZSignalProducesObservation` separately checks BlueZ signal decoding and
partial property updates with synthetic D-Bus signals.

The GitHub checks workflow builds the project, runs Go tests with race detection,
and audits tracked publication content. It requires no hardware or secrets.

## Browser smoke suite

With BLE Watch running:

```sh
npm ci
npx playwright install chromium
TEST_BASE_URL=http://localhost:8128 npm test
```

The suite runs on desktop and mobile. It checks liveness, counter types,
dashboard regions, device filters, registry/events pages, and theme persistence.
Zero observations is a valid smoke result. No device mutations are performed.
Screenshots, traces, video, and failure DOM snapshots are disabled because the
target may contain real device names and addresses.

`tests/ble-watch.spec.ts` is the public source. The shared test-agent holds an
identical copy under the same relative filename, exporting `suite.id` as
`ble-watch`. TEST-01 owns both copies and must update them together. The release
coordinator compares them before accepting a change:

```sh
cmp tests/ble-watch.spec.ts ../test-agent/tests/ble-watch.spec.ts
cd ../test-agent
./run-test.sh ble-watch.spec.ts
```

Do not publish the shared test-agent's other suites or its runtime reports as
part of this project. They belong to separate services and may contain private
details. Test-agent is a command runner, not a Go service; it has no reload script.

## Recurring local smoke run

The existing coverage-agent discovers test-agent specs at startup. Reload that
service after adding a spec. Use its API to schedule this file hourly:

```sh
curl -fsS -X PUT http://localhost:8109/api/schedules/ble-watch.spec.ts \
  -H 'Content-Type: application/json' \
  -d '{"interval_mins":60,"enabled":true}'
```

There is an existing naming mismatch in coverage-agent: timed schedules use
the filename (`ble-watch.spec.ts`), while the manual run endpoint resolves the
project ID (`ble-watch`):

```sh
curl -fsS -X POST http://localhost:8109/api/specs/ble-watch/run
curl -fsS http://localhost:8109/api/schedules/ble-watch.spec.ts
```

The run response includes `run_id`. Read `GET /api/runs/{run_id}` for completion.
A manually triggered pass verifies the runner, not that an hourly timer has
already fired. The scheduler inherits its process environment; BLE Watch's
suite defaults to localhost port 8128 unless `TEST_BASE_URL` is set.

This configuration runs only the smoke suite. Future replay and fault tests
must be implemented before adding them to automation. The task board records
those dependencies.

Follow-up caveat: discovery now prefers the co-located public BLE spec for the
manual project-ID route. The shared runner still has `testDir: './tests'`, so
that absolute outside path produces no matching tests. Until that owning
service/runner integration is corrected, use `./run-test.sh ble-watch.spec.ts`
from test-agent for a manual run. The timed filename schedule is unaffected.
Do not report the failing project-ID invocation as successful automation.

## Live hardware check

On the machine running the receiver and its database:

```sh
python3 scripts/check-live.py --seconds 45
```

The script reads scanner readiness, samples ingestion counters, and checks
whether SQLite observation IDs advance with recent timestamps. It requires
the same nonzero scanner start time at both boundaries and an increase in live
updates, not just cached seeds. It prints only
aggregate results, never device addresses or names. It uses a read-only database
connection and does not inject synthetic observations into the live service.

Exit 0 means activity was observed during the interval. Exit 2 means hardware
activity was not established or the check could not complete; inspect the fixed
error classification. Cached devices read during startup alone do not satisfy
this check. Keep hardware checks separate from browser liveness tests.

## Isolated demo

See [DEMO.md](DEMO.md). `go test -race ./...` includes an approximately 18-second
real-timer scenario test that asserts SQLite samples and event order. `-short`
skips that scenario, so do not use it for full demo acceptance. The Python helper
tests run with `python3 -m unittest discover -s scripts -p 'test_*.py'`.

The browser suite now checks scanner readiness and pipeline counters too. The
synthetic scenario test runs only with `BLE_DEMO_TEST=1`; live smoke runs skip it.
That flag must never point at the live receiver. Both public and shared copies
include these checks. No new scheduled radio or synthetic workload is enabled.

## Publication checks

Stage the intended public files, then run `python3 scripts/audit-public.py` and
`git diff --cached --check`. The script checks the Git index, not ignored local
files. It flags private paths/addresses, likely credentials, device identifiers
outside explicit synthetic examples, runtime artifacts, emoji, and em dashes.
Review new examples and generated assets manually. This is a targeted check,
not a guarantee that arbitrary secrets will be detected.

Review commit authors, commit messages, and reachable history separately. Use a
new sanitized snapshot for the first public release; the private repository's
history is not part of the public release.
