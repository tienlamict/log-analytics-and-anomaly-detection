---
phase: 02-detection-engine
plan: "03"
subsystem: detection
tags: [go, anomaly-detection, sliding-window, timezone, auth, security, heartbeat, goroutine]

# Dependency graph
requires:
  - phase: 02-detection-engine/02-01
    provides: "DetectorEngine, slidingWindow, config structs (AuthBurstConfig, OffHoursConfig, ServiceSilenceConfig)"
provides:
  - "AuthFailureBurstRule: per-IP/user sliding-window auth failure burst detector"
  - "OffHoursAccessRule: stateless timezone-aware off-hours sensitive path access detector"
  - "ServiceSilenceRule: adapter-pattern service heartbeat loss detector with cold-start guard"
  - "DetectorEngine.silenceWatcher: heartbeat goroutine for silence rule integration"
affects:
  - "02-04-PLAN (channel coordination and integration wiring)"
  - "02-05-PLAN (unit tests for these three rules)"
  - "03-storage-alerting (consumers of Anomaly channel)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Adapter pattern for absence-based detection: Evaluate updates state, separate CheckSilence produces anomalies"
    - "Cold-start guard: MinLogCount threshold before tracking begins to avoid false positives"
    - "Per-key sliding window: map[string]*slidingWindow keyed by IP or username for auth burst"
    - "Timezone-aware stateless rule: time.LoadLocation + entry.Timestamp.In(loc).Hour()"
    - "Cross-goroutine rule state protection: sync.Mutex only where required (pipeline vs heartbeat)"

key-files:
  created:
    - internal/detection/auth_burst.go
    - internal/detection/off_hours.go
    - internal/detection/service_silence.go
  modified:
    - internal/detection/engine.go

key-decisions:
  - "ServiceSilenceRule uses adapter pattern: Evaluate never fires, CheckSilence called by heartbeat goroutine via silenceWatcher"
  - "lastSeen tracks ingest time (time.Now()) not entry.Timestamp — silence detection requires wall-clock time not event time"
  - "Only ServiceSilenceRule needs sync.Mutex — all other rules satisfy single-goroutine-caller contract"
  - "silenceWatcher respects engine warmupUntil and cooldown map — consistent suppression policy with evictLoop"
  - "AuthFailureBurstRule prefers IP field over username field for keying — IP is more reliable for burst detection"

patterns-established:
  - "Stateless rules (OffHoursAccessRule): no window, no map, Reset is a no-op"
  - "Stateful per-key rules (AuthFailureBurstRule): map[string]*slidingWindow, Reset evicts per key"
  - "Absence-based rules (ServiceSilenceRule): adapter pattern with heartbeat goroutine"

requirements-completed: [DETECT-05, DETECT-06, DETECT-07]

# Metrics
duration: 5min
completed: 2026-03-21
---

# Phase 2 Plan 03: Auth Burst, Off-Hours, and Service Silence Detection Rules

**Three security and availability detection rules: per-IP/user auth failure burst via sliding window, timezone-aware off-hours sensitive path access (stateless), and adapter-pattern service heartbeat loss with cold-start guard and engine heartbeat goroutine.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-21T15:33:00Z
- **Completed:** 2026-03-21T15:36:46Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- AuthFailureBurstRule tracks per-IP/user auth failure counts in a sliding window and fires when the count exceeds the configured threshold within the window duration
- OffHoursAccessRule fires immediately when an event's path matches a sensitive path prefix and its timestamp falls outside business hours, with full timezone support via time.LoadLocation
- ServiceSilenceRule uses the adapter pattern: Evaluate updates tracking state (never fires), CheckSilence is called by the engine's heartbeat goroutine to detect services that have gone silent past the threshold; a cold-start guard (MinLogCount) prevents false positives on new services
- DetectorEngine extended with silenceWatcher heartbeat goroutine that applies existing warmup suppression and cooldown logic consistently

## Task Commits

Each task was committed atomically:

1. **Task 1: AuthFailureBurstRule and OffHoursAccessRule** - `efc3389` (feat)
2. **Task 2: ServiceSilenceRule with heartbeat and engine integration** - `47080b3` (feat)

## Files Created/Modified

- `internal/detection/auth_burst.go` - AuthFailureBurstRule: per-key sliding window for auth failure bursts
- `internal/detection/off_hours.go` - OffHoursAccessRule: stateless timezone-aware off-hours detector
- `internal/detection/service_silence.go` - ServiceSilenceRule: adapter pattern heartbeat loss detector with sync.Mutex
- `internal/detection/engine.go` - Added silenceRule field, silenceWatcher goroutine, type-assertion on constructor

## Decisions Made

- **Adapter pattern for silence rule:** Absence of events cannot be detected in Evaluate (which is event-driven). CheckSilence polls state — called by a dedicated heartbeat goroutine in the engine so anomalies flow through the same output channel as all other rules.
- **Ingest time vs event time for lastSeen:** ServiceSilenceRule tracks time.Now() (not entry.Timestamp) because silence detection depends on wall-clock elapsed time since last ingestion, not the timestamp embedded in the log event.
- **Only one mutex in the package:** sync.Mutex is used only in ServiceSilenceRule because it is the only rule where goroutine boundaries cross. All other rules satisfy the single-goroutine-caller contract from Plan 02-01.
- **silenceWatcher applies engine cooldown:** The heartbeat goroutine applies the same cooldown map and warmupUntil check as the main Evaluate path to ensure consistent suppression behavior across all anomaly types.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All seven detection rules are now implemented (error rate, latency breach, repeated failure from 02-02; auth burst, off-hours access, service silence from this plan)
- Ready for Plan 02-04 (channel coordination, pipeline wiring) and 02-05 (unit tests for these rules)
- The silenceWatcher goroutine and evictLoop share the cooldowns map without a mutex — this is acceptable under the single-goroutine-caller contract but should be revisited if race detector fires in 02-05 tests

---
*Phase: 02-detection-engine*
*Completed: 2026-03-21*
