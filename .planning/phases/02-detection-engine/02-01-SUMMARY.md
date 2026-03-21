---
phase: 02-detection-engine
plan: "01"
subsystem: detection
tags: [go, anomaly-detection, sliding-window, prometheus, zap, mapstructure]

# Dependency graph
requires:
  - phase: 01-foundation-and-ingestion
    provides: domain.Detector interface, domain.LogEntry and domain.Anomaly types, metrics.AnomaliesDetectedTotal counter
provides:
  - DetectorEngine with registry dispatch, warmup suppression, cooldown map, and eviction goroutine
  - slidingWindow circular buffer with Add, CountWithin, Evict methods
  - DetectionConfig struct tree with 6 rule config types and mapstructure tags
affects:
  - 02-02 (error-rate, latency, repeated-failure rule implementations)
  - 02-03 (auth-burst, off-hours, service-silence rule implementations)
  - 02-04 (YAML config wiring and pipeline integration)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Single-goroutine-caller contract on DetectorEngine.Evaluate avoids mutex overhead
    - Non-blocking channel send with zap warning drop on full anomaly channel
    - Warmup suppression via time.Before(warmupUntil) prevents false positives at startup
    - Cooldown map keyed by (ruleID, service) pair prevents alert storms
    - maxWindowDuration helper derives warmup period from all rule window durations

key-files:
  created:
    - internal/detection/config.go
    - internal/detection/window.go
    - internal/detection/engine.go
  modified: []

key-decisions:
  - "Single-goroutine-caller contract on Evaluate: no mutex needed since pipeline worker is single-goroutine; evictLoop only calls d.Reset() on detectors"
  - "Non-monotonic scan in CountWithin: Kafka out-of-order delivery means we cannot short-circuit on first old timestamp"
  - "Warmup multiplier defaults to 2 if cfg.WarmupMultiplier <= 0, ensuring at least 2x longest window before emitting anomalies"
  - "Cooldown eviction at 2x cooldownDur: prevents unbounded growth of cooldowns map without prematurely evicting live entries"

patterns-established:
  - "slidingWindow: fixed-capacity circular buffer with head/count pointers, no container/ring to avoid interface{} casts"
  - "evictLoop pattern: ticker-based goroutine calling d.Reset() on all detectors, stopped via close(evictStop)"
  - "Non-blocking anomaly channel send: select with default that logs a zap warning when channel full"

requirements-completed: [DETECT-01, DETECT-10]

# Metrics
duration: 2min
completed: 2026-03-21
---

# Phase 2 Plan 1: Detection Engine Core Summary

**DetectorEngine with warmup suppression, per-(rule,service) cooldown map, sliding-window circular buffer, and background eviction goroutine — foundation for all 6 detection rules**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-21T15:29:58Z
- **Completed:** 2026-03-21T15:31:58Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- DetectorEngine dispatches Evaluate to all registered domain.Detector implementations per LogEntry, with warmup suppression and per-(rule,service) cooldown
- slidingWindow circular buffer tracks event timestamps with Add, CountWithin (full-scan for out-of-order Kafka safety), and Evict methods
- DetectionConfig struct tree defines DetectionConfig, DetectionRulesConfig, and 6 per-rule config types (ErrorRate, Latency, RepeatedFailure, AuthBurst, OffHours, ServiceSilence) with mapstructure tags for Viper YAML loading

## Task Commits

Each task was committed atomically:

1. **Task 1: DetectionConfig struct tree and slidingWindow helper** - `3969810` (feat)
2. **Task 2: DetectorEngine with registry dispatch, cooldown, and eviction goroutine** - `3c41d9f` (feat)

## Files Created/Modified

- `internal/detection/config.go` - DetectionConfig + 6 per-rule config structs with mapstructure tags
- `internal/detection/window.go` - slidingWindow circular buffer with Add, CountWithin, Evict
- `internal/detection/engine.go` - DetectorEngine with warmup, cooldown, anomaly channel, eviction goroutine

## Decisions Made

- Single-goroutine-caller contract on `Evaluate`: plan explicitly avoids a mutex on the engine since the pipeline worker is a single goroutine; evictLoop only calls `d.Reset()` which each rule handles safely. Race detector findings, if any, will be resolved with channel-based coordination in Plan 02-04.
- Non-monotonic scan in `CountWithin`: Kafka delivery is not guaranteed monotonic, so the loop scans all entries rather than short-circuiting at first old timestamp.
- Warmup multiplier defaults to 2 when `cfg.WarmupMultiplier <= 0` to guard against zero-value config.
- Cooldown entries evicted at 2x `cooldownDur`: avoids unbounded map growth while keeping entries live long enough to suppress burst re-fires.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/detection/` package compiles and vets cleanly (`go build ./internal/detection/...`, `go vet ./internal/detection/...`)
- `domain.Detector` interface contract fulfilled by engine dispatch loop — Plans 02-02 and 02-03 can now implement rule detectors and register them with `NewDetectorEngine`
- `DetectionConfig` struct tree ready for YAML population in Plan 02-04

---
*Phase: 02-detection-engine*
*Completed: 2026-03-21*

## Self-Check: PASSED

- FOUND: internal/detection/config.go
- FOUND: internal/detection/window.go
- FOUND: internal/detection/engine.go
- FOUND: .planning/phases/02-detection-engine/02-01-SUMMARY.md
- FOUND commit: 3969810 (Task 1)
- FOUND commit: 3c41d9f (Task 2)
