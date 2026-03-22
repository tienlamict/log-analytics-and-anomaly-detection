---
gsd_state_version: 1.0
milestone: v1.0.0
milestone_name: milestone
status: unknown
stopped_at: Completed 03-storage-and-alerting/03-01-PLAN.md
last_updated: "2026-03-22T01:57:04.990Z"
progress:
  total_phases: 5
  completed_phases: 2
  total_plans: 14
  completed_plans: 12
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-21)

**Core value:** Detect anomalies in application logs in real-time and surface them via alerts and a queryable API — so problems are caught before users report them.
**Current focus:** Phase 03 — storage-and-alerting

---

## Current Status

**Phase:** 3
**Milestone:** v1.0.0
**Overall progress:** [█████████░] 86% — 12/14 plans complete

| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation and Ingestion | Complete (05/05 plans done) |
| 2 | Detection Engine | Complete (05/05 plans done) |
| 3 | Storage and Alerting | Pending |
| 4 | REST API | Pending |
| 5 | Integration and Hardening | Pending |

---

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Kafka via franz-go v1.20.7 | Pure Go, no CGo, actively maintained, first-class Prometheus/Zap plugins |
| Elasticsearch v9 (go-elasticsearch/v9) | TypedClient for compile-time checked schemas, native OTel, current major |
| chi v5 for HTTP | 100% net/http compatible, lightweight, no framework lock-in |
| go-mail v0.7.2 for SMTP | Security-patched (GO-2025-3988 fixed in v0.7.1), context-aware |
| Rule-based detection only in v1 | Predictable, debuggable, sufficient for known anomaly patterns |
| Config-driven thresholds (YAML) | Operators tune without redeployment |
| 7 detection rules in v1 | Error spike, latency breach, repeated failure, auth burst, off-hours access, service silence, dedup/cooldown |
| Module path github.com/log-analytics/server | Placeholder per research recommendation; update to actual repo URL before public push |
| go 1.22 minimum directive | Eliminates loop variable capture pitfall (Go 1.22+ fix), conservative and broadly compatible |
| kzap plugin v1.1.2 (not v1.20.7) | kzap has independent module versioning from franz-go core |
| Mark offset after channel send (not before) | Prevents silent data loss on crash — MarkCommitRecords called only after successful c.out send |
| Buffered channel capacity 1000 | Decouples Kafka poll rate from downstream processing speed at <1k logs/sec target |
| AlreadyRegisteredError pattern for Vec metric tests | Gather() omits Vec metrics with no observations; re-registering same collector returns AlreadyRegisteredError to prove registration |
| goleak.VerifyTestMain not VerifyNone | VerifyNone produces false positives with parallel tests; VerifyTestMain is the correct goroutine leak detection pattern |
| testutil.ToFloat64 on default registry for Prometheus assertions | Custom prometheus.NewRegistry() causes double-registration panic since metrics are already registered via init() |
| Consumer ordering invariants via source inspection | strings.Index ordering assertions validate at-least-once semantics (INGEST-02) without requiring a live Kafka broker; full integration deferred to Phase 5 |
| cooldownMu sync.Mutex added to DetectorEngine (Plan 02-05) | -race flag surfaced data race: Evaluate and silenceWatcher write cooldowns map while evictLoop iterates it; mutex added to all three paths |
| atomic.Int64 for mock detector counters in tests | evictLoop goroutine calls Reset() concurrently with test assertions; atomic avoids secondary lock hierarchy for simple counters |
| Non-monotonic scan in slidingWindow.CountWithin | Kafka out-of-order delivery means timestamps are not guaranteed monotonic; full scan safer than short-circuit optimization |
| Rules in internal/detection/ package (not sub-package) | slidingWindow is unexported; co-location avoids exporting it or using type aliases |
| Custom latencyWindow struct for LatencyThresholdRule | slidingWindow tracks timestamps only; breach rate math needs bool per entry alongside timestamp |
| ServiceSilenceRule adapter pattern: Evaluate never fires | Absence-based detection cannot be event-driven; CheckSilence polled by heartbeat goroutine so anomalies flow through same output channel |
| lastSeen tracks ingest time not entry.Timestamp | Silence detection requires wall-clock elapsed time since last ingestion, not the timestamp in the log event |
| Only ServiceSilenceRule needs sync.Mutex | It is the only rule where goroutine boundaries cross (pipeline Evaluate vs heartbeat CheckSilence); all others satisfy single-goroutine-caller contract |
| config imports detection (not vice versa) | detection package has no config dependency so no import cycle; DetectionConfig stays in detection package |
| No custom Viper DecodeHook for duration strings | Built-in StringToTimeDurationHookFunc handles "5m"->time.Duration; adding custom hook would override the default |
| SMTPAlerter uses WithPort + WithTLSPolicy (not WithTLSPortPolicy) | Port is explicit in SMTPConfig; WithTLSPortPolicy auto-selects port and would conflict with the configured value |
| SMTPAlerter queue-full drops return nil (non-fatal) with metric | Prevents dispatcher back-pressure from a transient SMTP buffer saturation event; failure is metered via EmailAlertsFailedTotal |
| client.Indices is a struct field not a method on TypedClient | MethodIndices is a struct field on the embedded *typedapi.MethodAPI; use client.Indices.PutIndexTemplate() not client.Indices().PutIndexTemplate() |
| X-Elastic-Product header required in mock ES transport responses | go-elasticsearch/v9 client validates this header before parsing; omitting it causes test failures unrelated to business logic |

---

## Open Questions

- What log format do target application services emit? (JSON structured vs logfmt vs unstructured) — affects parser complexity
- Is `duration_ms` / `latency_ms` reliably present in logs? Required for latency breach rule (DETECT-03)
- What is the expected cardinality of services? (handful vs hundreds) — affects per-service rule config structure

---

## Session Continuity

**Last session:** 2026-03-22T01:57:04.977Z
**Stopped at:** Completed 03-storage-and-alerting/03-01-PLAN.md
**Next action:** Phase 03 plan 03 complete — continue with Phase 03 plans 04 (alert dispatcher)

---

## Todos

(None yet)

---
*Last updated: 2026-03-21 after project initialization*
