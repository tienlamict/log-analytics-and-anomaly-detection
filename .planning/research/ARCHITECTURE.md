# Architecture Research: Log Analytics & Anomaly Detection

**Project:** Log Analytics and Anomaly Detection
**Researched:** 2026-03-21
**Updated:** 2026-04-08 — removed SMTP email alerting; added Prometheus metrics server and Grafana observability stack
**Confidence:** HIGH — Go pipeline patterns are well-established; specific library APIs verified against pkg.go.dev

---

## High-Level Architecture

```
┌─────────────┐      ┌──────────────────────────────────────────────────────┐      ┌───────────────────┐
│   Kafka     │      │                   Go Pipeline Process                 │      │  Elasticsearch    │
│  (topic:    │─────>│  Consumer → Parser → Worker → Detector → Dispatcher  │─────>│  (logs index +    │
│  app-logs)  │      │                                                       │      │  anomalies index) │
└─────────────┘      └────────────────────────┬─────────────────────────────┘      └───────────────────┘
                                              │
                              ┌───────────────┼───────────────┐
                              │               │               │
                       ┌──────┴──────┐ ┌──────┴──────┐ ┌──────┴──────┐
                       │  REST API   │ │  Prometheus  │ │   Grafana   │
                       │  :8080      │ │  :9090       │ │  :3000      │
                       └─────────────┘ └──────────────┘ └─────────────┘
                                              ▲
                                    scrapes :2112/metrics
```

Expanded pipeline view with concurrency layers:

```
Kafka Broker
    │
    │  PollFetches (franz-go, consumer group)
    ▼
┌─────────────────────────────────────────────────┐
│  Consumer (1 goroutine)                          │
│  kgo.PollFetches → *kgo.Record                  │
└───────────────────┬─────────────────────────────┘
                    │  chan RawMessage (buffered, 10 000)
                    ▼
┌─────────────────────────────────────────────────┐
│  Worker Pool (4 goroutines)                      │
│  Parse → IndexLog → Evaluate → metrics.Inc()    │
└───────────────────┬─────────────────────────────┘
                    │  chan Anomaly (via DetectorEngine)
                    ▼
┌─────────────────────────────────────────────────┐
│  Dispatcher (1 goroutine)                        │
│  AnomalyStore.IndexAnomaly()                     │
│  AlertChannel.Send()  ← nil in production        │
└─────────────────────────────────────────────────┘

Separate goroutines (same errgroup):
┌─────────────────┐  ┌──────────────────┐
│  REST API       │  │  Metrics Server  │
│  :8080          │  │  :2112/metrics   │
│  (Gin HTTP)     │  │  (promhttp)      │
└─────────────────┘  └──────────────────┘
```

---

## Component Breakdown

### 1. Consumer

**Responsibility:** Connect to Kafka, consume messages from the `application-logs` topic using a consumer group, and emit raw byte payloads for downstream processing.

**Library:** `github.com/twmb/franz-go/pkg/kgo`

**Inputs:** Kafka topic records (`*kgo.Record`)
**Outputs:** `chan RawMessage` (buffered, capacity 10 000)

**Key decisions:**
- One long-running goroutine calling `client.PollFetches(ctx)` in a loop.
- Offsets marked for commit only after the record is successfully sent into the output channel (at-least-once semantics). Uses `client.MarkCommitRecords` + `AutoCommitMarks`.
- `OnPartitionsRevoked` callback commits marked offsets before rebalance completes.
- After each fetch, per-partition lag is computed from `FetchPartition.HighWatermark` and the last record offset, then recorded to the `kafka_consumer_lag` Prometheus gauge.
- Shutdown: context cancellation causes `PollFetches` to return; consumer closes the output channel.

```go
type RawMessage struct {
    Payload   []byte
    Topic     string
    Partition int32
    Offset    int64
    Timestamp time.Time
}
```

---

### 2. Parser

**Responsibility:** Deserialize raw bytes into a normalized `LogEntry` struct. Handles both structured (JSON) and unstructured (plain text) log formats.

**Inputs:** `domain.RawMessage`
**Outputs:** `domain.LogEntry`

