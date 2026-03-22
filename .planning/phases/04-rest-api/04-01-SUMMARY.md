---
phase: 04-rest-api
plan: 01
subsystem: api
tags: [go, chi, elasticsearch, prometheus, zap, rest-api]

requires:
  - phase: 03-storage-and-alerting
    provides: LogIndexer and AnomalyIndexer with BulkIndexer; TypedClient; domain.LogStore and AnomalyStore interfaces
  - phase: 01-foundation-and-ingestion
    provides: domain types (LogEntry, Anomaly, LogQuery, AnomalyQuery), config infrastructure

provides:
  - domain.ErrNotFound sentinel for 404 discrimination
  - APIConfig struct with port 8080 default in config.Config
  - internal/api package: Server struct, NewServer, chi router with middleware
  - GET /health (ES ping), GET /ready (atomic flag), GET /metrics (promhttp)
  - PagedResponse and ErrorResponse JSON helpers
  - LogIndexer.SearchLogs and GetLog implemented against logs-* with bool/term/range queries
  - AnomalyIndexer.SearchAnomalies and GetAnomaly implemented against anomalies index

affects:
  - 04-02 (log query handlers depend on Server.Router() and LogStore search/get)
  - 04-03 (anomaly query handlers depend on Server.Router() and AnomalyStore search/get)

tech-stack:
  added:
    - github.com/go-chi/chi/v5 v5.2.5
  patterns:
    - Chi sub-router for /api/v1 with middleware applied at router level
    - atomic.Bool for readiness gate (no mutex needed for simple bool)
    - IDs query for wildcard-index GetLog (Get API does not support logs-*)
    - DateRangeQuery with RFC3339 string Gte/Lte for time filters
    - Pagination clamping: page default 1, size default 20, max 1000

key-files:
  created:
    - internal/domain/errors.go
    - internal/api/response.go
    - internal/api/server.go
    - internal/api/middleware.go
    - internal/api/health.go
  modified:
    - internal/config/config.go
    - internal/elasticsearch/log_indexer.go
    - internal/elasticsearch/anomaly_indexer.go
    - go.mod
    - go.sum

key-decisions:
  - "IDs query (not Get API) for LogIndexer.GetLog because logs-* is a wildcard index that does not support the Get API"
  - "AnomalyQuery.Type maps to ES rule_id field (not type) per anomalies index mapping"
  - "detected_at field (not @timestamp) used for anomaly date range filter per anomalies mapping"
  - "atomic.Bool for Server.ready avoids lock overhead for a simple boolean gate"
  - "chi middleware.Timeout(30s) per-handler + 60s http.Server read/write timeouts for defense in depth"

patterns-established:
  - "writeJSON/writeError helpers used by all handlers for consistent JSON responses"
  - "zapRequestLogger middleware captures status via WrapResponseWriter and logs after handler returns"
  - "Pagination clamped at source in store methods, not in handlers"

requirements-completed: [API-05, API-06]

duration: 6min
completed: 2026-03-22
---

# Phase 4 Plan 1: REST API Foundation Summary

**Chi router with health/ready/metrics endpoints, zap request logging, ES search/get implementations for LogIndexer and AnomalyIndexer, and ErrNotFound sentinel — all foundation infrastructure for log and anomaly query handlers in plans 04-02 and 04-03.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-22T03:56:11Z
- **Completed:** 2026-03-22T04:02:32Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments

- REST API foundation compiled: `internal/api` package with Server struct, chi router, middleware stack, and infrastructure routes
- Replaced ES stub implementations: LogIndexer.SearchLogs/GetLog and AnomalyIndexer.SearchAnomalies/GetAnomaly now execute real queries against Elasticsearch
- Added domain.ErrNotFound sentinel and APIConfig with port 8080 default to enable uniform 404 responses and configuration-driven port binding

## Task Commits

Each task was committed atomically:

1. **Task 1: Domain sentinel, API config, chi dependency, response helpers** - `7609085` (feat)
2. **Task 2: ES search/get implementations for LogIndexer and AnomalyIndexer** - `7d24acb` (feat)
3. **Task 3: Chi router, middleware, health/ready/metrics handlers** - `9acb70a` (feat)

## Files Created/Modified

- `internal/domain/errors.go` - ErrNotFound sentinel for store 404 discrimination
- `internal/config/config.go` - Added APIConfig struct and API field to Config; api.port default 8080
- `internal/api/response.go` - PagedResponse, ErrorResponse, writeJSON, writeError helpers
- `internal/api/server.go` - Server struct with LogStore/AnomalyStore/esClient/atomic.Bool; NewServer with chi + middleware; ListenAndServe/Shutdown/SetReady/Router
- `internal/api/middleware.go` - zapRequestLogger using WrapResponseWriter to capture status code
- `internal/api/health.go` - handleHealth (ES ping, 3s timeout), handleReady (atomic flag check)
- `internal/elasticsearch/log_indexer.go` - Added client field; SearchLogs with bool/term/range; GetLog via IDs query
- `internal/elasticsearch/anomaly_indexer.go` - Added client field; SearchAnomalies with term/range filters; GetAnomaly via Get API
- `go.mod` / `go.sum` - Added github.com/go-chi/chi/v5 v5.2.5

## Decisions Made

- IDs query (not Get API) for LogIndexer.GetLog: the Get API requires a concrete index name; `logs-*` is a wildcard pattern that only works with Search.
- AnomalyQuery.Type maps to `rule_id` ES field (not `type`) per the anomalies index mapping defined in setup.go.
- `detected_at` field for anomaly time range (not `@timestamp`) per the anomalies index mapping.
- `atomic.Bool` for Server.ready: single writer (SetReady), multiple readers; no mutex needed.
- Per-handler chi Timeout(30s) + conservative http.Server read/write timeouts (60s) for defense in depth.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 04-02 can immediately register log query handlers on `s.Router()` using `s.logStore.SearchLogs` and `s.logStore.GetLog`
- Plan 04-03 can immediately register anomaly query handlers using `s.anomStore.SearchAnomalies` and `s.anomStore.GetAnomaly`
- `go build ./...` exits 0 — all packages compile cleanly

## Self-Check: PASSED

All created files verified on disk. All task commits verified in git history (7609085, 7d24acb, 9acb70a). `go build ./...` exits 0.

---
*Phase: 04-rest-api*
*Completed: 2026-03-22*
