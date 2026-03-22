---
phase: 04-rest-api
plan: "04"
subsystem: testing
tags: [go, httptest, goleak, prometheus, chi, testify]

requires:
  - phase: 04-rest-api/04-01
    provides: Server struct, NewServer, SetReady, Router, LogStore/AnomalyStore interfaces, ES store implementations
  - phase: 04-rest-api/04-02
    provides: handleListLogs, handleGetLog, parseIntParam, parseTimeParam handler implementations
  - phase: 04-rest-api/04-03
    provides: handleListAnomalies, handleGetAnomaly handler implementations

provides:
  - 20 unit tests covering all API handler behaviors without external dependencies
  - mockLogStore and mockAnomalyStore implementing full domain interfaces
  - goleak.VerifyTestMain goroutine leak detection active for the api package
  - nil-guard in handleHealth preventing panic with nil ES client in tests

affects: [05-integration-hardening]

tech-stack:
  added: []
  patterns:
    - httptest.NewRecorder pattern for handler testing without port binding
    - Mock store structs implementing full interface for isolated unit tests
    - goleak.VerifyTestMain for goroutine leak detection in test suite
    - Observe Vec metric label before scraping /metrics to trigger output

key-files:
  created:
    - internal/api/server_test.go
    - internal/api/health_test.go
    - internal/api/logs_test.go
    - internal/api/anomalies_test.go
  modified:
    - internal/api/health.go

key-decisions:
  - "Nil-guard added to handleHealth: if esClient==nil return 503 immediately, prevents panic in tests without real ES"
  - "Vec metrics (CounterVec) require at least one label observation before appearing in Prometheus text output; test observes label with Add(0) before /metrics request"
  - "Mock stores implement full LogStore and AnomalyStore interfaces with per-test err injection; ErrNotFound sentinel used for 404 routing"

patterns-established:
  - "newTestServer helper: NewServer(logStore, anomStore, nil, zap.NewNop(), 0) — nil ES client triggers 503 on /health"
  - "s.Router().ServeHTTP(rec, req) pattern — no port binding, fast isolated tests"
  - "mockLogStore/mockAnomalyStore with entries []T, total int64, err error — covers happy path, not-found, and store error in each test"

requirements-completed: [API-01, API-02, API-03, API-04, API-05, API-06]

duration: 20min
completed: 2026-03-22
---

# Phase 04 Plan 04: API Handler Unit Tests Summary

**20 httptest-based unit tests proving all six REST API requirements pass with mock stores, -race, and goleak goroutine leak detection**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-03-22T04:15:00Z
- **Completed:** 2026-03-22T04:35:00Z
- **Tasks:** 3 completed
- **Files modified:** 5 (4 created, 1 modified)

## Accomplishments

- Created mock implementations of domain.LogStore and domain.AnomalyStore with in-memory state, err injection, and ErrNotFound sentinel routing
- 20 tests covering: valid queries, pagination params, invalid param 400 responses, store error 500 propagation, 404 for missing resources, and the Prometheus metrics endpoint
- goleak.VerifyTestMain active — any goroutine leak in the api package will fail the test suite
- Added nil-guard to handleHealth so tests run without a real Elasticsearch client

## Task Commits

Each task was committed atomically:

1. **Task 1: Mock stores, TestMain, and health/ready/metrics tests** - `7f4e8c0` (feat)
2. **Task 2: Log handler tests (list and detail)** - `2f6f388` (test)
3. **Task 3: Anomaly handler tests (list and detail)** - `4a8fb5b` (test)

## Files Created/Modified

- `internal/api/server_test.go` - TestMain with goleak, mockLogStore, mockAnomalyStore, newTestServer helper
- `internal/api/health_test.go` - Tests for health (503 nil ES), ready (503/200), metrics (/metrics Prometheus output)
- `internal/api/logs_test.go` - 9 tests: list valid, pagination, invalid page/size/from (400), store error (500), get found/not-found/error
- `internal/api/anomalies_test.go` - 7 tests: list valid, pagination, invalid page (400), store error (500), get found/not-found/error
- `internal/api/health.go` - Added nil-guard: `if s.esClient == nil { writeError(w, 503, "unhealthy"); return }`

## Decisions Made

- Nil-guard added to handleHealth rather than using a mock ES interface — the ES TypedClient is a concrete struct, not an interface, so nil-guarding is the only practical way to test the 503 path without a live ES instance
- Vec metrics (CounterVec) with labels need at least one observation before appearing in Prometheus text output — test calls `WithLabelValues(...).Add(0)` before hitting /metrics
- `pkgmetrics` named import used in health_test.go (not blank import) to be able to call `LogsConsumedTotal.WithLabelValues(...)` for label observation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added nil-guard to handleHealth**
- **Found during:** Task 1 (Mock stores, TestMain, and health/ready/metrics tests)
- **Issue:** handleHealth called `s.esClient.Ping().Do(ctx)` unconditionally; passing nil esClient to newTestServer would panic at test time
- **Fix:** Added early return: `if s.esClient == nil { writeError(w, http.StatusServiceUnavailable, "unhealthy"); return }`
- **Files modified:** internal/api/health.go
- **Verification:** TestHandleHealth_ESUnavailable passes and returns 503
- **Committed in:** 7f4e8c0 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed metrics assertion approach for Vec metrics**
- **Found during:** Task 1 (TestHandleMetrics)
- **Issue:** Plan specified asserting `logs_consumed_total` appears in body, but CounterVec metrics with unobserved labels produce no output lines in the Prometheus text format (not even HELP lines)
- **Fix:** Changed to named import of pkgmetrics and call `WithLabelValues(...).Add(0)` before the /metrics request to trigger output; assertion then checks for metric name string
- **Files modified:** internal/api/health_test.go
- **Verification:** TestHandleMetrics passes, body contains `logs_consumed_total` and `anomalies_detected_total`
- **Committed in:** 7f4e8c0 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 missing nil-guard, 1 test assertion bug)
**Impact on plan:** Both fixes necessary for test correctness. No scope creep.

## Issues Encountered

None beyond the deviations documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All six API requirements (API-01 through API-06) are proven by passing tests
- The api package runs cleanly under `-race` with no goroutine leaks detected
- Phase 5 (Integration and Hardening) can now build on a verified API layer
- No blockers

---
*Phase: 04-rest-api*
*Completed: 2026-03-22*
