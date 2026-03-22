---
phase: 03-storage-and-alerting
plan: "02"
subsystem: database
tags: [elasticsearch, esutil, bulk-indexer, go-elasticsearch, prometheus, sha256]

requires:
  - phase: 03-01
    provides: NewClient(*elasticsearch.TypedClient), ApplyIndexTemplates, package structure
  - phase: 01-foundation
    provides: domain.LogEntry, domain.Anomaly, domain.LogStore, domain.AnomalyStore interfaces
  - phase: 01-metrics
    provides: metrics.ESWriteErrorsTotal counter

provides:
  - LogIndexer implementing domain.LogStore (IndexLog, Close; stubs for Search/Get)
  - AnomalyIndexer implementing domain.AnomalyStore (IndexAnomaly, Close; stubs for Search/Get)
  - sha256HexID helper for deterministic Elasticsearch document IDs
  - Unit tests with mock HTTP transport covering success, ID routing, and failure metrics

affects:
  - 03-04-alert-dispatcher (AnomalyIndexer is the anomaly store wired in dispatcher)
  - 04-rest-api (LogIndexer/AnomalyIndexer will have SearchLogs/SearchAnomalies implemented)
  - 05-integration (both indexers wired in main.go)

tech-stack:
  added: []
  patterns:
    - "esutil.BulkIndexer with per-item Index override for dynamic daily indices"
    - "sha256(partition:offset) for deterministic, idempotent document IDs"
    - "OnFailure callback pattern: log error details + increment ESWriteErrorsTotal"
    - "roundTripFunc mock transport with X-Elastic-Product header for ES v9 tests"
    - "testutil.ToFloat64 on default registry for Prometheus counter assertions"

key-files:
  created:
    - internal/elasticsearch/log_indexer.go
    - internal/elasticsearch/log_indexer_test.go
    - internal/elasticsearch/anomaly_indexer.go
    - internal/elasticsearch/anomaly_indexer_test.go
  modified: []

key-decisions:
  - "LogIndexer uses Index='' in BulkIndexerConfig and sets per-item Index in BulkIndexerItem — allows daily rolling indices logs-{YYYY.MM.DD}"
  - "AnomalyIndexer uses Index='anomalies' in BulkIndexerConfig with no per-item override — fixed index for anomalies"
  - "Document ID for logs = sha256hex(fmt.Sprintf('%d:%d', partition, offset)) — deterministic, enables idempotent Kafka replay"
  - "Document ID for anomalies = anomaly.ID (UUID from DetectorEngine) — preserves semantic identity across re-indexing"
  - "SearchLogs/GetLog/SearchAnomalies/GetAnomaly return errors.New('not implemented') — deferred to Phase 4 REST API"
  - "Unused 'bytes' import in anomaly_indexer_test.go removed (Rule 1 auto-fix)"

patterns-established:
  - "BulkIndexer construction: NumWorkers 2 for logs (higher volume), 1 for anomalies (lower volume)"
  - "BulkIndexer flush: 5 MB / 5s for logs; 1 MB / 3s for anomalies"
  - "OnError (indexer-level) and OnFailure (item-level) both increment ESWriteErrorsTotal"
  - "Close(ctx) called on shutdown to flush remaining buffered items"

requirements-completed: [STORE-01, STORE-02, STORE-04]

duration: 3min
completed: 2026-03-22
---

# Phase 3 Plan 02: ES Indexers Summary

**LogIndexer and AnomalyIndexer via esutil.BulkIndexer with sha256 doc IDs, daily log indices, and per-item failure metering**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-22T01:58:40Z
- **Completed:** 2026-03-22T02:01:34Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- LogIndexer bulk-indexes log entries to daily `logs-{YYYY.MM.DD}` indices with sha256(partition:offset) document IDs — ensures idempotent Kafka replay
- AnomalyIndexer bulk-indexes anomalies to fixed `anomalies` index using anomaly UUID as document ID
- Both indexers implement per-item OnFailure callbacks that increment ESWriteErrorsTotal and log error details without halting the pipeline
- 7 unit tests cover success path, deterministic ID generation, daily index routing, and failure metric increments — all pass under -race

## Task Commits

Each task was committed atomically:

1. **Task 1: LogIndexer with BulkIndexer and deterministic IDs** - `c8726cf` (feat)
2. **Task 2: AnomalyIndexer with BulkIndexer** - `fe73804` (feat)

## Files Created/Modified

- `internal/elasticsearch/log_indexer.go` - LogIndexer: bulk indexing to daily logs-* indices with sha256 doc IDs
- `internal/elasticsearch/log_indexer_test.go` - Tests: success, deterministic IDs, daily index routing, failure metric
- `internal/elasticsearch/anomaly_indexer.go` - AnomalyIndexer: bulk indexing to fixed anomalies index with UUID doc IDs
- `internal/elasticsearch/anomaly_indexer_test.go` - Tests: success, ID propagation to NDJSON, failure metric

## Decisions Made

- LogIndexer sets Index per-item (in BulkIndexerItem) to support daily rolling log indices; AnomalyIndexer sets Index in BulkIndexerConfig since the anomalies index is fixed.
- Document IDs: sha256hex(partition:offset) for logs (idempotent replay), anomaly.ID for anomalies (UUID preserves semantic identity).
- SearchLogs, GetLog, SearchAnomalies, GetAnomaly return `errors.New("not implemented")` stubs — these are Phase 4 responsibilities.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unused "bytes" import in anomaly_indexer_test.go**
- **Found during:** Task 2 (AnomalyIndexer tests)
- **Issue:** `bytes` was initially imported but not used — build failed with "imported and not used" error
- **Fix:** Removed `"bytes"` from the import block
- **Files modified:** internal/elasticsearch/anomaly_indexer_test.go
- **Verification:** `go test -race ./internal/elasticsearch/... -run TestAnomalyIndexer` passes
- **Committed in:** fe73804 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — unused import)
**Impact on plan:** Trivial fix, no scope change.

## Issues Encountered

None beyond the unused import auto-fix above.

## Known Stubs

The following methods intentionally return `errors.New("not implemented")` — they are Phase 4 responsibilities:

| File | Method | Reason |
|------|--------|--------|
| internal/elasticsearch/log_indexer.go | `SearchLogs` | Phase 4 REST API |
| internal/elasticsearch/log_indexer.go | `GetLog` | Phase 4 REST API |
| internal/elasticsearch/anomaly_indexer.go | `SearchAnomalies` | Phase 4 REST API |
| internal/elasticsearch/anomaly_indexer.go | `GetAnomaly` | Phase 4 REST API |

These stubs satisfy the `domain.LogStore` and `domain.AnomalyStore` interface contracts and are expected at this stage. Phase 3's goal (STORE-01, STORE-02, STORE-04) covers only IndexLog and IndexAnomaly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- LogIndexer and AnomalyIndexer are ready to wire into the alert dispatcher (plan 03-04) and main.go (plan 05)
- Both implement their respective domain interfaces — compile-time verified
- Phase 4 REST API can complete the SearchLogs/GetLog/SearchAnomalies/GetAnomaly stubs

---
*Phase: 03-storage-and-alerting*
*Completed: 2026-03-22*
