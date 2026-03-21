# Roadmap: Log Analytics and Anomaly Detection

**Project start:** 2026-03-21
**Target:** v1.0.0
**Phases:** 5

---

## Overview

Build a production-grade, event-driven log analytics and anomaly detection system in Go. The pipeline consumes application logs from Kafka, parses and normalises them, evaluates them against rule-based detection logic, persists everything to Elasticsearch, fires email alerts on confirmed anomalies, and exposes a queryable REST API. Each phase delivers a vertically complete, testable capability; later phases depend on earlier ones strictly in sequence.

---

## Phases

- [x] **Phase 1: Foundation and Ingestion** - Go module scaffolding, domain types, interfaces, Kafka consumer, log parser, Zap logging, Prometheus metrics skeleton, and parser unit tests. (completed 2026-03-21)
- [ ] **Phase 2: Detection Engine** - Rule-based anomaly detection with sliding-window state, all seven detection rules, alert deduplication / cooldown, and config-driven thresholds in YAML.
- [ ] **Phase 3: Storage and Alerting** - Elasticsearch integration for logs and anomalies (index mappings applied at startup, BulkIndexer), and SMTP email alerting via go-mail with deduplication wired to cooldown state.
- [ ] **Phase 4: REST API** - chi-based REST API covering log query endpoints, anomaly query endpoints, and health / readiness / metrics endpoints.
- [ ] **Phase 5: Integration and Hardening** - End-to-end integration tests with testcontainers (real Kafka and Elasticsearch), graceful shutdown hardening, pipeline wiring validation, and setup README.

---

## Phase Details

### Phase 1: Foundation and Ingestion

**Goal**: Establish the project skeleton and deliver a working Kafka consumer that reliably ingests, parses, and normalises log entries with full observability instrumentation.
**Status:** Planned
**Depends on**: Nothing (first phase)
**Requirements**: INGEST-01, INGEST-02, INGEST-03, INGEST-04, PARSE-01, PARSE-02, PARSE-03, PARSE-04, OBS-01, OBS-02, TEST-01
**Plans:** 5/5 plans complete

Plans:
- [x] 01-01-PLAN.md — Module scaffold, domain types, interfaces, config loader
- [x] 01-02-PLAN.md — Kafka consumer with franz-go (at-least-once semantics)
- [x] 01-03-PLAN.md — Log parser and level normalisation
- [x] 01-04-PLAN.md — Observability skeleton (Prometheus metrics, Zap logging, /metrics endpoint)
- [x] 01-05-PLAN.md — Parser unit tests (table-driven, goleak)

### Success Criteria

- [ ] `go build ./...` succeeds from a clean checkout with no CGo dependencies.
- [ ] Consumer starts against a live Kafka broker, consumes messages from the configured topic, and commits offsets only after records pass through the parser — verified by killing the process mid-stream and confirming no messages are skipped on restart.
- [ ] Parser correctly normalises a valid JSON log, a plain-text log, a malformed byte sequence (does not panic, emits error metric, continues), and all documented log-level variants.
- [ ] All unit tests pass under `go test -race ./...` with zero data-race findings.
- [ ] `/metrics` endpoint returns Prometheus text exposition with all registered metrics present.

### Dependencies

- None (first phase)

---

### Phase 2: Detection Engine

**Goal**: Deliver a fully functional rule-based anomaly detection engine that evaluates every incoming log entry against all seven detection rules using sliding-window state, deduplicates repeated alerts via per-rule cooldown, and reads all thresholds from a YAML config file.
**Status:** Planned
**Depends on**: Phase 1
**Requirements**: DETECT-01, DETECT-02, DETECT-03, DETECT-04, DETECT-05, DETECT-06, DETECT-07, DETECT-08, DETECT-09, DETECT-10
**Plans:** 3/5 plans executed

Plans:
- [x] 02-01-PLAN.md — Detection engine core, sliding window, config types, eviction goroutine
- [x] 02-02-PLAN.md — Error spike, latency breach, and repeated failure rules
- [x] 02-03-PLAN.md — Auth burst, off-hours access, and service silence rules
- [ ] 02-04-PLAN.md — YAML config loading, Viper defaults, config.yaml detection section
- [ ] 02-05-PLAN.md — Detection unit tests (all rules, engine, cooldown, warm-up, eviction)

### Plans (Detail)