**Key decisions:**
- Pure function `Parse(RawMessage) (LogEntry, error)` — no state, trivially testable.
- Parse errors are non-fatal: a plain-text fallback `LogEntry` is always returned alongside a non-nil error; `parse_errors_total` Prometheus counter is incremented by the calling worker.
- JSON parsing with `encoding/json`; timestamp parsed as RFC3339, defaulting to `time.Now()` on failure.
- Log level normalised to canonical values: `error`, `warn`, `info`, `debug`, `unknown`.

```go
type LogEntry struct {
    ID        string         // UUID generated at ingest
    Timestamp time.Time
    Level     string         // "error", "warn", "info", "debug", "unknown"
    Service   string
    Message   string
    Fields    map[string]any // arbitrary key-value pairs from JSON "fields"
    RawSource string         // original unparsed payload
    Source    RawMessage     // for offset tracking
}
```

---

### 3. Worker Pool

**Responsibility:** Fan the raw message channel across 4 goroutines, each performing the full per-record sequence: parse → index log → detect → record metrics.

**Inputs:** `<-chan domain.RawMessage` (from Consumer)
**Outputs:** calls `LogIndexer.IndexLog`, `DetectorEngine.Evaluate`, increments Prometheus counters

**Key decisions:**
- 4 workers saturate physical CPU cores while leaving threads free for Kafka polling and ES bulk flush goroutines.
- `range consumer.Messages()` distributes naturally across goroutines — Go runtime handles scheduling.
- `logs_consumed_total{topic, partition}` is incremented per record after successful channel receive.
- `logs_processed_duration_seconds` histogram observes end-to-end latency per record (parse + ES index + evaluate).

---

### 4. Detector

**Responsibility:** Apply rule-based anomaly detection against each log entry. Maintains time-windowed in-memory state (sliding windows, per-service counters). Emits `Anomaly` events when rules fire.

**Inputs:** `domain.LogEntry` (called synchronously by worker goroutines via `engine.Evaluate`)
**Outputs:** `chan domain.Anomaly` (via `DetectorEngine`)

**Key decisions:**
- `DetectorEngine` owns a goroutine that routes anomalies from the internal channel with cooldown enforcement — prevents alert storms for the same rule+service pair.
- Rules run sequentially per entry inside the worker goroutine. Rule state is protected by a mutex inside `DetectorEngine`.
- Window state uses an in-memory circular timestamp buffer (`slidingWindow`) with periodic eviction.
- Cooldown window (default 15 min) suppresses duplicate anomalies per `(RuleID, Service)`.

**Implemented rules:**

| Rule | Trigger |
|---|---|
| `error_rate_spike` | Error-level log count per service exceeds threshold within window |
| `latency_threshold` | `fields.latency_ms` breach rate per service exceeds configured % |
| `repeated_failure` | Same service/endpoint fails N times within window |
| `auth_failure_burst` | Auth failures from a single IP or username exceed threshold |
| `off_hours_access` | Sensitive path accessed outside configured business hours |
| `service_silence` | A previously active service emits no logs for `silence_after` duration |

```go
type Detector interface {
    Name() string
    Evaluate(entry LogEntry) (Anomaly, bool)
    Reset()
}
```

---

### 5. Dispatcher

**Responsibility:** Consume detected anomalies, persist them to Elasticsearch, and optionally forward to an alert channel. Failure in one output does not block the other.

**Inputs:** `<-chan domain.Anomaly`
**Outputs:** `AnomalyStore.IndexAnomaly`, `AlertChannel.Send` (optional)

**Key decisions:**
- Single goroutine; no rate-limiting needed here — cooldown is enforced upstream by `DetectorEngine`.
- `AlertChannel` is wired as `nil` in production. The interface is retained for future notification integrations (webhook, PagerDuty, etc.) without changing the dispatcher.
- `anomalies_detected_total{rule}` Prometheus counter is incremented by the detection engine before the anomaly reaches the dispatcher.

```go
type AlertChannel interface {
    Send(ctx context.Context, anomaly Anomaly) error
    Name() string
}
```

---

### 6. Indexer (Elasticsearch)

**Responsibility:** Persist `LogEntry` records to the `logs-{YYYY.MM.DD}` index and `Anomaly` records to the `anomalies` index.

**Library:** `github.com/elastic/go-elasticsearch/v8` with `esutil.BulkIndexer`

**Inputs:** `LogEntry` (log indexer), `Anomaly` (anomaly indexer)
**Outputs:** Elasticsearch documents

