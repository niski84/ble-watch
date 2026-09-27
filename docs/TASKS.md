# Work board

The coordinator owns this file. Status values are `ready`, `blocked`, `active`,
`review`, and `done`. A task is done only when its acceptance checks pass and
the coordinator records the evidence. Unassigned tasks have no writer.

## Agent handoff rules

1. The coordinator records a task ID, agent, branch/worktree, exact file claim,
   and start time before dispatch. Overlapping file claims block dispatch.
2. An agent reads only its task, relevant contracts, and source. New work belongs
   in a separate worktree. Agents do not edit this board or merge other branches.
3. Shared files include `go.mod`, `go.sum`, `cmd/ble-watch/main.go`, API routing,
   store migrations, and CI configuration. The coordinator serializes changes to
   these files and lands contract changes before dependent work begins.
4. An agent stops before expanding a claim. It returns required changes and
   reasons to the coordinator. It never silently moves logic into another service.
5. Completion evidence includes commit, changed paths, commands/results,
   environment (synthetic or live), and remaining limitations. The coordinator
   reviews the diff, records acceptance, and releases the claim.
6. Abandoned work is marked blocked with a checkpoint before reassignment. A
   second writer must not infer that a quiet agent has released a claim.

## Release tasks

Current writer: release coordinator (root agent), working on `master` in the
existing BLE Watch checkout. The test worker has finished and released its file.
The coordinator is the only remaining writer. Release task claims below are
sequential; REL-04 cannot edit the article until REL-03 is accepted. Future
workers must record branch/worktree and UTC claim time when dispatched.

- **REL-01: public snapshot and dependency.** Status: active. Owner: release
  coordinator. Files: `go.mod`, `go.sum`, `.gitignore`, `README.md`, `ENDPOINTS.md`,
  `.github/workflows/checks.yml`, package manifests, `playwright.config.ts`,
  `tests/ble-watch.spec.ts`, `docs/TESTING.md`, release script and application
  cleanup files already staged by the coordinator. No worker may claim these
  until publication is complete.
  Acceptance: sanitized source and commit metadata, reproducible dependency,
  clean-checkout build, public repository link, no private history uploaded.
- **REL-02: test-agent smoke implementation.** Status: done. Owner: test worker;
  claim released after review. Coordinator owns scheduling and public test
  packaging under REL-01. Evidence: 10 desktop/mobile checks passed; coverage-agent manual
  runner passed; hourly filename-based schedule registered. No timer firing is
  claimed yet. Acceptance: API and real dashboard
  checks pass, failure artifacts do not capture live identities, recurring
  schedule is registered, and its first run has a recorded result.
- **REL-03: verification and writing audit.** Status: active. Owner: release
  coordinator. Files: `docs/VERIFICATION.md`, `docs/BLOG_DRAFT.md`, public prose,
  privacy check, integration tests. Acceptance: separate privacy and prose
  findings recorded, build passes, database integration evidence exists, live
  hardware result is reported accurately, no emoji or em dashes in owned text.
- **REL-04: article and screenshots.** Status: blocked on REL-03, REP-01 and UI-01.
  Owner: unassigned. Files: `docs/BLOG_DRAFT.md`, later `docs/images/`.
  Acceptance: screenshots use synthetic data, every result links to evidence,
  article reads as a first-person project account without unimplemented claims.
  Publishing a separate blog is a later release action.

## Implementation tasks

- **HW-01: establish continuous live reception.** Status: ready. Owner:
  unassigned. Claim to be assigned after checking receiver status. Acceptance:
  controlled owned beacon causes new observations and database writes after
  startup; loss of receiver coverage is distinguishable from quiet devices.
  Evidence so far: five startup observations persisted, but a separate
  45-second streaming interval saw no new samples. Do not declare hardware
  integration complete on the strength of HTTP health or cached device reads.

- **REP-01: source and clock contracts.** Status: ready after REL-01.
  Owner: unassigned; coordinator integrates shared entrypoint changes.
  Paths: new `internal/replay/`, scanner source contract, fixture definitions.
  Acceptance: isolated replay without an adapter, fixed clock and observation
  IDs, same results at different replay speeds. No live database writes.
