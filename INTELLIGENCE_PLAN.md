# BLE Watch intelligence plan

Status: implementation roadmap. The initial task-library integration exists;
the intelligence milestones below are not implemented. Written 2026-09-27.

## Working agreement

Read [docs/TASKS.md](docs/TASKS.md) before starting a task. It is the coordinator's
record of status, dependencies, ownership, and acceptance evidence. Agents work
in separate branches or worktrees and claim exact files before editing. Only the
coordinator changes the task board or merges shared contracts. A passing local
test makes a task ready for review; it does not mark it complete.

[docs/TESTING.md](docs/TESTING.md) defines the shared test-agent integration and
automation. Each milestone must extend the tests that establish its behavior.
[docs/BLOG_DRAFT.md](docs/BLOG_DRAFT.md) is the article draft. Update its claims
only after the corresponding task is accepted. Use synthetic devices for public
screenshots and keep live captures private.

## Outcome

Turn BLE Watch into a useful household device observatory. Show which devices
have been observed, how their state changes, which devices may be related, and
what evidence supports an unusual event. Every inference must be inspectable,
replayable, and correctable by the user.

The first demonstration must work from recorded or synthetic observations without
Bluetooth hardware. A live demonstration on our own labeled devices follows.
Measure whether Witchcraft reduces repeated processing, recovers failed work,
and makes processing failures easier to diagnose.

## What exists today

The implementation inspected at commit `bed853f` contains:

- BlueZ discovery and property conversion in `internal/scanner/scanner.go`.
- A Witchcraft submitter and one worker in `internal/ingest/processor.go`.
- Device state, recognition, RSSI baselines, appearance/disappearance, and
  anomaly heuristics together in `internal/anomaly/detector.go`.
- SQLite device, sampled observation, and event storage in `internal/store`.
- Device pages, history charts, SSE, and optional webhook delivery.
- Submission and processing counters at `GET /api/ingestion`.

There is no device classification engine, person presence model, relationship
graph, durable work ledger, or deterministic replay harness yet.

Several earlier descriptions overstated the integration:

- Worker concurrency is limited; the collapsing queue has no configured capacity.
- Queue equality uses the whole comparable observation, not its `String()` value.
  `String()` supplies a health key. It does not define deduplication.
- `Detector.Process` returns no error. Database failures logged there do not reach
  Witchcraft, and `processed` does not prove successful persistence.
- Submitting successfully means accepted into memory, not durably committed.
- Cancellation does not provide application-level draining or crash recovery.
- Delayed submission exists in the fork but BLE Watch does not currently use it.
- The task health source is not connected to the application's health response.

## Scope and useful household questions

Start with one receiver and a small set of our own labeled devices. Keep the
implementation within BLE Watch. Add receivers only when single-receiver results
and recovery behavior are measurable.

The first useful questions are:

1. Is a selected device recently observed, becoming stale, or no longer observed?
2. Has a normally observed stationary device stopped reporting while scanning
   still works?
3. Do two devices repeatedly appear or disappear together?
4. What changed, and which observations support that conclusion?
5. Does an explicitly associated carried device provide evidence that its owner
   may be nearby?

Person presence is an optional inference from user-confirmed device associations.
A device can be left at home. Its silence is not proof that its owner left.
Unrecognized addresses remain device observations, with no automatic assignment
to a named person. The system must be able to answer "unknown."

## Model observations separately from interpretations

### Observation and receiver

An observation has an immutable ID, receiver ID, receiver session ID, source
sequence, observation time, receive time, address and address type, available
manufacturer/service fields, RSSI availability, and schema version.

Record field provenance: directly reported, carried forward from the scanner
cache, or missing. The existing adapter consumes BlueZ property updates; do not
label these as a complete capture of every radio advertisement. Event counts
describe our source stream, not necessarily radio packet counts.

A receiver has health states `starting`, `scanning`, `degraded`, and `offline`.
Use explicit scanner status and heartbeat evidence. Radio silence alone must not
be used to declare the receiver broken or every device absent.

### Device and presence

Use an internal device ID with address observations attached to it. A new address
initially creates a separate candidate. A proposed link between addresses stores
evidence and remains reversible; shared names, vendors, or RSSI alone must not
silently merge identities.

Presence states are `unknown`, `observed`, `stale`, and `not_observed`:

- Valid recent observations move a device to `observed`.
- A configurable silence interval moves it to `stale`.
- Continued silence with healthy receiver coverage moves it to `not_observed`.
- Unavailable coverage or insufficient history produces `unknown`.
- A fresh observation restores `observed`; minimum dwell times prevent flapping.

Store the transition time, reason, input revision, and policy version. Presence
uses observation time with an explicit late-data policy. Older reports cannot
move `last_seen` backward or erase newer state.