**Key decisions:**
- Two separate `BulkIndexer` instances: one for `logs-{YYYY.MM.DD}` (daily rollover), one for `anomalies`.
- `esutil.BulkIndexer` batches internally, flushes on byte threshold or interval — far more efficient than individual index calls.
- Index mappings set via index templates applied at startup (`ApplyIndexTemplates`).
- `elasticsearch_write_errors_total` Prometheus counter is incremented on bulk item failure via the `OnFailure` callback.
- `Close(ctx)` flushes remaining buffered documents during graceful shutdown.

**Index schemas:**

`logs-{YYYY.MM.DD}`:
```json
{
  "@timestamp": "2026-04-08T10:00:00Z",
  "level": "error",
  "service": "api-gateway",
  "message": "connection timeout to upstream",
  "fields": { "latency_ms": 5000, "source_ip": "10.0.0.1" },
  "raw_source": "<original log line>"
}
```

`anomalies`:
```json
{
  "id": "uuid",
  "rule_id": "error_rate_spike",
  "severity": "high",
  "service": "api-gateway",
  "description": "error rate spike: 12 errors in 5m for service api-gateway",
  "detected_at": "2026-04-08T10:01:00Z",
  "evidence": [{ ... }]
}
```

---

### 7. Prometheus Metrics Server

**Responsibility:** Expose runtime metrics on a dedicated port for Prometheus scraping. Completely separate from the REST API server.

**Library:** `github.com/prometheus/client_golang/prometheus/promhttp`

**Endpoint:** `GET :2112/metrics`

**Registered metrics:**

| Metric | Type | Labels | Description |
|---|---|---|---|
| `logs_consumed_total` | Counter | `topic`, `partition` | Kafka records consumed |
| `logs_processed_duration_seconds` | Histogram | — | End-to-end processing latency per record |
| `anomalies_detected_total` | Counter | `rule` | Anomalies detected, by rule |
| `kafka_consumer_lag` | Gauge | `topic`, `partition` | Consumer lag per partition |
| `elasticsearch_write_errors_total` | Counter | — | ES bulk write failures |
| `parse_errors_total` | Counter | — | Plain-text fallback parse events |

---

### 8. Grafana

**Responsibility:** Visualise metrics from Prometheus with a pre-built provisioned dashboard.

**Access:** `http://localhost:3000` (credentials: `admin / admin123`)

**Provisioning:**
- Datasource: `grafana/provisioning/datasources/prometheus.yml` — points to `http://prometheus:9090`, fixed `uid: prometheus`
- Dashboard provider: `grafana/provisioning/dashboards/dashboard.yml` — loads from `/var/lib/grafana/dashboards`
- Dashboard definition: `grafana/dashboards/log-analytics.json`

**Dashboard panels:**

| Panel | Query | Type |
|---|---|---|
| Log Ingestion Rate | `sum(rate(logs_consumed_total[1m]))` | Time series |
| Processing Latency | `histogram_quantile(0.50/0.95/0.99, rate(logs_processed_duration_seconds_bucket[5m]))` | Time series |
| Anomaly Detection Rate | `rate(anomalies_detected_total[5m])` | Time series |
| Anomalies by Rule | `sum by(rule)(anomalies_detected_total)` | Pie chart |
| Kafka Consumer Lag | `kafka_consumer_lag` | Time series |
| Elasticsearch Write Errors | `increase(elasticsearch_write_errors_total[1h])` | Stat |
| Parse Errors | `increase(parse_errors_total[1h])` | Stat |

---

### 9. REST API

**Responsibility:** Expose HTTP endpoints for querying logs and anomalies stored in Elasticsearch.

**Library:** `net/http` (standard library; no external router framework)

**Port:** `:8080`

**Endpoints:**
```
GET  /api/v1/logs            ?service=, ?level=, ?from=, ?to=, ?q= (full-text), ?page=, ?size=
GET  /api/v1/logs/:id
GET  /api/v1/anomalies       ?severity=, ?rule=, ?service=, ?from=, ?to=, ?page=, ?size=
GET  /api/v1/anomalies/:id
GET  /health                 Elasticsearch ping
GET  /ready                  Atomic ready flag (used by Docker healthcheck)
```

