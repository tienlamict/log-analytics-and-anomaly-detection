---
phase: 02-detection-engine
plan: "05"
subsystem: testing
tags: [go, testify, goleak, race-detector, unit-tests, integration-tests, sliding-window, detection-rules, anomaly-detection]

# Dependency graph
requires:
  - phase: 02-detection-engine/02-02
    provides: sliding window and all seven detection rule implementations
  - phase: 02-detection-engine/02-03
    provides: DetectorEngine with cooldown, warmup, eviction, and silence watcher
  - phase: 02-detection-engine/02-04
    provides: DetectionConfig, YAML integration, and production wiring

provides:
  - Comprehensive unit tests for slidingWindow (5 cases)
  - Table-driven unit tests for all seven detection rules (25+ cases)
  - Engine integration tests: dispatch, cooldown, warmup suppression, eviction, stop
  - goleak.VerifyTestMain validating no goroutine leaks at package level
  - Race-clean test suite verified with -race flag

affects: [05-integration-and-hardening, phase-03-storage-and-alerting]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - atomic.Int64 counters in mock types to eliminate test-goroutine data races
    - drainAnomalies helper for channel-based timing tests
    - minimalCfg() helper for bypassing warmup in engine tests (WindowDuration=0)
    - t.Cleanup(engine.Stop) pattern for goroutine leak prevention in engine tests

key-files:
  created:
    - internal/detection/window_test.go
    - internal/detection/rules_test.go
    - internal/detection/engine_test.go
  modified:
    - internal/detection/engine.go

key-decisions:
  - "atomic.Int64 for mock counters (not sync.Mutex) — avoids secondary lock hierarchy and simpler for counters"
  - "cooldownMu sync.Mutex added to DetectorEngine to fix pre-existing data race between Evaluate/silenceWatcher (writers) and evictLoop (iterator) on the cooldowns map"
  - "WindowDuration=0 in minimalCfg() bypasses warmup — warmupUntil = Now+2*0 = Now (already past)"
  - "Only one TestMain per package — placed in window_test.go; goleak.VerifyTestMain called at package level"

patterns-established:
  - "Engine test pattern: create engine, defer/Cleanup Stop, assert via drainAnomalies helper"
  - "Mock detectors use atomic counters for cross-goroutine safe assertions"
  - "Rules tests use makeEntry/makeEntryAt helpers to minimise boilerplate"

requirements-completed: [DETECT-01, DETECT-02, DETECT-03, DETECT-04, DETECT-05, DETECT-06, DETECT-07, DETECT-08, DETECT-09, DETECT-10]

# Metrics
duration: 25min
completed: 2026-03-21
---

# Phase 02 Plan 05: Detection Engine Unit and Integration Tests Summary

**Comprehensive test suite for all seven detection rules plus engine dispatch, cooldown, warmup, and eviction — all passing with -race and goleak**

## Performance

- **Duration:** 25 min
- **Started:** 2026-03-21T15:46:40Z
- **Completed:** 2026-03-21T16:11:00Z
- **Tasks:** 2
- **Files modified:** 4 (3 created, 1 modified)

## Accomplishments
- 5 slidingWindow unit tests covering Add, CountWithin, circular overwrite, Evict, and empty state
- 25+ table-driven rule tests: all seven rules have firing, non-firing, and edge-case coverage
- 6 engine integration tests covering all DETECT-01 through DETECT-10 requirements
- Fixed pre-existing data race in `engine.go` (cooldowns map accessed from 3 goroutines without synchronisation)
- Full project test suite passes with -race flag and no goroutine leaks

## Task Commits

Each task was committed atomically:

1. **Task 1: Sliding window unit tests and rule unit tests for all seven rules** - `2c35c8d` (test)
2. **Task 2: Engine integration tests for dispatch, cooldown, warm-up, and eviction** - `ecb6317` (test + fix)

**Plan metadata:** _(to be added in final commit)_

## Files Created/Modified
- `internal/detection/window_test.go` - 5 slidingWindow unit tests, TestMain with goleak.VerifyTestMain
- `internal/detection/rules_test.go` - Table-driven tests for all 7 rules: ErrorRate, Latency, RepeatedFailure, AuthBurst, OffHours, ServiceSilence
- `internal/detection/engine_test.go` - mockDetector with atomic counters, drainAnomalies helper, 6 engine integration tests
- `internal/detection/engine.go` - Added cooldownMu sync.Mutex; refactored cooldowns map access to be goroutine-safe

## Decisions Made
- Used `atomic.Int64` for mock `evaluated` and `resetCount` fields — the evictLoop goroutine calls `Reset()` concurrently with test assertions, requiring thread-safe counter reads
- Set `WindowDuration=0` in `minimalCfg()` to bypass warmup — `warmupUntil = time.Now() + WarmupMultiplier * 0 = time.Now()` (already in the past)
- Placed `TestMain` only in `window_test.go` — one TestMain per package restriction; goleak.VerifyTestMain validates all tests in the package

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test logic error in TestLatencyThresholdRule_DoesNotFireBelowRate**
- **Found during:** Task 1 (rules unit tests)
- **Issue:** Test added 3 breaching entries first (100% breach rate at that point), triggering assertions before the 7 non-breaching entries could dilute the rate
- **Fix:** Reordered entries — 7 non-breaching first, then 3 breaching, so final rate = 30% (below 80% threshold)
- **Files modified:** internal/detection/rules_test.go
- **Verification:** Test passes, assertion checks per evaluate call
- **Committed in:** 2c35c8d (Task 1 commit)

**2. [Rule 1 - Bug] Fixed data race in engine.go cooldowns map**
- **Found during:** Task 2 (engine integration tests with -race flag)
- **Issue:** `e.cooldowns` map was written by `Evaluate()` (pipeline goroutine) and `silenceWatcher()` (heartbeat goroutine), while simultaneously iterated/deleted by `evictLoop()` (eviction goroutine) — no synchronisation
- **Fix:** Added `cooldownMu sync.Mutex` to `DetectorEngine`; wrapped all reads and writes to `e.cooldowns` in Lock/Unlock in all three goroutine paths
- **Files modified:** internal/detection/engine.go
- **Verification:** `go test -race ./internal/detection/... -count=1` passes with zero race warnings
- **Committed in:** ecb6317 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 - Bug)
**Impact on plan:** Both fixes were required for correctness. First fix: test logic. Second fix: pre-existing production race condition that the -race tests surfaced. No scope creep.

## Issues Encountered
- The comment in engine.go (Plan 02-03 decision log) said "channel coordination deferred to Plan 02-04 if race detector fires" — the race detector fired in Plan 02-05 tests instead. Fixed immediately as Rule 1 bug.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All DETECT-01 through DETECT-10 requirements covered by automated tests
- Detection package is race-clean; safe to use in Phase 5 integration tests
- Full project test suite passes: kafka, metrics, pipeline, detection packages all green

## Self-Check: PASSED

- FOUND: internal/detection/window_test.go
- FOUND: internal/detection/rules_test.go
- FOUND: internal/detection/engine_test.go
- FOUND: commit 2c35c8d (Task 1)
- FOUND: commit ecb6317 (Task 2)

---
*Phase: 02-detection-engine*
*Completed: 2026-03-21*
