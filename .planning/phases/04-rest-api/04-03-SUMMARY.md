---
phase: 04-rest-api
plan: 03
subsystem: api
tags: [go, chi, elasticsearch, anomalies, rest-api, pagination]

# Dependency graph
requires:
  - phase: 04-01
    provides: Server struct, AnomalyStore interface, domain.ErrNotFound, writeJSON/writeError/PagedResponse helpers
  - phase: 04-02
    provides: parseIntParam/parseTimeParam helpers in logs.go (reused by anomalies.go)
provides:
  - GET /api/v1/anomalies handler with type/service/severity/from/to/page/size filtering
  - GET /api/v1/anomalies/{id} handler with 404 discrimination via domain.ErrNotFound
affects: [04-04, 05-integration]

# Tech tracking
tech-stack:
  added: []
  patterns: [chi URL param extraction, domain.ErrNotFound 404 discrimination, PagedResponse envelope, AnomalyQuery.Type maps to rule_id ES field]

key-files:
  created:
    - internal/api/anomalies.go
  modified:
    - internal/api/server.go

key-decisions:
  - "Reused parseIntParam/parseTimeParam from logs.go — 04-02 ran first so helpers already existed"
  - "AnomalyQuery.Type maps to rule_id in ES (established in 04-01); handler uses ?type= query param"

patterns-established:
  - "Anomaly handler pattern: parse params -> build query -> call store -> writeJSON/writeError"
  - "404 discrimination: errors.Is(err, domain.ErrNotFound) before generic 500"

requirements-completed: [API-03, API-04]

# Metrics
duration: 5min
completed: 2026-03-22
---

# Phase 4 Plan 03: Anomaly Query Handlers Summary

**GET /api/v1/anomalies with type/service/severity/time-range filters plus GET /api/v1/anomalies/{id} with 404 discrimination via domain.ErrNotFound**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-22T04:10:00Z
- **Completed:** 2026-03-22T04:15:00Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Implemented handleListAnomalies parsing type (rule_id), service, severity, from, to, page, size query params
- Implemented handleGetAnomaly with proper 404 vs 500 discrimination using errors.Is(err, domain.ErrNotFound)
- Registered both anomaly routes on the chi router in server.go alongside existing log routes

## Task Commits

Each task was committed atomically:

1. **Task 1: Anomaly list and detail handlers** - `d5faeb4` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `internal/api/anomalies.go` - handleListAnomalies and handleGetAnomaly HTTP handlers
- `internal/api/server.go` - registered GET /anomalies and GET /anomalies/{id} routes, removed TODO comment

## Decisions Made
- Reused parseIntParam/parseTimeParam helpers from logs.go since plan 04-02 had already run and created them — no need for a separate params.go file
- AnomalyQuery.Type field maps to rule_id in Elasticsearch (per 04-01 decision); the HTTP query param name is "type" for operator convenience

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Self-Check: PASSED

All files present and commit d5faeb4 verified.

## Next Phase Readiness
- All four /api/v1 routes are now registered: GET /logs, GET /logs/{id}, GET /anomalies, GET /anomalies/{id}
- Ready for 04-04 (integration wiring or remaining phase work)
- `go build ./internal/api/...` exits 0

---
*Phase: 04-rest-api*
*Completed: 2026-03-22*
