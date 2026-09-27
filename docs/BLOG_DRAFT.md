# Using Witchcraft Go Tasks in BLE Watch

Draft for a project write-up. Screenshots and measured replay results are still pending.

I wanted a better view of the Bluetooth devices around my infrastructure. BLE
Watch collects observations through BlueZ, stores history in SQLite, and shows
devices and events in a small Go web application.

The first version passed observations straight into a detector. That was enough
to build presence tracking, signal history, and a few anomaly rules. As I looked
at adding more background work, I started evaluating how to handle submissions,
repeated work, failures, and shutdown.

I liked the task execution model in Palantir's Witchcraft Go Tasks and adopted
it in the observation path. Three things made it worth trying:

- A worker pool gives the processing path an explicit concurrency limit, and
  its collapsing queue can combine repeated work.
- Retry backoff and delayed submission provide a common way to handle work
  that fails or needs to run later.
- Task health reporting can describe failures in the jobs that evaluate device
  state and anomalies.

I also found a small contribution I could use immediately. The existing
submission methods did not return an acceptance error. I added `TrySubmit` and
`TrySubmitAfter` to report cancellation or queue shutdown, while keeping the
original methods. BLE Watch uses `TrySubmit` and records rejections.

That is a limited guarantee: accepted into an in-memory queue does not mean
written to disk or completed. A process can still stop with work outstanding.
The current detector also handles database errors internally, so the queue
cannot retry those failures yet. Those limits shaped the next part of the plan.

I want committed observations to be the source of truth, with background jobs
recomputing device state from that evidence. A burst of observations for one
device should be handled by fewer reconciliation jobs. Restarting the service
should leave a record of what still needs processing.

After the replay harness and inference rules exist, a combined demo will use
a replay clock and labeled synthetic devices. It
will show a stationary beacon going silent, two devices repeatedly appearing
together, and a receiver outage that makes presence uncertain. Each finding
should point back to its evidence. A shared name or a changing address is not
enough to call something an attack.

Witchcraft supplies the queues and workers. BLE Watch supplies the device
model and detection rules. I plan to measure whether that division improves the
application before extending it into a larger pipeline.

Before publishing this article separately, add a synthetic-data screenshot,
link the release verification, and replace planned results only when the replay
tests and measurements exist. This repository is independent of Palantir; the
integration does not imply endorsement or adoption of a proprietary platform.