| # | Plan | Description |
|---|------|-------------|
| 02-01 | Detection engine core and sliding window | Implement `internal/detection` package: `DetectorEngine` that runs a registry of `Detector` implementations sequentially per log entry; sliding-window counter using a circular event-timestamp buffer (avoids fixed tumbling-window boundary artefacts — Pitfall A3); background goroutine for stale-window eviction to prevent unbounded memory growth (DETECT-10); single-goroutine ownership of all window state to eliminate lock contention (addresses Pitfall G2). |
| 02-02 | Error spike, latency breach, and repeated failure rules | Implement `ErrorRateRule` (DETECT-02): per-service error count exceeds threshold in window. Implement `LatencyThresholdRule` (DETECT-03): `duration_ms` field present and breach rate exceeds N% of requests in window. Implement `RepeatedFailureRule` (DETECT-04): same error fingerprint (message stripped of dynamic values) exceeds repeat count in window. |
| 02-03 | Auth burst, off-hours access, and service silence rules | Implement `AuthFailureBurstRule` (DETECT-05): auth failure events per source IP or user exceed threshold in window. Implement `OffHoursAccessRule` (DETECT-06): access to configured sensitive endpoint paths outside defined business-hours window. Implement `ServiceSilenceRule` (DETECT-07): timer-based heartbeat goroutine checks `last_seen` map per service; fires when silence exceeds threshold; requires service to have emitted N prior logs before entering active tracking (cold-start guard). |
| 02-04 | Alert deduplication, cooldown, and YAML config | Implement per-rule cooldown state (DETECT-08) as a separate `map[(ruleID, service)]time.Time` — distinct from window counters; suppress re-alerting within the configured window. Load all rule thresholds, window durations, severity levels, sensitive endpoint allowlists, business hours, and cooldown durations from YAML config at startup via Viper (DETECT-09). Include a warm-up period that suppresses alerts for 2× the window duration after startup (Pitfall A1 mitigation). |
| 02-05 | Detection unit tests | Table-driven unit tests for all seven rules: verify firing conditions, non-firing conditions, cooldown suppression, warm-up suppression, stale-window eviction, and the service-silence timer. All tests run with `-race`; no external dependencies. |

### Success Criteria

- [ ] All seven detection rules fire correctly when their threshold conditions are met, verified by unit tests that inject synthetic log entries with controlled timestamps.
- [ ] None of the seven rules fire during the warm-up period regardless of input volume.
- [ ] A rule that fires does not re-fire for the same service within the configured cooldown window when the condition persists.
- [ ] Window state map size remains bounded after receiving traffic for inactive services followed by eviction cycles.
- [ ] All detection thresholds and rule parameters are readable from `config.yaml` without code changes; changing a threshold value in the config file and restarting the process changes the rule's behaviour.

### Dependencies

- Phase 1 (domain types, `LogEntry` struct, `Detector` interface, Prometheus metrics)

---

### Phase 3: Storage and Alerting

**Goal**: Persist raw log entries and detected anomalies to Elasticsearch using bulk indexing with correct index mappings applied at startup, and dispatch asynchronous SMTP email alerts for confirmed anomalies with deduplication wired to the existing cooldown state.
**Status:** Pending
**Depends on**: Phase 2
**Requirements**: STORE-01, STORE-02, STORE-03, STORE-04, ALERT-01, ALERT-02, ALERT-03

### Plans

| # | Plan | Description |
|---|------|-------------|
| 03-01 | Elasticsearch index setup and mapping | At application startup, before the Kafka consumer starts, apply index templates for `logs-*` (daily rollover, explicit mapping with `dynamic: false`, `@timestamp`, `level`, `service`, `message`, `fields` as a `flattened` or `keyword` blob to prevent mapping explosion — Pitfall E2/O5) and `anomalies` (explicit mapping for all `Anomaly` struct fields) via the ES `TypedClient`. Fail fast if templates cannot be applied (Pitfall E6). Set shard count to 1 primary per daily index for the target volume scale. |
| 03-02 | Log and anomaly BulkIndexer | Implement `internal/elasticsearch` with two `esutil.BulkIndexer` instances: one for `logs-{YYYY.MM.DD}` (daily-rollover index name generated at index time) and one for `anomalies`. Use deterministic document IDs derived from `sha256(partition + offset)` for logs and anomaly UUID for anomalies to ensure idempotent upsert on reprocessing (Pitfall O6). Parse every bulk response body — not just HTTP status — and increment `elasticsearch_write_errors_total` on per-item failures (Pitfall E1). Configure explicit `http.Transport` with bounded connection pool and `ResponseHeaderTimeout` (Pitfall E5). |
| 03-03 | SMTP email alerter | Implement `internal/smtp` `SMTPAlerter` using `github.com/wneessen/go-mail` v0.7.2 (patched for GO-2025-3988). Email includes anomaly type, affected service, detection time, threshold breached, severity, and a sample of contributing log lines (ALERT-02). All SMTP config — host, port, credentials, recipient list, TLS policy — is externalised via `config.yaml` / environment variables (ALERT-03). Alerting is fully asynchronous: detection pipeline writes to a buffered `chan Anomaly`; a separate goroutine drains the channel and sends emails; SMTP failures are logged and metered but do not block the pipeline (Pitfall O4). |
| 03-04 | Alerter orchestrator and deduplication wiring | Implement `internal/alert` `Alerter`: consumes the `anomaly` channel, checks the cooldown state from Phase 2 before dispatching (DETECT-08 wired), forwards the anomaly to both the SMTP alerter and the anomaly BulkIndexer regardless of email outcome (anomaly record is the source of truth). Wire the full fan-out path: `LogEntry` → tee → `[LogIndexer, DetectorEngine]`; `Anomaly` → `Alerter` → `[SMTPAlerter, AnomalyIndexer]`. |

