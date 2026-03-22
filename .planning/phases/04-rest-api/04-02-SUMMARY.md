---
phase: 04-rest-api
plan: "02"
subsystem: api
tags: [go, chi, elasticsearch, http, pagination, query-params]

# Dependency graph
requires:
  - phase: 04-rest-api/04-01
    provides: Server struct, PagedResponse, writeJSON/writeError helpers, domain.ErrNotFound, LogStore/AnomalyStore interfaces, chi router setup

provides:
  - GET /api/v1/logs paginated list handler with service/level/time-range filters
  - GET /api/v1/logs/{id} single log entry handler with 404 discrimination
  - parseIntParam and parseTimeParam query-param helpers

affects: [04-03-anomalies, 05-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "parseIntParam with clamp: bound page/size to [min, max] ranges before store call"
    - "errors.Is(err, domain.ErrNotFound) for 404 discrimination in handlers"
    - "r.Context() propagation: all store calls receive the request context for ES timeout"

key-files:
  created:
    - internal/api/logs.go
  modified:
    - internal/api/server.go

key-decisions:
  - "Default page=1, size=20; max size clamped to 1000; max page clamped to 10000 to prevent unbounded scans"
  - "parseIntParam clamps (does not reject) values outside range, then returns clamped value — invalid non-integer strings still return 400"
  - "Empty service/level params passed through to LogStore (store decides whether to filter)"

patterns-established:
  - "Query-param parsing with parseIntParam/parseTimeParam: consistent error messages, clamp strategy, zero-value passthrough"
  - "404 discrimination pattern: errors.Is(err, domain.ErrNotFound) before generic 500 fallback"

requirements-completed: [API-01, API-02]

# Metrics
duration: 1min
completed: 2026-03-22
---

# Phase 4 Plan 02: Log Query Handlers Summary

**Paginated GET /api/v1/logs with service/level/time-range filters and GET /api/v1/logs/{id} with ErrNotFound-based 404 discrimination**

## Performance

- **Duration:** 1 min
- **Started:** 2026-03-22T04:04:33Z
- **Completed:** 2026-03-22T04:05:18Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- handleListLogs parses service, level, from (RFC3339), to (RFC3339), page (default 1, max 10000), size (default 20, max 1000) and returns PagedResponse envelope
- handleGetLog extracts `id` via chi.URLParam and returns 404 JSON for domain.ErrNotFound, 500 for other errors
- parseIntParam and parseTimeParam helpers provide consistent error messages for bad query params (400)
- Both handlers pass r.Context() to store calls for Elasticsearch timeout propagation
- Routes registered on chi sub-router under /api/v1

## Task Commits

Each task was committed atomically:

1. **Task 1: Log list and detail handlers** - `552a1cb` (feat)

## Files Created/Modified

- `internal/api/logs.go` - handleListLogs, handleGetLog, parseIntParam, parseTimeParam
- `internal/api/server.go` - Registered GET /logs and GET /logs/{id} routes in /api/v1 block

## Decisions Made

- Default page=1, size=20 (sensible defaults for interactive queries); max size=1000, max page=10000 to bound ES scan depth
- parseIntParam clamps out-of-range ints rather than rejecting them, but non-integer strings return 400
- Empty service/level query params are passed as empty strings to LogStore — the store layer decides how to handle absent filters

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- GET /api/v1/logs and GET /api/v1/logs/{id} are fully implemented and compiled
- Ready for 04-03: anomaly query handlers (GET /api/v1/anomalies, GET /api/v1/anomalies/{id})
- chi sub-router TODO comment for anomalies still present in server.go (correct — 04-03 will fill it)

---
*Phase: 04-rest-api*
*Completed: 2026-03-22*
