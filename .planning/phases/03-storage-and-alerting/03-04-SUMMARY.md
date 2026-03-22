---
phase: 03-storage-and-alerting
plan: 04
subsystem: alerting
tags: [go, dispatcher, fan-out, anomaly, elasticsearch, smtp, concurrency]

# Dependency graph
requires:
  - phase: 03-02
    provides: AnomalyIndexer implementing domain.AnomalyStore (IndexAnomaly)
  - phase: 03-03
    provides: SMTPAlerter implementing domain.AlertChannel (Send)
provides:
  - Dispatcher struct with NewDispatcher constructor and Run method
  - Fan-out from anomaly channel to AnomalyStore + AlertChannel with error isolation
  - Unit test coverage for fan-out, error independence, shutdown, multi-anomaly dispatch
affects: [04-rest-api, 05-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Fan-out select loop: read from channel, call both outputs regardless of each other's error"
    - "Error isolation: log error from either output and continue dispatching — no early return"
    - "Clean exit on channel close (ok==false returns nil); ctx.Done returns ctx.Err()"

key-files:
  created:
    - internal/alert/dispatcher.go
    - internal/alert/dispatcher_test.go
  modified: []

key-decisions:
  - "No cooldown in Dispatcher — cooldown is handled upstream by DetectorEngine; Anomalies() channel emits only post-cooldown events"
  - "Both IndexAnomaly and Send called unconditionally per anomaly — failure in one must not suppress the other"
  - "Run returns nil on channel close, ctx.Err() on cancellation — caller distinguishes clean vs cancelled shutdown"

patterns-established:
  - "Fan-out pattern: call each output in sequence; log+continue on error; no early return"
  - "Mock interfaces in test file with separate err field per output to test partial failure scenarios"

requirements-completed: [ALERT-01, STORE-01, STORE-02]

# Metrics
duration: 2min
completed: 2026-03-22
---

# Phase 3 Plan 04: Alert Dispatcher Summary

**Dispatcher fan-out that independently delivers each anomaly to both AnomalyStore (Elasticsearch) and AlertChannel (SMTP) with per-output error isolation and clean goroutine lifecycle**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-22T02:04:23Z
- **Completed:** 2026-03-22T02:05:58Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Implemented `Dispatcher` struct with `NewDispatcher` and `Run` in `internal/alert/dispatcher.go`
- Fan-out loop calls `IndexAnomaly` then `Send` per anomaly; errors in either are logged (anomaly_id + rule_id context) and processing continues
- Clean exit on channel close (returns nil); context cancellation returns `ctx.Err()`
- 6 unit tests covering: both-receive, index failure, alert failure, channel closed, context cancelled, multiple anomalies — all pass under `-race`

## Task Commits

Each task was committed atomically:

1. **Task 1: Dispatcher fan-out with error isolation and tests** - `5e406f6` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `internal/alert/dispatcher.go` - Dispatcher struct with fan-out Run loop, no cooldown logic
- `internal/alert/dispatcher_test.go` - 6 unit tests with mock AnomalyStore and mock AlertChannel

## Decisions Made

- No cooldown in Dispatcher: cooldown is already handled by `DetectorEngine` upstream; the `Anomalies()` channel only emits post-cooldown events; adding it again would suppress legitimate re-triggers
- Both outputs unconditional: `IndexAnomaly` and `Send` are always called for each anomaly regardless of each other's result; this is the core correctness invariant of the fan-out
- `Run` returns `nil` on closed channel and `ctx.Err()` on cancellation: gives callers precise shutdown classification

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `Dispatcher` is ready to be wired in Phase 4/5 alongside `DetectorEngine`, `AnomalyIndexer`, and `SMTPAlerter`
- All three fan-out participants (engine + indexer + alerter) are implemented; only the top-level `main.go` wiring and REST API remain
- No blockers

---
*Phase: 03-storage-and-alerting*
*Completed: 2026-03-22*