**Key decisions:**
- Runs in its own goroutine, independent of pipeline goroutines.
- Read-only — the pipeline is the sole data producer.
- Pagination via `page`/`size` (mapped to ES `from`/`size`).

---

## Concurrency Model

The system uses an **errgroup-based staged pipeline** where goroutines are started once and communicate through channels and direct calls. All goroutines share a single `errgroup` context — the first fatal error cancels all others.

### Goroutine Map

| Goroutine | Count | Communication |
|---|---|---|
| Kafka consumer | 1 | writes `chan RawMessage` |
| Pipeline worker | 4 | reads `chan RawMessage`; calls ES, detector directly |
| Anomaly dispatcher | 1 | reads `chan Anomaly` from `DetectorEngine` |
| REST API server | 1 | reads from ES on demand |
| Prometheus metrics server | 1 | serves `/metrics` |
| Shutdown watcher | 1 | waits for ctx, calls graceful stops |

### Lifecycle Management

```go
g, gCtx := errgroup.WithContext(ctx)

g.Go(func() error { return consumer.Run(gCtx) })
for range 4 {
    g.Go(func() error { /* worker: parse → index → evaluate */ })
}
g.Go(func() error { return dispatcher.Run(gCtx) })
g.Go(func() error { return apiServer.ListenAndServe() })
g.Go(func() error { return metricsServer.ListenAndServe() })
g.Go(func() error {
    <-gCtx.Done()
    // graceful shutdown: api, metrics, engine, indexers
})

return g.Wait()
```

### Shutdown Sequence

1. OS signal (SIGINT/SIGTERM) cancels root context.
2. Kafka consumer's `PollFetches` returns; consumer closes `rawMessages` channel.
3. Worker goroutines drain remaining messages from closed channel, then return.
4. `DetectorEngine.Stop()` closes the anomaly channel.
5. Dispatcher drains remaining anomalies, returns.
6. API and metrics servers receive `Shutdown(ctx)` with 8-second timeout.
7. Log and anomaly `BulkIndexer.Close()` flushes pending ES documents.
8. `errgroup.Wait()` returns nil (or the first non-`context.Canceled` error).

---

## Interface Design

All key extension points are behind interfaces defined in `internal/domain`.

```go
// MessageConsumer reads from the message source
type MessageConsumer interface {
    Run(ctx context.Context) error
    Messages() <-chan RawMessage
}

// Parser converts raw bytes to a structured log entry
type Parser interface {
    Parse(msg RawMessage) (LogEntry, error)
}

// Detector evaluates a log entry against a single anomaly rule
type Detector interface {
    Name() string
    Evaluate(entry LogEntry) (Anomaly, bool)
    Reset()
}

// AlertChannel sends anomaly notifications (nil in production — reserved for future use)
type AlertChannel interface {
    Send(ctx context.Context, anomaly Anomaly) error
    Name() string
}

// LogStore persists and queries log entries
type LogStore interface {
    IndexLog(ctx context.Context, entry LogEntry) error
    SearchLogs(ctx context.Context, query LogQuery) ([]LogEntry, int64, error)
    GetLog(ctx context.Context, id string) (LogEntry, error)
}

// AnomalyStore persists and queries anomalies
type AnomalyStore interface {
    IndexAnomaly(ctx context.Context, anomaly Anomaly) error
    SearchAnomalies(ctx context.Context, query AnomalyQuery) ([]Anomaly, int64, error)
    GetAnomaly(ctx context.Context, id string) (Anomaly, error)
}
```

---

## Configuration Shape (YAML via Viper)

