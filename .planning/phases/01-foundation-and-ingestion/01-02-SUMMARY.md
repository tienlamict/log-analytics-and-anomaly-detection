---
phase: 01-foundation-and-ingestion
plan: 02
subsystem: ingestion
tags: [kafka, franz-go, kgo, kzap, zap, at-least-once, consumer-group, offset-commit]

# Dependency graph
requires:
  - phase: 01-foundation-and-ingestion/01-01
    provides: "domain types (RawMessage), domain interfaces (MessageConsumer), config structs (KafkaConfig), module scaffold with franz-go dependency"
provides:
  - "KafkaConsumer implementing domain.MessageConsumer using franz-go"
  - "At-least-once offset semantics via AutoCommitMarks + MarkCommitRecords after channel send"
  - "Partition revoke drain via OnPartitionsRevoked + CommitMarkedOffsets"
  - "Configurable initial offset (oldest/newest) via config.KafkaConfig.InitialOffset"
  - "Graceful shutdown via context cancellation stopping PollFetches loop"
affects:
  - detection-engine
  - storage-and-alerting
  - integration-and-hardening

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "At-least-once Kafka consumption: AutoCommitMarks + MarkCommitRecords strictly after channel send"
    - "Partition rebalance safety: OnPartitionsRevoked flushes offsets synchronously before returning"
    - "Graceful shutdown: PollFetches respects context cancellation; output channel closed via defer"
    - "Constructor injection: *zap.Logger and config.KafkaConfig passed to New()"

key-files:
  created:
    - internal/kafka/consumer.go
  modified: []

key-decisions:
  - "Mark offset AFTER channel send (not before) — prevents silent data loss on crash (Pitfall K1)"
  - "Buffered channel capacity 1000 — decouples Kafka poll rate from downstream processing speed"
  - "OnPartitionsRevoked with synchronous CommitMarkedOffsets — prevents duplicate replay after rebalance"
  - "kzap.New(logger) bridges Kafka client logs into the application Zap logger"

patterns-established:
  - "At-least-once ordering: send to channel first, mark offset second — mandatory pattern for all consumers"
  - "Defer both close(c.out) and c.client.Close() in Run() — ensures cleanup on any exit path"

requirements-completed: [INGEST-01, INGEST-02, INGEST-03, INGEST-04]

# Metrics
duration: 1min
completed: 2026-03-21
---

# Phase 01 Plan 02: Kafka Consumer Summary

**franz-go KafkaConsumer with AutoCommitMarks at-least-once semantics, partition revoke drain, and configurable initial offset**

## Performance

- **Duration:** ~1 min
- **Started:** 2026-03-21T08:00:12Z
- **Completed:** 2026-03-21T08:01:10Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Implemented `internal/kafka/consumer.go` with `Consumer` struct and `New` constructor using franz-go
- Established at-least-once offset commit semantics: `MarkCommitRecords` called only after successful channel send
- Added `OnPartitionsRevoked` callback that calls `CommitMarkedOffsets` synchronously to prevent duplicate replay on rebalance
- Configurable initial offset: `"newest"` maps to `kgo.NewOffset().AtEnd()`, all other values to `kgo.NewOffset().AtStart()`
- Graceful shutdown via context cancellation — `PollFetches` returns when context is cancelled, output channel closed via `defer`

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement KafkaConsumer with franz-go** - `0fe628c` (feat)

**Plan metadata:** (pending — this summary commit)

## Files Created/Modified

- `internal/kafka/consumer.go` — KafkaConsumer implementing domain.MessageConsumer with at-least-once offset semantics

## Decisions Made

- Buffered output channel capacity set to 1000 to decouple Kafka poll rate from downstream processing speed without risking unbounded memory growth at <1k logs/sec target volume
- No additional error handling deviation needed — fetch errors are logged and the loop continues, consistent with at-least-once design (failed fetches are retried by franz-go automatically)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None — all franz-go dependencies were already present in go.mod from the phase 01-01 scaffold. Build and vet passed on first attempt.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Kafka consumer is the ingestion entry point for the entire pipeline
- `domain.MessageConsumer` interface is fully implemented and ready to be wired into the pipeline orchestrator
- The `Messages()` channel is ready to be consumed by the parser (next plan in phase)
- No blockers

---
*Phase: 01-foundation-and-ingestion*
*Completed: 2026-03-21*