### Success Criteria

- [ ] Elasticsearch index templates for `logs-*` and `anomalies` exist before the first document is written; a fresh deployment against an empty cluster creates both templates automatically on startup.
- [ ] Log entries consumed from Kafka appear in the `logs-{YYYY.MM.DD}` index within the configured bulk flush interval; re-indexing the same Kafka offsets produces no duplicate documents.
- [ ] Detected anomalies are indexed to the `anomalies` index and are queryable by service, rule ID, and time range.
- [ ] A configured SMTP recipient receives an email alert when an anomaly is triggered; the email body contains anomaly type, service, detection time, threshold breached, severity, and at least one sample log line.
- [ ] An SMTP send failure (unreachable server) logs an error and increments the failure metric but does not interrupt log ingestion or anomaly indexing.

### Dependencies

- Phase 1 (domain types, `LogStore` / `AnomalyStore` interfaces, observability)
- Phase 2 (detection engine, `Anomaly` type, cooldown state)

---

### Phase 4: REST API

**Goal**: Expose a chi-based HTTP API that allows operators to query persisted logs and anomalies with filtering and pagination, and provides health, readiness, and Prometheus metrics endpoints for container orchestration.
**Status:** Pending
**Depends on**: Phase 3
**Requirements**: API-01, API-02, API-03, API-04, API-05, API-06

### Plans

| # | Plan | Description |
|---|------|-------------|
| 04-01 | Router, middleware, and health endpoints | Initialise `github.com/go-chi/chi/v5` router in `internal/api`; apply `middleware.Recoverer`, `middleware.RequestID`, `middleware.Timeout`, and structured Zap request-logging middleware. Implement `GET /health` (returns 200 when Kafka and ES are reachable, 503 otherwise) and `GET /ready` (returns 200 once the pipeline is fully initialised). Mount `promhttp.Handler()` at `GET /metrics`. Run API server in its own goroutine under the shared `errgroup` context. |
| 04-02 | Log query endpoints | Implement `GET /api/v1/logs`: query Elasticsearch `logs-*` with optional `?service=`, `?level=`, `?from=` (RFC 3339), `?to=`, and pagination via `?page=` / `?size=` (capped at 1000); return `{"data": [...], "total": N, "page": P, "size": S}`. Implement `GET /api/v1/logs/{id}`: retrieve a single log entry by ID; return 404 with a structured error body if not found. |
| 04-03 | Anomaly query endpoints | Implement `GET /api/v1/anomalies`: query the `anomalies` index with optional `?type=`, `?service=`, `?severity=`, `?from=`, `?to=`, and pagination; return the same envelope structure as the log endpoint. Implement `GET /api/v1/anomalies/{id}`: retrieve a single anomaly by ID; return 404 with structured error body if not found. |
| 04-04 | API handler unit tests | Unit-test all four query handlers using `httptest.NewRecorder` with mock `LogStore` and `AnomalyStore` implementations; cover valid queries, missing resources (404), invalid query parameters (400), and Elasticsearch error propagation (500). All tests run with `-race`. |

### Success Criteria

- [ ] `GET /api/v1/logs?service=api&level=error&from=2026-03-21T00:00:00Z` returns a JSON array of matching log entries with correct `total` and `page` metadata.
- [ ] `GET /api/v1/logs/{id}` returns a single log document for a valid ID and a 404 JSON error body for an unknown ID.
- [ ] `GET /api/v1/anomalies?severity=high` returns anomaly records filtered by severity with correct pagination envelope.
- [ ] `GET /health` returns 503 when Elasticsearch is unreachable and 200 when it is available.
- [ ] `GET /metrics` returns a Prometheus text response including at least `logs_consumed_total` and `anomalies_detected_total`.