```yaml
kafka:
  brokers:
    - "kafka:29092"
  topic: "application-logs"
  group_id: "log-analytics"
  initial_offset: "oldest"       # "oldest" | "newest"

elasticsearch:
  addresses:
    - "http://elasticsearch:9200"
  username: ""
  password: ""
  max_idle_conns: 50
  response_timeout: "15s"

metrics:
  port: 2112                     # Prometheus scrape endpoint

detection:
  window_duration: "5m"
  eviction_interval: "30s"
  cooldown_duration: "15m"
  warmup_multiplier: 2
  rules:
    error_rate:
      enabled: true
      threshold: 10
      window: "5m"
      severity: "high"
    latency:
      enabled: true
      threshold_ms: 500
      breach_rate_percent: 20
      window: "5m"
      severity: "medium"
    repeated_failure:
      enabled: true
      threshold: 5
      window: "5m"
      severity: "medium"
    auth_burst:
      enabled: true
      threshold: 10
      window: "5m"
      ip_field: "source_ip"
      user_field: "username"
      severity: "high"
    off_hours:
      enabled: true
      business_hours_start: 9
      business_hours_end: 17
      timezone: "UTC"
      sensitive_paths: ["/admin", "/api/v1/users", "/internal"]
      severity: "medium"
    service_silence:
      enabled: true
      silence_after: "5m"
      check_interval: "30s"
      min_log_count: 5
      severity: "critical"

api:
  port: 8080

log:
  level: "info"
```

---

## Error Handling Strategy

| Error Type | Handling | Rationale |
|---|---|---|
| Parse failure | Emit plain-text `LogEntry`, increment `parse_errors_total`, continue | Malformed logs must not stop the pipeline |
| Elasticsearch index failure | Log error, increment `elasticsearch_write_errors_total`, continue | Pipeline throughput takes priority; documents are best-effort in v1 |
| Kafka consumer error | Log, continue — franz-go handles reconnect internally | Transient broker issues must not crash the pipeline |
| Rule evaluation panic | `recover()` in engine loop, log stack trace, continue | Bugs in detection rules must not crash the pipeline |
| Kafka partition rebalance | `OnPartitionsRevoked` commits marked offsets before returning | Prevents offset regression / duplicate processing after reassignment |

**At-least-once semantics:** Offsets are committed only after the record enters the `rawMessages` channel. On restart the consumer re-reads from the last committed offset. Elasticsearch document IDs are stable UUIDs generated at parse time — duplicate submissions on replay are idempotent.

---

## Package Structure

```
.
├── cmd/
│   ├── server/
│   │   ├── main.go             # Entrypoint: config load, logger, signal handling
│   │   └── wire.go             # Wiring: constructs all components, runs errgroup
│   └── loadtest/
│       └── main.go             # Load generator: 5-min phased Kafka producer for manual testing
├── internal/
│   ├── config/                 # Config structs, Viper loading, defaults
│   ├── domain/                 # Core types (LogEntry, Anomaly, RawMessage) and interfaces — zero external deps
│   ├── kafka/                  # Consumer implementation (franz-go); lag metric reporting
│   ├── pipeline/               # Parser (Parse func + LogParser); ProcessMessage helper
│   ├── detection/              # DetectorEngine + all rule implementations + sliding window
│   ├── alert/                  # Dispatcher: anomaly fan-out to AnomalyStore and AlertChannel
│   ├── metrics/                # Prometheus metric variable declarations and init() registration
│   ├── elasticsearch/          # LogIndexer, AnomalyIndexer (BulkIndexer), index template setup, ES client
│   └── api/                    # HTTP server, handlers (/logs, /anomalies, /health, /ready), middleware
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/prometheus.yml   # Prometheus datasource (uid: prometheus)
│   │   └── dashboards/dashboard.yml     # Dashboard file provider config
│   └── dashboards/
│       └── log-analytics.json           # Pre-built Grafana dashboard (7 panels)
├── docs/
│   └── elasticsearch-query-guide.md    # ES query reference for logs and anomalies indices
├── prometheus.yml              # Prometheus scrape config (scrapes app:2112)
├── docker-compose.yml          # Full stack: kafka + kafka-init + elasticsearch + app + prometheus + grafana
└── config.docker.yaml          # Runtime config for Docker deployment
```

**Key boundary:** `internal/domain` has zero external dependencies. All other packages import domain types inward; no cross-imports between sibling packages (enforced by wiring only in `cmd/`).

---

## Sources

- Go Pipelines blog post (official): https://go.dev/blog/pipelines
- franz-go consumer API: https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo
- go-elasticsearch v8 BulkIndexer: https://pkg.go.dev/github.com/elastic/go-elasticsearch/v8/esutil
- errgroup: https://pkg.go.dev/golang.org/x/sync/errgroup
- Prometheus Go client: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus
- Grafana provisioning docs: https://grafana.com/docs/grafana/latest/administration/provisioning/
- Viper configuration: https://pkg.go.dev/github.com/spf13/viper
