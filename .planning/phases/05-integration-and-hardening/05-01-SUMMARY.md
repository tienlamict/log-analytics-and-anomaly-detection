---
phase: 05-integration-and-hardening
plan: 01
subsystem: infra
tags: [errgroup, testcontainers, kafka, elasticsearch, pipeline-wiring, integration-testing, goleak]

# Dependency graph
requires:
  - phase: 04-rest-api
    provides: "api.NewServer, domain.LogStore, domain.AnomalyStore interfaces"
  - phase: 03-storage-and-alerting
    provides: "LogIndexer, AnomalyIndexer, SMTPAlerter, Dispatcher"
  - phase: 02-detection-engine
    provides: "DetectorEngine, 6 rule constructors"
  - phase: 01-foundation-and-ingestion
    provides: "kafka.Consumer, pipeline.Parse, config.Load"
provides:
  - "cmd/server/wire.go: runPipeline function wiring all components via errgroup"
  - "cmd/server/main.go: updated entrypoint delegating to runPipeline"
  - "internal/integration/main_test.go: TestMain with shared Kafka+ES testcontainers"
  - "testdata/integration-config.yaml: integration test config with zero warmup and 3-error threshold"
affects:
  - 05-02-pipeline-integration-tests
  - 05-03-offset-and-shutdown-tests

# Tech tracking
tech-stack:
  added:
    - testcontainers-go v0.41.0
    - testcontainers-go/modules/kafka v0.41.0
    - testcontainers-go/modules/elasticsearch v0.41.0
    - golang.org/x/sync v0.20.0 (errgroup)
  patterns:
    - errgroup.WithContext for fan-out pipeline goroutine coordination
    - LIFO shutdown watcher goroutine (API → engine.Stop → indexer flushes)
    - TestMain-scoped shared containers (not per-test)
    - goleak.VerifyTestMain with IgnoreTopFunction for integration leak detection
    - newESClient() bypasses project NewClient to use container CACert for TLS

key-files:
  created:
    - cmd/server/wire.go
    - internal/integration/main_test.go
    - testdata/integration-config.yaml
  modified:
    - cmd/server/main.go
    - go.mod
    - go.sum

key-decisions:
  - "runPipeline extracted to wire.go (not inlined in main.go) so integration tests can import and invoke it directly without subprocess overhead"
  - "LIFO shutdown order: apiServer.Shutdown → engine.Stop → logIndexer.Close → anomalyIndexer.Close — ensures HTTP drains before indexer flushes"
  - "warmup_multiplier: 0 in integration-config.yaml bypasses 10-minute warmup suppression so anomalies fire immediately in tests"
  - "newESClient() in test file uses CACert from container settings — ES 8+ uses HTTPS by default, project NewClient uses http.Transport without CACert"
  - "TestMain-scoped containers (not per-test) — starting containers per test is 10-30x slower and causes port flakiness"
  - "goleak.IgnoreTopFunction filters for net/http and poll goroutines — testcontainers background goroutines cause false positives otherwise"

patterns-established:
  - "Pattern: Pipeline worker range consumer.Messages() — log parse errors with Warn but continue (Parse always returns a valid entry)"
  - "Pattern: errgroup shutdown watcher blocks on gCtx.Done() then runs teardown with fresh context.WithTimeout(background, 8s)"
  - "Pattern: go vet -tags integration ./internal/integration/... as the verify command for gated integration test files"

requirements-completed: [TEST-02]

# Metrics
duration: 27min
completed: 2026-03-22
---

# Phase 5 Plan 01: Pipeline Wiring and Integration Test Infrastructure Summary

**errgroup-wired pipeline (Kafka → parse → ES log index → detection → dispatch → SMTP) with testcontainers TestMain scaffold for shared Kafka+ES containers**

## Performance

- **Duration:** 27 min
- **Started:** 2026-03-22T06:03:59Z
- **Completed:** 2026-03-22T06:31:19Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Created `cmd/server/wire.go` with `runPipeline` function wiring all 7 components (ES, Kafka, detection engine with 6 rules, SMTP alerter, dispatcher, API server) via `errgroup.WithContext`
- Updated `cmd/server/main.go` to delegate to `runPipeline` — removed old metrics-only HTTP server code
- Created `internal/integration/main_test.go` with `TestMain` starting shared Kafka and Elasticsearch testcontainers, goleak integration, and `newESClient()` helper
- Created `testdata/integration-config.yaml` with `warmup_multiplier: 0` and 3-error threshold for fast integration test execution

## Task Commits

Each task was committed atomically:

1. **Task 1: Create pipeline wiring function and update main.go** - `9b29c97` (feat)
2. **Task 2: Install testcontainers and create TestMain with shared containers + integration config** - `33dcbad` (feat)

## Files Created/Modified

- `cmd/server/wire.go` - runPipeline function with errgroup, all components wired, LIFO shutdown watcher
- `cmd/server/main.go` - Simplified entrypoint calling runPipeline; removed metrics-only server
- `internal/integration/main_test.go` - TestMain with Kafka+ES containers, goleak, newESClient helper
- `testdata/integration-config.yaml` - Integration test config: warmup=0, threshold=3, window=10s
- `go.mod` - Added testcontainers-go v0.41.0 (+kafka+elasticsearch modules), golang.org/x/sync v0.20.0
- `go.sum` - Updated checksums

## Decisions Made

- `runPipeline` extracted to `wire.go` (not inlined in `main.go`) so integration tests in plans 05-02/05-03 can import and call it directly without spawning a subprocess
- LIFO shutdown order in watcher goroutine: `apiServer.Shutdown` → `engine.Stop` → `logIndexer.Close` → `anomalyIndexer.Close` with 8s deadline context
- `warmup_multiplier: 0` in integration config bypasses the 10-minute warm-up suppression period that would prevent any anomaly from appearing in tests
- `newESClient()` in test file uses `CACert` from container settings — ES 8+ runs HTTPS, project `NewClient` uses plain `http.Transport` without certificate trust

## Deviations from Plan

None - plan executed exactly as written.

The only runtime deviation was that `go get golang.org/x/sync` upgraded `go` directive from 1.24.0 to 1.25.0 (golang.org/x/sync v0.20.0 requires go 1.25.0). This is a minor toolchain bump with no behavioral impact.

## Issues Encountered

- `go mod tidy` removed testcontainers after initial installation because no Go file in the module imports it yet (the test file uses `//go:build integration`). Resolved by explicitly re-running `go get` with all three testcontainers packages after creating `main_test.go`.
- `go build` triggered download of go1.25.0 toolchain after the go directive upgrade. This is a one-time download; no code changes required.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `runPipeline` in `wire.go` is importable by integration tests in plans 05-02 and 05-03
- `TestMain` scaffold with shared Kafka+ES containers is ready; plans 05-02/05-03 only need to add test functions to the `integration` package
- `testdata/integration-config.yaml` ready for use as config source when constructing test pipeline instances
- All unit tests still pass (`go test ./internal/...` — 9 packages, all green)

---
*Phase: 05-integration-and-hardening*
*Completed: 2026-03-22*
