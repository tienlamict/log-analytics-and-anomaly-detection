---
phase: 02-detection-engine
plan: "02"
subsystem: detection
tags: [golang, sliding-window, anomaly-detection, error-rate, latency, repeated-failure, regex, fingerprinting]

# Dependency graph
requires:
  - phase: 02-detection-engine/02-01
    provides: slidingWindow circular buffer, DetectionConfig structs, DetectorEngine scaffold

provides:
  - ErrorRateRule implementing domain.Detector — fires on per-service error count threshold
  - LatencyThresholdRule implementing domain.Detector — fires on breach rate percentage threshold
  - RepeatedFailureRule implementing domain.Detector — fires on repeated same-class error fingerprint

affects: [02-03, 02-04, integration-tests]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Per-service sliding window map pattern (map[string]*slidingWindow keyed by service name)
    - Composite key pattern for multi-dimensional tracking (service + "|" + fingerprint)
    - Entry timestamp used for window operations — NOT time.Now() — enabling testability with synthetic timestamps
    - Reset() evicts stale entries without clearing state — preserves recent events across eviction cycles
    - Package-level compiled regexp for message fingerprinting (fingerprintNoise)

key-files:
  created:
    - internal/detection/error_rate.go
    - internal/detection/latency.go
    - internal/detection/repeated_failure.go
  modified: []

key-decisions:
  - "Rules placed in internal/detection/ package (not a sub-package) to access unexported slidingWindow type directly"
  - "latencyWindow is a custom struct tracking both timestamp and breach bool per entry — slidingWindow is insufficient for breach rate math"
  - "fingerprint() uses ReplaceAllLiteralString (not ReplaceAllString) to treat the replacement X as a literal not a regexp reference"
  - "Reset() deletes zero-count map entries from RepeatedFailureRule to prevent unbounded growth for transient error fingerprints"

patterns-established:
  - "Rule files live in internal/detection/ package to share access to unexported slidingWindow"
  - "Evaluate() uses entry.Timestamp for all window Add() calls — never time.Now() for event recording"
  - "Reset() evicts stale entries via Evict() — never clears the entire windows map"
  - "Breach rate computed as integer percentage: (breaches * 100) / total"

requirements-completed: [DETECT-02, DETECT-03, DETECT-04]

# Metrics
duration: 2min
completed: 2026-03-21
---

# Phase 02 Plan 02: Detection Rules (Error Rate, Latency, Repeated Failure) Summary

**Three rule-based detectors using per-service sliding windows: error rate spike, latency breach rate, and regex-fingerprinted repeated failure — all implementing domain.Detector**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-21T15:34:42Z
- **Completed:** 2026-03-21T15:35:56Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- ErrorRateRule counts error-level log entries per service and fires when count >= threshold in window
- LatencyThresholdRule extracts duration_ms/latency_ms, tracks breach rates via custom latencyWindow, fires when breach percentage >= configured rate
- RepeatedFailureRule normalizes error messages via regex fingerprinting and fires when same error class repeats >= threshold times per service

## Task Commits

1. **Task 1: ErrorRateRule and LatencyThresholdRule** - `a6aa2e4` (feat)
2. **Task 2: RepeatedFailureRule with message fingerprinting** - `65b9a99` (feat)

## Files Created/Modified

- `internal/detection/error_rate.go` - ErrorRateRule with per-service slidingWindow tracking, fires on error count threshold
- `internal/detection/latency.go` - LatencyThresholdRule with custom latencyWindow tracking breach/total per entry, fires on breach rate percentage
- `internal/detection/repeated_failure.go` - RepeatedFailureRule with regexp fingerprinting, composite service+fingerprint window keys, fires on repeat count threshold

## Decisions Made

- Rules placed in `internal/detection/` (not `internal/detection/rules/`) to keep slidingWindow unexported — plan recognized this and prescribed the co-location approach
- A custom `latencyWindow` struct was needed for LatencyThresholdRule since the existing `slidingWindow` only tracks timestamps, not breach booleans per entry
- `fingerprint()` uses `ReplaceAllLiteralString` so the replacement string "X" is not interpreted as a regexp back-reference
- `Reset()` in RepeatedFailureRule deletes map entries with zero count after eviction to prevent unbounded map growth from transient error patterns

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All three rules implement `domain.Detector` and are ready to be wired into `DetectorEngine` from Plan 02-01
- Plans 02-03 (AuthBurstRule, OffHoursRule, ServiceSilenceRule) can proceed in parallel — same pattern established here
- Plan 02-04 can integrate all rules into the full pipeline

---
*Phase: 02-detection-engine*
*Completed: 2026-03-21*
