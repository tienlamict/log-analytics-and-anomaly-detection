---
phase: 01-foundation-and-ingestion
plan: 03
subsystem: pipeline
tags: [go, parsing, json, prometheus, zap, uuid]

# Dependency graph
requires:
  - phase: 01-01
    provides: domain.RawMessage and domain.LogEntry types, domain.Parser interface
  - phase: 01-04
    provides: metrics.ParseErrorsTotal counter for metering parse errors
provides:
  - Parse(RawMessage) -> (LogEntry, error) pure function in internal/pipeline/parser.go
  - NormaliseLevel(raw string) -> string covering all documented level variants
  - LogParser struct satisfying domain.Parser interface
  - ProcessMessage(RawMessage, *zap.Logger) -> LogEntry wiring parse errors to metrics and logging
affects:
  - 02-detection-engine
  - 05-integration-and-hardening

# Tech tracking
tech-stack:
  added:
    - github.com/google/uuid v1.6.0 (ID generation for LogEntry)
  patterns:
    - Pure function parser: Parse is a stateless function; LogParser struct wraps it to satisfy interface
    - Never-drop pattern: Parse always returns a valid LogEntry, error path is additive not destructive
    - Metrics wiring at call site: ProcessMessage owns the increment/log side-effects, keeping Parse pure

key-files:
  created:
    - internal/pipeline/parser.go
    - internal/pipeline/worker.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Parse is a pure package-level function; LogParser is a thin struct wrapper for interface satisfaction — keeps testing simple"
  - "ProcessMessage owns metrics.ParseErrorsTotal.Inc() — parser stays pure, wiring is explicit at call site"
  - "uuid.NewString() for LogEntry ID generation — no dependency on Kafka offset for uniqueness"

patterns-established:
  - "Never-drop: Parse returns (LogEntry, error) where LogEntry is always valid even on error"
  - "Level normalisation via NormaliseLevel applied at parse time so downstream rules use canonical values only"
  - "Call-site wiring: metrics and logging side-effects live in ProcessMessage, not in Parse"

requirements-completed: [PARSE-01, PARSE-02, PARSE-03, PARSE-04]

# Metrics
duration: 5min
completed: 2026-03-21
---

# Phase 1 Plan 3: Log Parser Summary

**Pure-function JSON/plain-text log parser with canonical level normalisation and metrics-wired ProcessMessage call site**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-21T08:05:40Z
- **Completed:** 2026-03-21T08:10:00Z
- **Tasks:** 2
- **Files modified:** 4 (parser.go, worker.go, go.mod, go.sum)

## Accomplishments
- Parse(RawMessage) -> (LogEntry, error) handles structured JSON and plain-text fallback, never drops a message
- NormaliseLevel covers all 12 documented variants: ERROR/ERR/FATAL/CRITICAL/CRIT -> error, WARN/WARNING -> warn, INFO/INFORMATION -> info, DEBUG/TRACE/VERBOSE -> debug, default -> unknown
- LogParser struct satisfies domain.Parser interface via thin wrapper
- ProcessMessage wires parse errors to metrics.ParseErrorsTotal.Inc() and zap.Logger.Warn with topic/partition/offset context (PARSE-04)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Parse and NormaliseLevel functions** - `6cfd3dc` (feat)
2. **Task 2: Implement ProcessMessage wiring Parse to metrics and logging** - `f600844` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `internal/pipeline/parser.go` - Parse function, NormaliseLevel, LogParser struct
- `internal/pipeline/worker.go` - ProcessMessage wiring metrics and Zap logging
- `go.mod` - Added github.com/google/uuid v1.6.0
- `go.sum` - Updated checksums

## Decisions Made
- Parse is implemented as a pure package-level function; LogParser is a thin struct wrapper added to satisfy domain.Parser interface without complicating the pure function.
- ProcessMessage owns all side-effects (metrics increment, warning log) so Parse remains testable in isolation.
- uuid.NewString() used for ID generation — IDs are independent of Kafka offset/partition, which is correct since an offset identifies position not entry identity.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added github.com/google/uuid dependency**
- **Found during:** Task 1 (parser.go creation)
- **Issue:** uuid package referenced in plan but not present in go.mod
- **Fix:** Ran `go get github.com/google/uuid@latest` — added v1.6.0
- **Files modified:** go.mod, go.sum
- **Verification:** go build ./internal/pipeline/... exits 0
- **Committed in:** 6cfd3dc (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — missing dependency)
**Impact on plan:** Required for correctness; uuid was explicitly specified by the plan. No scope creep.

## Issues Encountered
None beyond the missing uuid dependency handled as Rule 3.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Parser is ready for consumption by detection engine (Phase 02)
- ProcessMessage is the integration point — callers pass a RawMessage and receive a LogEntry; errors are already metered and logged
- domain.Parser interface is satisfied by LogParser; can be injected anywhere the interface is used

---
*Phase: 01-foundation-and-ingestion*
*Completed: 2026-03-21*