### Dependencies

- Phase 1 (Zap, Prometheus, domain types, `LogStore` / `AnomalyStore` interfaces)
- Phase 3 (Elasticsearch implementations of `LogStore` and `AnomalyStore`)

---

### Phase 5: Integration and Hardening

**Goal**: Validate the end-to-end pipeline against real infrastructure using testcontainers, harden graceful shutdown sequencing, and produce a setup README so the project can be cloned and run from scratch.
**Status:** Pending
**Depends on**: Phase 4
**Requirements**: TEST-02

### Plans

| # | Plan | Description |
|---|------|-------------|
| 05-01 | Testcontainers integration test suite | Write `//go:build integration` tests in `internal/integration` using `testcontainers-go` v0.41.0 Kafka and Elasticsearch modules; spin up one shared `TestMain`-scoped container pair per package (not per test). Test scenarios: publish log messages to Kafka topic → assert documents appear in `logs-*` ES index; publish log pattern matching the error-rate rule threshold → assert anomaly document created in `anomalies` index; verify offset commitment on restart (consumer resumes from correct position after kill/restart cycle); assert no duplicate documents for replayed offsets. |
| 05-02 | Graceful shutdown hardening | Audit and enforce the correct LIFO shutdown sequence: `signal.NotifyContext` cancels root context → Kafka `PollFetches` returns → parser pool drains → processor drains → tee channels close → detector and log indexer drain → alerter drains → `BulkIndexer.Close()` flushes remaining documents → `errgroup.Wait()` returns (Pitfall K5). Add a shutdown integration test that measures wall-clock time from SIGTERM to process exit and asserts it is under 10 seconds. Verify no goroutine leaks via `goleak` in all integration tests. |
| 05-03 | Pipeline wiring validation and README | Wire all components in `cmd/server/main.go` using the wiring function from ARCHITECTURE.md; confirm the full pipeline (`ingest → parse → enrich → tee → [detect → alert → store-anomaly, store-log]`) runs end-to-end in the integration test suite. Write a `README.md` covering: prerequisites (Go, Docker), dependency installation commands, `docker compose up` for Kafka and Elasticsearch, running the service, running unit tests (`go test -race ./...`), and running integration tests (`go test -race -tags integration ./...`). |

### Success Criteria

- [ ] Running `go test -race -tags integration ./...` against a Docker-available host completes successfully with Kafka and Elasticsearch containers started automatically by testcontainers.
- [ ] A log message published to Kafka during the integration test is retrievable via `GET /api/v1/logs/{id}` within 5 seconds.
- [ ] A synthetic log burst that satisfies the error-rate rule threshold produces an `Anomaly` document in the `anomalies` Elasticsearch index within the detection window.
- [ ] The process exits cleanly within 10 seconds of receiving SIGTERM with no goroutine leaks reported by `goleak` and no uncommitted Kafka offsets lost.
- [ ] A developer with Go and Docker installed can follow the README to run the full pipeline locally without consulting any other document.

### Dependencies

- Phase 1 (consumer, parser, observability)
- Phase 2 (detection engine)
- Phase 3 (Elasticsearch, alerter)
- Phase 4 (REST API, health endpoints)

---

## Progress

**Execution order:** Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation and Ingestion | 5/5 | Complete   | 2026-03-21 |
| 2. Detection Engine | 3/5 | In Progress|  |
| 3. Storage and Alerting | 0/4 | Not started | - |
| 4. REST API | 0/4 | Not started | - |
| 5. Integration and Hardening | 0/3 | Not started | - |

---

## Summary

| Phase | Name | Plans | Requirements | Status |
|-------|------|-------|--------------|--------|
| 1 | Foundation and Ingestion | 5 | INGEST-01-04, PARSE-01-04, OBS-01-02, TEST-01 | Planned |
| 2 | Detection Engine | 5 | DETECT-01-10 | Planned |
| 3 | Storage and Alerting | 4 | STORE-01-04, ALERT-01-03 | Pending |
| 4 | REST API | 4 | API-01-06 | Pending |
| 5 | Integration and Hardening | 3 | TEST-02 | Pending |

**Total:** 5 phases, 21 plans, 34 v1 requirements mapped, 0 unmapped.
