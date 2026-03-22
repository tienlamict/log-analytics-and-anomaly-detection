# Requirements: Log Analytics and Anomaly Detection

**Defined:** 2026-03-21
**Core Value:** Detect anomalies in application logs in real-time and surface them via alerts and a queryable API — so problems are caught before users report them.

---

## v1 Requirements

### Ingestion

- [x] **INGEST-01**: System consumes log messages from a Kafka topic using a consumer group with at-least-once semantics
- [x] **INGEST-02**: Consumer commits offsets only after successful downstream processing (no silent data loss on crash)
- [x] **INGEST-03**: Consumer handles graceful shutdown — drains in-flight messages before closing on SIGTERM
- [x] **INGEST-04**: Consumer supports configurable initial offset (oldest vs newest) for first-deploy and replay scenarios

### Parsing

- [x] **PARSE-01**: Parser normalizes structured JSON log entries into a canonical `LogEntry` schema (`id`, `timestamp`, `level`, `service`, `message`, `fields`)
- [x] **PARSE-02**: Parser handles unstructured / plain-text log lines as a fallback (does not drop unparseable messages)
- [x] **PARSE-03**: Log level variants are normalized (`err`, `ERR`, `FATAL` → `ERROR`; `warn`, `WARNING` → `WARN`)
- [x] **PARSE-04**: Parse errors are logged and metered; pipeline continues processing remaining messages

### Detection

- [x] **DETECT-01**: Detection engine evaluates incoming log entries against registered rules using a sliding time window
- [x] **DETECT-02**: Rule: **Error Rate Spike** — fires when error-level log count for a service exceeds threshold within a window
- [x] **DETECT-03**: Rule: **Latency Threshold Breach** — fires when `duration_ms` field exceeds threshold for >N% of requests in window (requires `duration_ms` field present)
- [x] **DETECT-04**: Rule: **Repeated Failure** — fires when the same error (by normalized message fingerprint) repeats >N times in window
- [x] **DETECT-05**: Rule: **Auth Failure Burst** — fires when auth failure events for a single IP or user exceed threshold in window
- [x] **DETECT-06**: Rule: **Off-Hours Access** — fires when access to configured sensitive endpoints occurs outside defined business hours
- [x] **DETECT-07**: Rule: **Service Silence** — fires when a known-active service emits no logs for longer than a configurable threshold (timer-based, not event-driven)
- [x] **DETECT-08**: Alert deduplication — per-rule cooldown window prevents repeated alerts for the same ongoing condition
- [x] **DETECT-09**: Detection rules and thresholds are configurable via YAML/JSON config file (no code changes required to tune thresholds)
- [x] **DETECT-10**: Window state is cleaned up for inactive services to prevent unbounded memory growth

### Storage

- [x] **STORE-01**: Raw log entries are indexed to Elasticsearch (`logs-{YYYY.MM.DD}` daily-rollover index) via bulk indexer
- [x] **STORE-02**: Detected anomalies are indexed to a separate Elasticsearch index (`anomalies`)
- [x] **STORE-03**: Elasticsearch index mappings are applied at startup (self-contained deployment, no manual DevOps setup)
- [x] **STORE-04**: Bulk indexer write failures are logged and metered; pipeline continues processing

### Alerting

- [x] **ALERT-01**: Email alert is sent via SMTP when an anomaly is detected (after cooldown check)
- [x] **ALERT-02**: Alert email includes: anomaly type, affected service, detection time, threshold breached, severity, and sample log lines
- [x] **ALERT-03**: SMTP configuration (host, port, credentials, recipient) is externalized via config/environment

### API

- [x] **API-01**: REST API endpoint `GET /api/v1/logs` — query logs by service, level, and time range with pagination
- [x] **API-02**: REST API endpoint `GET /api/v1/logs/{id}` — retrieve a single log entry by ID
- [x] **API-03**: REST API endpoint `GET /api/v1/anomalies` — query anomalies by type, service, severity, and time range with pagination
- [x] **API-04**: REST API endpoint `GET /api/v1/anomalies/{id}` — retrieve a single anomaly by ID
- [x] **API-05**: Health endpoints `GET /health` and `GET /ready` for container orchestration
- [x] **API-06**: Metrics endpoint `GET /metrics` exposing Prometheus metrics

### Observability

- [x] **OBS-01**: Structured logging via Zap throughout the pipeline (not fmt.Printf)
- [x] **OBS-02**: Prometheus metrics: logs consumed, processing latency, anomalies detected by rule, ES write errors, alerts sent/failed, Kafka consumer lag

### Testing

- [x] **TEST-01**: Unit tests for parser, detection rules, and alert deduplication logic (no external dependencies)
- [ ] **TEST-02**: Integration tests using testcontainers (real Kafka + Elasticsearch) covering the end-to-end pipeline

---

## v2 Requirements

### Alerting Expansion

- **ALERT-V2-01**: Webhook / Slack alerting channel
- **ALERT-V2-02**: Alert routing rules (route by severity, service, or rule type to different channels)
- **ALERT-V2-03**: Anomaly suppression API — mute alerts for a service for N minutes (planned maintenance)

### Detection Expansion

- **DETECT-V2-01**: Statistical / ML-based anomaly detection (Z-score, rolling baseline)
- **DETECT-V2-02**: Error rate ratio rule (errors / total requests with minimum volume guard)
- **DETECT-V2-03**: Anomaly rate trending ("3x more errors than same time yesterday")

### Operational

- **OPS-V2-01**: Rule hot-reload via `fsnotify` (tune thresholds without restart)
- **OPS-V2-02**: Dead letter queue for unparseable log messages (route to `logs-dlq` index)
- **OPS-V2-03**: Consumer lag alerting (pipeline falling behind)
- **OPS-V2-04**: Request/correlation ID threading in API responses

---

## Out of Scope

| Feature | Reason |
|---------|--------|
| Dashboard / UI | REST API is the v1 interface; doubles project surface area |
| Multi-tenancy | Single-tenant design; add when a second team needs to share the system |
| Log agent / shipper | Services push directly to Kafka; use Filebeat if needed — not our concern |
| Real-time streaming API (SSE/WebSocket) | Poll the REST API; streaming is v2 when low-latency UI is needed |
| Log archival / cold storage tiering | Configure ES ILM delete policy at 30 days; revisit when storage cost is a problem |
| Distributed tracing integration | Store `trace_id` as a field; actual Jaeger/Zipkin correlation is v2 |
| Geo-IP / location-based anomalies | Requires MaxMind or similar; off-hours access is sufficient for v1 security |
| High-availability / clustering | Single consumer process with Kafka offset management is sufficient at <1k/sec |
| Complex query DSL / aggregations API | Pre-defined query endpoints only; avoid generic query builders in v1 |

---

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INGEST-01 – INGEST-04 | Phase 1 | Pending |
| PARSE-01 – PARSE-04 | Phase 1 | Pending |
| OBS-01 – OBS-02 | Phase 1 | Pending |
| TEST-01 | Phase 1 | Complete |
| DETECT-01 – DETECT-10 | Phase 2 | Pending |
| STORE-01 – STORE-04 | Phase 3 | Pending |
| ALERT-01 – ALERT-03 | Phase 3 | Pending |
| API-01 – API-06 | Phase 4 | Pending |
| TEST-02 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 34 total
- Mapped to phases: 34
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-21*
*Last updated: 2026-03-21 after initial definition*
