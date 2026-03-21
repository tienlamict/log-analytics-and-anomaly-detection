---
phase: 01-foundation-and-ingestion
plan: "01"
subsystem: infra
tags: [go, kafka, franz-go, viper, zap, prometheus, domain-types, interfaces]

requires: []

provides:
  - Go module github.com/log-analytics/server at go 1.22 with all Phase 1 dependencies
  - internal/domain package: RawMessage, LogEntry, Anomaly, LogQuery, AnomalyQuery types
  - internal/domain package: MessageConsumer, Parser, Enricher, Detector, AlertChannel, LogStore, AnomalyStore interfaces
  - internal/config package: Config struct with KafkaConfig (brokers, topic, group_id, initial_offset), MetricsConfig, LogConfig
  - config.yaml: default configuration file
  - cmd/server/main.go: compilable entrypoint stub that loads config

affects:
  - 01-02 (Kafka consumer implementation — imports domain and config)
  - 01-03 (log parser — imports domain types)
  - 01-04 (metrics — imports domain)
  - 01-05 (tests — imports domain and testify/goleak already in go.mod)
  - all subsequent phases import internal/domain

tech-stack:
  added:
    - github.com/twmb/franz-go v1.20.7 (Kafka consumer)
    - github.com/twmb/franz-go/plugin/kzap v1.1.2 (Zap bridge for Kafka client)
    - go.uber.org/zap v1.27.0 (structured logging)
    - github.com/prometheus/client_golang v1.23.2 (metrics)
    - github.com/google/uuid v1.6.0 (UUID generation)
    - github.com/spf13/viper v1.21.0 (YAML config + env var binding)
    - github.com/stretchr/testify v1.11.1 (test assertions)
    - go.uber.org/goleak v1.3.0 (goroutine leak detection)
  patterns:
    - Domain-first: internal/domain has zero external imports, all other packages import it
    - Config via Viper with defaults, env var override, YAML file, no panic on missing file
    - Entrypoint validates config load at startup before any pipeline wiring

key-files:
  created:
    - go.mod (module definition and all Phase 1 dependencies)
    - go.sum (dependency checksums)
    - internal/domain/types.go (RawMessage, LogEntry, Anomaly, LogQuery, AnomalyQuery)
    - internal/domain/interfaces.go (7 core interfaces)
    - internal/config/config.go (Config struct and Load function using Viper)
    - config.yaml (default configuration)
    - cmd/server/main.go (entrypoint stub)
  modified: []

key-decisions:
  - "Module path github.com/log-analytics/server — placeholder acceptable for v1, must match actual repo URL on push"
  - "go 1.22 as minimum directive — eliminates loop variable capture pitfall (Go 1.22+ fix), conservative and broadly compatible"
  - "kzap plugin version v1.1.2 (not v1.20.7) — kzap has independent versioning from franz-go core"

patterns-established:
  - "Pattern: Domain keystone — internal/domain imports only time and context from stdlib, zero project imports"
  - "Pattern: Viper config loading — SetDefault before ReadInConfig, ConfigFileNotFoundError is non-fatal"
  - "Pattern: go mod tidy after all go get calls to prune unused entries"

requirements-completed:
  - INGEST-01
  - INGEST-04

duration: 6min
completed: 2026-03-21
---

# Phase 01 Plan 01: Go Module Init and Domain Foundation Summary

**Go module with 8 Phase 1 dependencies, 5 domain types, 7 core interfaces, Viper config loader with YAML defaults, and compilable entrypoint stub.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-21T07:51:01Z
- **Completed:** 2026-03-21T07:57:44Z
- **Tasks:** 2
- **Files modified:** 7 created + go.sum

## Accomplishments

- Established internal/domain as the architectural keystone with zero external imports — only stdlib `time` and `context`
- Defined all 5 domain types (RawMessage, LogEntry, Anomaly, LogQuery, AnomalyQuery) and all 7 interfaces (MessageConsumer, Parser, Enricher, Detector, AlertChannel, LogStore, AnomalyStore) that every subsequent plan imports
- Wired Viper config loader with typed Config struct, YAML defaults, environment variable override support, and non-fatal ConfigFileNotFoundError handling
- Added all 8 Phase 1 dependencies to go.mod so later plans can import without additional `go get` steps

## Task Commits

Each task was committed atomically:

1. **Task 1: Go module init, domain types, and interfaces** - `6ded27f` (feat)
2. **Task 2: Config package with Viper and entrypoint stub** - `483c3c5` (feat)

## Files Created/Modified

- `go.mod` - Module definition at go 1.22 with all Phase 1 direct dependencies
- `go.sum` - Dependency checksums
- `internal/domain/types.go` - RawMessage, LogEntry, Anomaly, LogQuery, AnomalyQuery structs (stdlib only)
- `internal/domain/interfaces.go` - 7 core interfaces (stdlib only: context)
- `internal/config/config.go` - Config struct with KafkaConfig, MetricsConfig, LogConfig; Load() using Viper
- `config.yaml` - Default config (kafka brokers, topic, group_id, initial_offset, metrics port, log level)
- `cmd/server/main.go` - Stub entrypoint: loads config, prints kafka settings, exits cleanly

## Decisions Made

- Used `github.com/log-analytics/server` as module path (placeholder aligned with research recommendation — update to actual GitHub repo URL before public push)
- Set `go 1.22` minimum directive to eliminate loop variable capture pitfall (Go 1.22+ fix) per research recommendation
- kzap plugin installed as `v1.1.2` — the kzap plugin has its own versioning independent of franz-go core (`v1.20.7` is only the core module version)

## Deviations from Plan

None — plan executed exactly as written. The kzap version discrepancy (plan specified `v1.20.7` but kzap has independent versioning at `v1.1.2`) is not a deviation — it is the correct current version of the plugin module.

## Issues Encountered

- `go mod tidy` after installing franz-go, zap, prometheus, uuid, testify, and goleak removed them from go.mod because no source files import them yet. Fixed by re-running `go get` for each dependency after `go mod tidy` so they remain in go.mod for later plans.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All domain types and interfaces ready for implementation in 01-02 (Kafka consumer)
- Config struct has `InitialOffset` field required by INGEST-04
- All Phase 1 dependencies in go.mod — 01-02 through 01-05 can import without additional `go get`

---
*Phase: 01-foundation-and-ingestion*
*Completed: 2026-03-21*

## Self-Check: PASSED

- go.mod: FOUND
- go.sum: FOUND
- internal/domain/types.go: FOUND
- internal/domain/interfaces.go: FOUND
- internal/config/config.go: FOUND
- config.yaml: FOUND
- cmd/server/main.go: FOUND
- .planning/phases/01-foundation-and-ingestion/01-01-SUMMARY.md: FOUND
- Commit 6ded27f: FOUND
- Commit 483c3c5: FOUND
- go build ./...: PASSED
- go vet ./...: PASSED
