# What BLE Watch taught me about background task submission

Draft article. Not published to the portfolio. Screenshots will use synthetic data.

I built BLE Watch because I wanted a clearer picture of the Bluetooth devices
around my infrastructure. It collects observations through BlueZ, stores device
and signal history in SQLite, and serves a small Go dashboard. The first version
already tracked appearances, disappearances, and changes in signal strength.

Once that worked, I started looking at the work between receiving an observation
and recording its effects. What happens when processing falls behind? What
happens when the service shuts down while another observation arrives? Can I
tell the difference between accepting work and finishing it?

Those questions led me to Palantir's
[Witchcraft Go Tasks](https://github.com/palantir/witchcraft-go-tasks). I liked its
approach to background execution and integrated it into BLE Watch.

## Why I tried it

There were three reasons to use the library:

- It provides worker pools and a collapsing queue, giving repeated work a shared
  execution path and an explicit concurrency limit.
- It supports retry backoff and delayed submission, which fit work that needs
  another attempt or a later state check.
- It reports task health, which can help explain failures in the jobs that
  evaluate device state and anomalies.

Witchcraft does not supply BLE anomaly detection. Those rules belong to my
application. The library supplies the workers and queue that execute them.

The current integration uses one detector worker. The detector keeps state, so
adding more workers would need an ordering strategy before it could provide a
safe speed improvement. Queue storage is still unbounded. These are limits I
need to address, not benefits I can claim from adding a dependency.

## A submission is not a completed observation

The existing submission methods return no error. That is reasonable for callers
that want fire-and-forget behavior, but I needed to record when shutdown prevents
the queue from accepting an observation.

I added an optional observable submitter to a
[fork](https://github.com/niski84/witchcraft-go-tasks/tree/submission-errors) and
[submitted the change upstream](https://github.com/palantir/witchcraft-go-tasks/pull/142).
`NewObservableItemSubmitter` exposes `TrySubmit` and `TrySubmitAfter`.
A caller can inspect cancellation and shutdown errors with `errors.Is`.
BLE Watch uses the immediate method to count rejected observations separately
from accepted submissions. The upstream change is under review, not merged.

I also had to consider compatibility. Adding methods directly to an existing Go
interface can break someone else's implementation. The revised change adds
separate interfaces and a constructor, leaving the original interfaces,
constructor type, and fire-and-forget behavior intact. Tests include a minimal
legacy implementation so that compatibility is checked by the compiler.

The acceptance check uses the queue's own synchronization. That avoids a
separate "is it running?" check becoming stale before enqueueing the item.
An identical item that collapses into existing work still counts as accepted.

Success means accepted into memory. It does not mean committed to SQLite or
guaranteed to finish. Shutdown can still discard pending work, including delayed
work whose timer has not fired.

## Checking what the integration proves

I added an integration test that sends a synthetic observation through the
Witchcraft processor and the real detector into a temporary SQLite database.
It checks the sample, device state, and rejection of a cancelled submission.
A separate scanner test decodes synthetic BlueZ signals and checks that a
partial property update preserves cached device metadata.

The browser suite runs against the real application on desktop and mobile. It
checks the dashboard, filters, registry, event log, and persisted theme choice.
Those tests establish that the web application works. They do not establish
that the receiver is collecting fresh radio observations.

That distinction mattered during the first live check. Restarting the service
read five observations from BlueZ and wrote them to SQLite. A subsequent
45-second interval produced no new observations, even though BlueZ reported
discovery active and the HTTP health endpoint was green. Cached device state
was enough to make startup look successful.

I recorded that as an unresolved streaming check. I want separate evidence for
cached startup reads, live device updates, task processing, and database writes.
A single "healthy" response cannot stand in for all four. The
[verification report](VERIFICATION.md) records follow-up results without
publishing device identities.

## What still needs work

I now have an isolated demonstration, labeled Skitchcraft / BLE Watch. It feeds
a synthetic sensor through the same task processor and detector, using a new
temporary SQLite database. The sensor builds a baseline, produces a delayed
stronger reading, disappears, and returns. Bluetooth and webhooks are disabled.
This gives me something repeatable to show without publishing nearby devices.

Building the demo found two useful bugs: malformed event-stream framing stopped
browser updates, and a mismatched SQL projection broke loading recognized
devices. Both now have regression coverage. The demo checks actual stored rows
and events rather than treating an accepted task as evidence of persistence.

I kept the Smash Deck patterns already in the app: Go handlers, Templ views,
HTMX fragment updates, and SQLite. They give me a familiar deployment shape,
typed server-rendered views, and a dashboard without a separate client data
layer. Witchcraft sits behind that interface; it does not replace the web stack.
The [architecture notes](ARCHITECTURE.md) distinguish those benefits from the
library features I have not wired up. Browser assets still depend on CDNs.

The detector logs database failures internally. It does not return them to
Witchcraft, so the library cannot retry those writes. Changing the return type
alone would not solve the problem: retrying a partially completed detector call
could repeat state changes or publish an event twice.

The next design step is to commit observations and pending work together, then
have background jobs recompute derived state from committed evidence. Each job
needs a stable key and revision so repeated scheduling can collapse safely and
old work cannot overwrite a newer result. Restart recovery should come from a
stored work record, rather than assuming an in-memory queue is durable.

The synthetic demonstration is not yet a replay benchmark. That harness should run the same
observations through the old and new paths and compare results, job counts,
processing lag, and resource use. That will show whether the added machinery
reduces repeated work and improves recovery.

Later, I want to explore tentative device relationships and explained presence
changes. Two devices appearing together could support an association; it would
not prove common ownership or a Bluetooth connection. Those are planned
features, separate from the integration described here.

This is the kind of infrastructure work I enjoy: follow a real observation
through the system, identify what each boundary guarantees, and make the failure
cases testable. BLE Watch has given me a practical place to exercise the library
and develop a small upstream contribution. I want the remaining architecture
to earn its complexity through the same process.

## Publication notes

Before posting this article on the portfolio, review the latest hardware
findings and choose whether to include a synthetic-data screenshot. Update the
PR status if it changes. This project is independent of Palantir and does not
imply endorsement or reproduce a proprietary Palantir platform.