Familiarity, user trust, device type, and presence are separate attributes.
Repeated sightings may establish familiarity but must not automatically establish
trust. Type labels such as beacon, phone, or headphones begin with deterministic
rules and may remain `unknown` or user-confirmed.

### Relationships and optional person presence

Represent relationships as typed edges: `observed_together`,
`possible_same_device`, and `user_associated_with`. Each edge records supporting
and contradicting evidence, independent encounter count, observation period,
score, rule version, and user confirmation or dismissal.

For `observed_together`, compare arrival/departure episodes across repeated
encounters. Discount devices that are almost always present. One shared window
is insufficient. Restrict candidate pairs to overlapping encounters and cap work
per window so the graph does not require all-pairs comparisons forever.

An evidence score is not a calibrated probability. Until measured against labels,
show evidence strength and sample counts rather than "95% certain."

Optional person states are `unknown`, `presence_supported`, and
`presence_not_recently_supported`. Derive them only from explicitly associated
devices, with freshness, receiver coverage, and contradictory evidence visible.
Use language such as "The associated phone was observed recently." Do not claim a direct
person detection or a precise location from that fact.

Graph lines describe inferred associations, not observed Bluetooth connections.

## Execution architecture

Keep validation, normalization, and durable intake together. Do not introduce a
queue for every function. Use asynchronous tasks where work can be collapsed,
delayed, retried safely, or limited independently.

1. The scanner adapter produces observation envelopes. Normalization validates
   required fields, preserves missing values, and orders set-like UUID fields.
2. A bounded intake buffer feeds SQLite. Commit observations and affected work
   revisions together. Count and expose any input rejected before persistence.
3. Witchcraft schedules reconciliation by device and analysis window. Workers
   read committed observations, calculate aggregates and presence, then commit
   derived state, transitions, and further work intents transactionally.
4. Separate task types compute relationship candidates and, later, classification
   suggestions. They consume versioned aggregates rather than raw scanner events.
5. A durable event outbox feeds SSE and optional webhooks after state commits.
   SSE clients refresh state on reconnect. Webhooks include stable event IDs so
   consumers can deduplicate repeated delivery.

Start with three task families: device reconciliation, relationship analysis,
and outbox delivery. Person presence can be a small deterministic projection of
confirmed associations and device states.

Use comparable keys such as `(device_id, window_start, policy_version)`. Store
the payload and latest requested revision outside the in-memory queue. Multiple
submissions for a key can collapse while the worker still reads all committed
inputs. Do not collapse observations before preserving evidence needed by counts
and anomaly features.

Persist requested and completed revisions. If inputs change while processing,
the newer revision stays pending. An outdated worker cannot overwrite newer
results. Begin with serial device reconciliation; add parallelism only after
ordering and revision tests pass.

Witchcraft provides task execution, collapsing, delay, retries, and health
reporting. SQLite remains the source of truth for pending work and due times.
A dispatcher resubmits outstanding work on startup and periodically. Permanent
failures become visible failed jobs with an explicit retry action. Exhausted
library retries must not make durable work disappear or loop forever unnoticed.

Use delayed submission for a presence reevaluation or window close. A fresh
sighting may make an earlier scheduled check obsolete. The handler rereads
current state and persisted deadlines before making a transition; it never
assumes that a timer firing means a device has disappeared.

On shutdown, stop intake, finish or bound pending intake commits, stop scheduling,
allow active work a deadline, and close storage after workers finish. Unfinished
committed work remains pending for restart. Exercise the fork's immediate and
delayed submission errors in this path and expose rejected scheduling attempts.

## Anomalies with explanations

Implement two rules first:

- Unexpected silence for a user-selected stationary device, gated on healthy
  receiver coverage and sufficient history of its observed cadence.
- Sustained RSSI change relative to that device's baseline on that receiver,
  gated on sample count, duration, and a minimum absolute change.

Evaluate against the preceding baseline before updating it. Start with a robust
window statistic and an explicit minimum variation floor. Include a warmup
state; do not present an alert before a baseline exists. Version parameters and
test sensitivity on labeled recordings before choosing shipping defaults.

Each finding stores its rule/version, expected behavior, observed behavior,
supporting observation/window IDs, coverage, severity, and suppression reason.
Use cooldowns, recovery conditions, and hysteresis to avoid repeated alerts.

The proposed policy treats name reuse and bursts of random addresses as
informational observations. The release UI uses "Name match" and "Address churn,"
but the existing internal event types and severity behavior are unchanged.
Some still emit critical events. A same-name device is not established spoofing,
and address churn is not proof of a threat. Review severity during migration.

Later candidates include changed daily presence patterns and relationship
changes. Add them only with enough labeled history to assess false positives.
AI or Jev may suggest a type or summarize evidence later; deterministic code
owns state transitions, persistence, retries, and alert policy.

