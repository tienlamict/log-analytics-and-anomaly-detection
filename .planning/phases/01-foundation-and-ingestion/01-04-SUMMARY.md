---
phase: 01-foundation-and-ingestion
plan: 04
subsystem: observability
tags: [prometheus, metrics, zap, logging, http-server, signal-handling]
dependency_graph:
  requires: ["01-01"]
  provides: ["internal/metrics package", "Prometheus /metrics endpoint", "Zap production logger in main"]
  affects: ["all pipeline stages that import internal/metrics"]
tech_stack:
  added: ["github.com/prometheus/client_golang v1.23.2 (direct)", "go.uber.org/zap v1.27.0 (direct)"]
  patterns: ["init() registration pattern", "blank import side-effect", "constructor injection for logger"]
key_files:
  created:
    - internal/metrics/metrics.go
    - internal/metrics/metrics_test.go
  modified:
    - cmd/server/main.go
    - go.mod
    - go.sum
decisions:
  - "Use AlreadyRegisteredError check instead of Gather() to verify Vec metric registration — Gather() omits Vec metrics with no observed label combinations"
  - "zap.NewProduction() used (not NewDevelopment()) per OBS-01 requirement for deployed builds"
  - "Blank import pattern for metrics package ensures init() fires exactly once with no constructor registration"
metrics:
  duration_seconds: 205
  completed_date: "2026-03-21"
  tasks_completed: 2
  files_changed: 5
---

# Phase 01 Plan 04: Observability Instrumentation Summary

Registered all 8 Prometheus metrics via init() in a central metrics package, wired Zap production logger into main.go, and exposed /metrics endpoint with graceful SIGTERM shutdown.

## Tasks Completed

### Task 1: Register all Prometheus metrics and add registration test

Created `internal/metrics/metrics.go` with 8 package-level metric variables registered in `init()`:
- `LogsConsumedTotal` (CounterVec, labels: topic/partition)
- `LogsProcessedDuration` (Histogram, DefBuckets)
- `AnomaliesDetectedTotal` (CounterVec, labels: rule)
- `ESWriteErrorsTotal` (Counter)
- `EmailAlertsSentTotal` (Counter)
- `EmailAlertsFailedTotal` (Counter)
- `KafkaConsumerLag` (GaugeVec, labels: topic/partition)
- `ParseErrorsTotal` (Counter)

Created `internal/metrics/metrics_test.go` with `TestAllMetricsRegistered` verifying all 8 are in the default registry.

**Commit:** a2ee9c7

### Task 2: Wire Zap logger, metrics HTTP server, and signal handling in main.go

Rewrote `cmd/server/main.go` to:
- Load config via `config.Load()`
- Create `zap.NewProduction()` logger with `defer logger.Sync()`
- Blank-import `internal/metrics` to trigger registration
- Set up `signal.NotifyContext` for SIGINT/SIGTERM
- Mount `promhttp.Handler()` at `/metrics` on configured port
- Start HTTP server in goroutine, block on context cancellation
- Gracefully shut down HTTP server with 5-second timeout

**Commit:** a3bdda1

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test approach for Vec metric registration verification**

- **Found during:** Task 1 verification
- **Issue:** `prometheus.DefaultGatherer.Gather()` does not emit `CounterVec` or `GaugeVec` metrics until at least one label combination has been observed. The plan's test template using `Gather()` produced false negatives for `logs_consumed_total`, `anomalies_detected_total`, and `kafka_consumer_lag`.
- **Fix:** Changed test to attempt re-registration of each exported metric variable (exact same collector reference). Re-registering an already-registered collector returns `AlreadyRegisteredError`, proving presence in the registry. This is the canonical Prometheus pattern for this assertion.
- **Files modified:** `internal/metrics/metrics_test.go`
- **Commit:** a2ee9c7

## Verification Results

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test -race ./internal/metrics/...` — PASS (TestAllMetricsRegistered)
- All 8 metric names present in metrics.go
- main.go uses Zap (no fmt.Printf in pipeline logging)
- /metrics endpoint mounted via promhttp.Handler()
- Signal handling with graceful 5-second shutdown timeout

## Self-Check: PASSED

All created files found on disk. Both task commits (a2ee9c7, a3bdda1) confirmed in git log.

## Known Stubs

None — all metrics are registered and gatherable. The /metrics endpoint is live when the server runs. No placeholder data.
