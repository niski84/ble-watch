# Why Smash Deck and Witchcraft fit together

Smash Deck is the workspace's application pattern, not a replacement task queue.
Witchcraft Go Tasks is a background-execution library, not a web framework or
BLE intelligence engine. BLE Watch uses both at different boundaries.

## What runs today

BlueZ produces observations for the live receiver. The isolated demo supplies
synthetic observations instead. Both submit to the same ingestion processor,
which uses Witchcraft and one detector worker. The detector updates SQLite and
publishes events. Go HTTP handlers expose records and Templ views. Browser
EventSource notifications trigger HTMX fragment refreshes.

The detector owns presence and signal rules. The store owns database access.
The API owns HTTP presentation. No other service needs to take over BLE logic
to display this dashboard. This remains one deployable Go service with internal
packages, not a new network of microservices.

## Why retain the Smash Deck patterns

- **Reuse the working service structure.** Go handlers, environment configuration,
  a reload script, and the shared browser-test runner fit the existing workspace.
  There is no new application framework or frontend build system to operate.
- **Keep presentation close to its data.** Templ gives typed, compiled components;
  HTMX refreshes server-rendered fragments. That avoids maintaining a second
  client-side copy of the device and event rendering rules.
- **Keep a small deployment footprint.** SQLite and embedded templates let this
  receiver run as a single service. A separate receiver on another machine
  would need a deliberate ingestion contract, not shared access to its database.

BLE Watch follows the main stack choices: Go, Templ, HTMX, Tailwind, DaisyUI,
and a small amount of JavaScript. It also loads Alpine, although the current
views do not depend on Alpine components. It is not fully aligned with the
workspace's current build convention: Tailwind/DaisyUI and other browser assets
are CDN-loaded instead of compiled and embedded. The reload script is standalone
rather than using the shared reload core. These are explicit follow-up items.

## What Witchcraft earns here

### Current integration verdict

The live per-observation submission is an experiment, not the intended final
work boundary. Witchcraft accepts short tasks; task duration is not the problem.
The problem is that a BlueZ update contains a changing timestamp and often a
changing RSSI. Queue equality uses the whole observation, so the repeated
submissions that its collapsing queue could combine are uncommon. Every
detector call also performs database work, while detector errors are logged
inside the call instead of returned to Witchcraft. The library therefore
cannot retry a failed database operation here.

After restoring live Bluetooth reception, a five-second sample accepted about
92 observations per second and returned from the detector about 56 per second.
The accepted-minus-returned difference increased by 180 during that interval.
This is pressure evidence, not an exact queue length: accepted duplicates can
collapse, and the library's queue gauge is not connected to the application.
BLE Watch used about 7% of one CPU in a separate short sample, so there is no
evidence that the host CPU was saturated. The serial worker, SQLite writes,
queue behavior, and source burst rate still need a measured breakdown.

The demo proves that the library executes accepted work, delayed submissions,
and rejection reporting. It does not establish a throughput or reliability
advantage over a small Go worker for the current scanner path. An interview
description should present this as a measured integration experiment and an
architecture correction, not as proof that Witchcraft improved BLE ingestion.

### Better work boundary

1. Store validated observations in bounded batches before scheduling derived
   work. Record overflow explicitly. Preserve the observations needed for
   signal statistics and event evidence.
2. Persist a pending revision keyed by device and analysis window in the same
   transaction as the new evidence. Submit that small comparable key to
   Witchcraft after commit. Repeated requests for a key can then collapse while
   the worker reads every committed observation.
3. Make the worker calculate an idempotent projection and return transient
   failures. Witchcraft can retry it; a restart dispatcher can resubmit pending
   revisions. A stale worker must not overwrite a newer revision.
4. Use delayed tasks for presence reevaluation. The worker rechecks persisted
   last-seen time and receiver coverage when it fires. A timer alone must not
   mark a device absent.
5. Expose the actual queue gauge, task duration, failed jobs, intake overflow,
   committed observations, and projection freshness in the UI. Compare with the
   current path on the same recorded input before claiming a benefit.

BlueZ provides a stream of updates, not a required file of completed scans.
An import/replay command is still valuable for repeatable tests; live intake
should use the same durable evidence contract. The detailed milestones and
acceptance checks are in [INTELLIGENCE_PLAN.md](../INTELLIGENCE_PLAN.md).

### What the initial integration does

The single worker serializes calls to a stateful detector, and queue ownership
gives submission and shutdown a clear boundary. The fork adds explicit errors
for rejected immediate or delayed submissions. The demo exercises delayed
submission, while the real receiver uses immediate submission.

The library also offers collapsing work, retry backoff, and task-health support.
Their presence in the dependency does not mean BLE Watch uses them effectively:

- Observations include timestamps and signal values, so most are distinct.
  There is no measured queue-collapse benefit yet.
- Detector database errors are logged internally instead of returned as task
  errors. Persistence retries are not implemented.
- Task health is not wired into the HTTP health endpoint.
- Queue storage is unbounded and in-memory. Accepted work can be lost on exit.

For today's small workload, a plain Go worker could be simpler. The defensible
benefit now is a tested lifecycle boundary and reusable scheduling primitives,
not a throughput claim. Durable work and measured comparisons remain future
work; the dependency must justify any further complexity.

## Names and scope

BLE Watch is the repository and application. Skitchcraft is a working demo or
studio label. Witchcraft Go Tasks remains credited to Palantir. This project is
independent and makes no claim to implement or reproduce Palantir's proprietary
platform. The professional-representative ideas are aspirational and outside
this service's scope.