## Demo and product surface

Provide a replay command with an injected clock, isolated database, fixed
fixture, and adjustable playback speed. It must execute the same domain logic
as live intake. Fixture labels are hidden evaluation truth, not model inputs.

The first complete scripted demonstration:

1. A stationary beacon, a phone, and headphones appear. Their timelines explain
   the transitions from unknown to observed.
2. Repeated phone/headphone encounters create a tentative relationship. An
   always-present beacon does not become a strong companion merely by overlap.
3. A user confirms which phone belongs to them. The optional person card shows
   the associated device evidence and its limitations.
4. The beacon stops reporting while the receiver remains healthy. One explained
   silence finding appears after the configured threshold.
5. The receiver goes offline. Presence becomes uncertain and device-silence
   findings are suppressed for that coverage gap.
6. A persistence failure and process restart occur. Committed work resumes and
   derived events are not duplicated.

Build a device timeline, an evidence panel for every finding, and a relationship
graph with different styles for tentative and confirmed edges. Keep raw facts,
inferences, and user labels distinguishable. Show replay mode and receiver health
prominently. Follow the existing dark-theme and web-stack conventions.

## Verification and evidence of value

Capture the existing implementation's results before changing behavior. Run
the same fixture through the baseline and proposed pipeline where comparable.
Record input count, stored count, derived job executions, CPU time, peak memory,
storage growth, processing lag, and findings against labeled expected outcomes.

Required tests and acceptance gates:

- Replay is repeatable: identical inputs, clock, and policy produce the same
  domain outputs, independent of playback speed.
- Duplicates and restart do not duplicate derived state transitions or events.
  Webhook delivery remains at least once, with stable deduplication IDs.
- Late reports, missing RSSI, repeated names, and address changes do not silently
  corrupt identity, counters, or last-seen state.
- Transient storage errors recover; permanent failures appear as failed work.
- Scanner outage suppresses silence findings in the outage fixture.
- Co-presence evidence stays tentative and explains both support and conflict.
- Intake and dispatcher limits bound in-memory work during a tenfold replay
  burst. Overflow, persistence rejection, and backlog age are visible.
- The collapse benchmark executes fewer reconciliation jobs for a burst of one
  key while producing the same final state and preserving evidence counts.
- Graceful shutdown and forced restart preserve all committed pending work.
- Unit and race tests cover revision conflicts, delayed reevaluation, retries,
  and shutdown. Integration tests use real SQLite and controlled failure points.

Set numerical latency and resource budgets after measuring the baseline on the
target machine. Report measurements, not an assumed benefit from the dependency.
Synthetic fixtures establish correctness for their scenarios; live labeled
sessions are required before making accuracy claims about household use.

## Delivery milestones

1. **Replay and baseline.** Add a source interface, clock, isolated fixtures, and
   baseline report. Demo: replay the existing detector without BLE hardware.
2. **Reliable state.** Add durable intake/work revisions, normalization, receiver
   coverage, and presence transitions. Demo: restart and outage without false
   departure events or lost committed work.
3. **Useful anomalies.** Extract pure rules, add baselines and evidence records,
   and expose the timeline. Demo: explain a silence and sustained RSSI finding.
4. **Related devices.** Add encounter aggregates and tentative relationship edges
   with feedback. Demo: distinguish recurring companions from constant overlap.
5. **Optional person inference.** Add explicit associations and cautious presence
   summaries. Demo: show what is known when a phone remains behind.
6. **Operational proof.** Compare burst/recovery results, surface task health,
   finalize the demo, and document where Witchcraft helped and what it cost.

Each milestone should be usable and reviewable on its own. Begin with milestone
1 before committing to the whole refactor. Leave classification models, multiple
receivers, and Hive integration for subsequent proposals supported by evidence.

## Migration and open decisions

Preserve current API compatibility where practical. Use additive database
migrations and compare new projections in shadow mode before making them active.
Only one path may publish alerts during comparison. Keep a rollback path to the
current detector while collecting labeled recordings.

The fork requires Go 1.27. Public builds must pin an accessible fork revision
and succeed without a sibling checkout. Do not publish private development
history, live seeds, database contents, logs, or personal commit email addresses.

Resolve during the early milestones:

- Which owned devices provide stable enough observations for the first live demo?
- What does BlueZ report for those devices, including cached versus fresh fields?
- How much raw evidence should be retained, given measured disk growth?
- What silence thresholds fit each selected device's measured cadence?
- Which household question proves useful enough to justify ongoing operation?

The next intelligence milestone is the replay harness, baseline measurements,
and fixture labels. Publication, smoke automation, and documentation have their
own release tasks; completing those does not complete the intelligence roadmap.
