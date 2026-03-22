---
phase: 03-storage-and-alerting
plan: 01
subsystem: database
tags: [elasticsearch, go-elasticsearch, typed-client, index-templates, config, smtp]

# Dependency graph
requires:
  - phase: 02-detection-engine
    provides: domain types (LogEntry, Anomaly), interfaces (LogStore, AnomalyStore, AlertChannel), metrics counters (ESWriteErrorsTotal)
provides:
  - NewClient constructor returning *elasticsearch.TypedClient with bounded connection pool
  - ApplyIndexTemplates for logs-* and anomalies indexes with dynamic:false and FlattenedProperty
  - ESConfig and SMTPConfig structs loadable from config.yaml via Viper
affects: [03-02-log-indexer, 03-03-smtp-alerter, 04-rest-api, cmd/server/main.go]

# Tech tracking
tech-stack:
  added:
    - github.com/elastic/go-elasticsearch/v9 v9.3.1 (TypedClient, PutIndexTemplate)
    - github.com/elastic/elastic-transport-go/v8 v8.8.0 (transitive)
    - go.opentelemetry.io/otel v1.35.0 (transitive from go-elasticsearch)
  patterns:
    - TypedClient construction with bounded http.Transport and retry-on-status
    - Index templates applied at startup via TypedAPI (idempotent PUT upserts)
    - dynamic:false + FlattenedProperty prevents mapping explosion from arbitrary fields keys
    - ClientConfig struct as a local config type (mirrored by ESConfig in config package)
    - Mock transport via roundTripFunc for unit testing TypedClient calls without live ES

key-files:
  created:
    - internal/elasticsearch/client.go
    - internal/elasticsearch/setup.go
    - internal/elasticsearch/setup_test.go
  modified:
    - internal/config/config.go
    - config.yaml
    - go.mod
    - go.sum

key-decisions:
  - "client.Indices.PutIndexTemplate (field access, not method call) — MethodIndices is a struct field on *typedapi.MethodAPI embedded in TypedClient"
  - "X-Elastic-Product header required in mock transport responses — v9 client validates this header before parsing responses"
  - "FlattenedProperty for both fields (LogEntry) and evidence (Anomaly) — prevents unbounded per-key sub-mappings in ES"
  - "Fail-fast: applyLogsTemplate before applyAnomaliesTemplate, first error propagated immediately"

patterns-established:
  - "Pattern: roundTripFunc implements http.RoundTripper for mock ES transport in tests"
  - "Pattern: TypedClient with MaxIdleConnsPerHost + ResponseHeaderTimeout bounds connection pool"
  - "Pattern: ESConfig/SMTPConfig added to Config struct with Viper defaults before ReadInConfig"

requirements-completed: [STORE-03]

# Metrics
duration: 9min
completed: 2026-03-22
---

# Phase 03 Plan 01: ES Client Infrastructure and Config Extension Summary

**Elasticsearch TypedClient with bounded connection pool, fail-fast index template setup (logs-* and anomalies with dynamic:false), and ESConfig/SMTPConfig added to Viper-backed config**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-22T01:45:58Z
- **Completed:** 2026-03-22T01:54:58Z
- **Tasks:** 2
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments
- NewClient constructor with bounded http.Transport (MaxIdleConnsPerHost, ResponseHeaderTimeout) and RetryOnStatus [502,503,504,429]
- ApplyIndexTemplates applies logs-template (logs-*) and anomalies-template at startup with dynamic:false and FlattenedProperty
- Three unit tests validating success, logs failure, and anomalies failure paths using mock transport
- ESConfig and SMTPConfig structs with mapstructure tags added to Config struct with Viper defaults
- config.yaml extended with elasticsearch and smtp sections fully populated

## Task Commits

Each task was committed atomically:

1. **Task 1: ES client constructor, index templates, and unit tests** - `621203e` (feat)
2. **Task 2: Add ESConfig and SMTPConfig to config package and config.yaml** - `e0aac54` (feat)

## Files Created/Modified
- `internal/elasticsearch/client.go` - NewClient constructor with ClientConfig, bounded transport, retry config
- `internal/elasticsearch/setup.go` - ApplyIndexTemplates, applyLogsTemplate, applyAnomaliesTemplate
- `internal/elasticsearch/setup_test.go` - 3 unit tests with roundTripFunc mock transport
- `internal/config/config.go` - ESConfig, SMTPConfig structs added; Config extended; Viper defaults added
- `config.yaml` - elasticsearch and smtp sections added
- `go.mod` / `go.sum` - go-elasticsearch/v9 and transitive dependencies added

## Decisions Made
- `client.Indices.PutIndexTemplate(...)` not `client.Indices().PutIndexTemplate(...)` — MethodIndices is a struct field on the embedded *typedapi.MethodAPI, not a method. The research example used `()` which would not compile.
- Mock transport must include `X-Elastic-Product: Elasticsearch` header — the v9 client validates this and rejects responses without it.
- roundTripFunc type alias for http.RoundTripper keeps test code concise without a full mock struct.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Indices field access syntax**
- **Found during:** Task 1 (ES client constructor and setup)
- **Issue:** Research pattern showed `client.Indices()` with parentheses (method call), but the TypedClient's Indices field is type MethodIndices (struct field), not a method. Build failed with "cannot call non-function".
- **Fix:** Changed to `client.Indices.PutIndexTemplate(...)` (no parentheses) matching the actual generated API.
- **Files modified:** internal/elasticsearch/setup.go
- **Verification:** go build ./... exits 0
- **Committed in:** 621203e (part of Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug fix for API field access syntax)
**Impact on plan:** Required for compilation; no scope change.

## Issues Encountered
- go-elasticsearch v9 TypedAPI uses struct fields (not methods) for API namespacing. The MethodIndices type contains PutIndexTemplate as a method on it, but Indices itself is a field on MethodAPI. Research had incorrect `()` after Indices.

## User Setup Required
None - no external service configuration required for this plan.

## Next Phase Readiness
- ES TypedClient constructor ready for use in log_indexer.go (plan 03-02)
- Index templates must be called at startup in main.go before consumer starts
- ESConfig.ResponseTimeout is time.Duration — Viper's built-in StringToTimeDurationHookFunc handles "10s" string from YAML
- SMTPConfig ready for use in smtp/alerter.go (plan 03-03)

---
*Phase: 03-storage-and-alerting*
*Completed: 2026-03-22*