- **REP-02: baseline measurements.** Status: blocked on REP-01.
  Owner: unassigned. Paths: replay fixtures and benchmark/report scripts.
  Acceptance: known labels are evaluation-only, baseline output plus CPU,
  memory, storage, job counts, and lag recorded with the command and revision.
- **DATA-01: durable observation and work contract.** Status: blocked on REP-01.
  Owner: unassigned. Paths: store migrations, new intake/work persistence code.
  Acceptance: bounded intake, transactional observation/work revisions,
  explicit overflow counters, rollback-compatible additive migrations.
- **TASK-01: reconciliation dispatcher.** Status: blocked on DATA-01.
  Owner: unassigned. Paths: `internal/ingest/`, shared main wiring by coordinator.
  Acceptance: pending jobs survive restart, transient errors retry, permanent
  failures become visible, stale workers cannot overwrite newer revisions.
- **STATE-01: receiver and device presence.** Status: blocked on DATA-01 and
  TASK-01. Owner: unassigned. Paths: new presence package and its tests.
  Acceptance: unknown/observed/stale/not-observed transitions, coverage-aware
  silence, late-input policy, obsolete delayed checks do not cause departures.
- **RULE-01: explained anomalies.** Status: blocked on STATE-01 and REP-02.
  Owner: unassigned. Paths: anomaly package and evidence records; coordinator
  lands any shared store changes first. Acceptance: tested stationary-device
  silence and sustained signal change, warmup, recovery, suppression, evidence.
- **RELATE-01: relationship evidence.** Status: blocked on STATE-01 and REP-02.
  Owner: unassigned. Paths: new relationship package and fixtures.
  Acceptance: repeated encounters support tentative links, constant overlap is
  discounted, contradictory evidence is visible, candidate work is bounded.
- **PERSON-01: optional association projection.** Status: blocked on STATE-01.
  Owner: unassigned. Paths: new association/presence projection and tests.
  Acceptance: user-confirmed ownership only, phone-left-behind fixture,
  uncertain coverage, no automatic attribution of strangers to people.
- **UI-01: timeline and evidence views.** Status: blocked on STATE-01 and
  RULE-01. Owner: unassigned. Paths: templates, generated templates, page handlers.
  Coordinator reserves routes. Acceptance: real browser checks in test-agent,
  synthetic screenshot fixture, evidence links, accessible dark/light behavior.
- **UI-02: relationship graph.** Status: blocked on UI-01 and RELATE-01.
  Owner: unassigned. Paths: graph view and page handler. Acceptance: tentative
  versus confirmed edges, inspectable evidence, no claim to network topology.
- **OPS-01: outbox and shutdown.** Status: blocked on DATA-01 and TASK-01.
  Owner: unassigned. Paths: alert delivery, shutdown coordination, outbox store.
  Acceptance: stable event IDs, restart recovery, bounded drain, explicit
  immediate/delayed submission failures, documented at-least-once delivery.
- **TEST-01: milestone automation.** Status: blocked on REP-01; starts after
  REL-02. Owner: unassigned. Paths: test-agent BLE specs and synthetic fixtures.
  Acceptance: state, anomaly, relationship, and failure tests added with their
  feature tasks; hardware checks are separately labeled and can report blocked.
  TEST-01 owns both public and workspace spec copies. UI and domain workers
  return required scenarios to this owner; they do not concurrently edit specs.
  The coordinator lands feature contracts before dispatching their test changes.
- **PROOF-01: value comparison.** Status: blocked on REP-02, TASK-01, RULE-01,
  and OPS-01. Owner: unassigned. Paths: benchmark results and release report.
  Acceptance: baseline comparison quantifies collapsed work and recovery;
  accuracy claims include labels and limitations; article updated with results.

## Checkpoint

The initial Witchcraft observation submission integration is implemented. The
larger intelligence pipeline is still proposed. Release verification and public
links are recorded in `VERIFICATION.md`; they do not imply future tasks passed.
