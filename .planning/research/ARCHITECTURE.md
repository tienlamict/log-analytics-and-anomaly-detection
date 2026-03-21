# Architecture Research: Log Analytics & Anomaly Detection

**Project:** Log Analytics and Anomaly Detection
**Researched:** 2026-03-21
**Confidence:** HIGH — Go pipeline patterns are well-established; specific library APIs verified against pkg.go.dev

---

## High-Level Architecture

```
┌─────────────┐      ┌──────────────────────────────────────────────────────────────────┐      ┌───────────────────┐
│   Kafka     │      │                      Go Pipeline Process                          │      │  Elasticsearch    │
│  (topic:    │─────>│  Consumer → Parser → Processor → Detector → Alerter → Indexer   │─────>│  (logs index +    │
│  app-logs)  │      │                                                                   │      │  anomalies index) │
└─────────────┘      └───────────────────────────────┬──────────────────────────────────┘      └───────────────────┘
                                                      │
                                               ┌──────┴──────┐
                                               │  REST API   │
                                               │  (Gin HTTP) │
                                               └─────────────┘
```

Expanded pipeline view with concurrency layers:

```
Kafka Broker
    │
    │  PollFetches (franz-go, consumer group)
    ▼
┌─────────────────────────────────────────────────┐
│  Consumer (1 goroutine per partition)            │
│  kgo.PollFetches → *kgo.Record                  │
└───────────────────┬─────────────────────────────┘
                    │  chan RawMessage (buffered)
                    ▼
┌─────────────────────────────────────────────────┐
│  Parser Worker Pool (N goroutines)              │
│  JSON/text → LogEntry struct                    │
└───────────────────┬─────────────────────────────┘
                    │  chan LogEntry (buffered)
                    ▼
┌─────────────────────────────────────────────────┐
│  Processor / Enricher (1 goroutine)             │
│  Normalize fields, attach metadata              │
└───────────────────┬─────────────────────────────┘
                    │  chan LogEntry (fan-out: two consumers)
          ┌─────────┴─────────┐
          ▼                   ▼
┌─────────────────┐  ┌────────────────────────────┐
│  ES Indexer     │  │  Detector (1 goroutine)     │
│  BulkIndexer    │  │  Rule engine, window state  │
│  (raw logs)     │  └──────────────┬─────────────┘
└─────────────────┘                 │  chan Anomaly
                                    ▼
                         ┌──────────────────────┐
                         │  Alerter (1 goroutine)│
                         │  SMTP + ES index      │
                         └──────────────────────┘
                                    │
                              ┌─────┴──────┐
                              │  ES Index  │
                              │ (anomalies)│
                              └────────────┘
```

---

## Component Breakdown

### 1. Consumer

**Responsibility:** Connect to Kafka, consume messages from the `app-logs` topic using a consumer group, and emit raw byte payloads for parsing.

**Library:** `github.com/twmb/franz-go/pkg/kgo` (preferred over Sarama — lower overhead, modern Kafka protocol support, idiomatic context cancellation).

**Inputs:** Kafka topic records (`*kgo.Record`)
**Outputs:** `chan RawMessage` (struct wrapping `[]byte` + metadata: topic, partition, offset, timestamp)

**Key decisions:**
- One long-running goroutine calling `client.PollFetches(ctx)` in a loop.
- Commits offsets after downstream processing confirms receipt (at-least-once semantics). Use `client.MarkCommitRecords` + `AutoCommitMarks`.
- `OnPartitionsRevoked` callback drains in-flight work before rebalance completes.
- Shutdown: context cancellation propagates upstream, `PollFetches` returns, consumer closes.

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

**Responsibility:** Deserialize raw bytes into a normalized `LogEntry` struct. Handle both structured (JSON) and unstructured (plain text) log formats.

**Inputs:** `<-chan RawMessage`
**Outputs:** `chan<- LogEntry`

**Key decisions:**
- Worker pool of N goroutines (N = `runtime.NumCPU()` as a starting default; tunable via config).
- Parse errors are non-fatal: emit a `ParseError` metric, drop the message, continue.
- Parser is a pure function `Parse([]byte) (LogEntry, error)` — no state, easy to test.
- JSON parsing with `encoding/json` for structured logs; regex-based extraction for unstructured.

```go
type LogEntry struct {
    ID        string            // UUID generated at ingest
    Timestamp time.Time
    Level     string            // "error", "warn", "info", "debug"
    Service   string
    Message   string
    Fields    map[string]any    // arbitrary key-value pairs
    RawSource string            // original unparsed line (for Elasticsearch storage)
    Source    RawMessage        // for offset commit tracking
}
```

---

### 3. Processor / Enricher

**Responsibility:** Apply normalization and enrichment to parsed log entries before fan-out to the detector and indexer. Examples: normalize log level casing, extract HTTP status codes from message strings, add ingestion timestamp.

**Inputs:** `<-chan LogEntry`
**Outputs:** `chan<- LogEntry` (same type, enriched)

**Key decisions:**
- Single goroutine. At <1k logs/sec this is not a bottleneck. Simplifies state if enrichment ever needs service metadata lookups.
- Enrichment steps implemented as a slice of `EnrichFunc` applied in order — easy to extend without modifying core logic.

```go
type EnrichFunc func(LogEntry) LogEntry
```

---

### 4. Detector

**Responsibility:** Apply rule-based anomaly detection against the stream of log entries. Maintain time-windowed state (sliding windows, counters). Emit `Anomaly` events when rules trigger.

**Inputs:** `<-chan LogEntry`
**Outputs:** `chan<- Anomaly`

**Key decisions:**
- Single goroutine owning all window state — avoids lock contention on shared counters.
- Rules implemented behind a `Detector` interface; multiple rules run sequentially per entry.
- Window state uses in-memory ring buffers or `time.Ticker`-based eviction (not Redis/external state — v1 is single-process).
- A rule that fires emits an `Anomaly`; the rule's own state is responsible for de-duplication (e.g., rate-limit alerts to once per 5 minutes per rule+service combination).

**Rule types to implement:**
- `ErrorRateRule`: count errors per service per rolling window; alert if rate > threshold
- `LatencyThresholdRule`: parse latency from log fields; alert if p99 exceeds threshold
- `RepeatedFailureRule`: N identical failures within window
- `AuthFailureRule`: auth failures from same IP within window
- `UnusualAccessRule`: access from IP not seen in prior rolling window

```go
type Anomaly struct {
    ID          string
    RuleID      string
    Severity    string  // "critical", "high", "medium", "low"
    Service     string
    Description string
    Evidence    []LogEntry  // contributing log entries
    DetectedAt  time.Time
}

type Detector interface {
    Name() string
    Evaluate(entry LogEntry) (Anomaly, bool)  // bool = anomaly detected
    Reset()                                    // for testing
}
```

---

### 5. Indexer (Elasticsearch)

**Responsibility:** Persist `LogEntry` records to the `logs` index and `Anomaly` records to the `anomalies` index in Elasticsearch.

**Library:** `github.com/elastic/go-elasticsearch/v8` with `esutil.BulkIndexer`

**Inputs:** `<-chan LogEntry` (for raw logs), `<-chan Anomaly` (for anomalies, tee'd from the alerter path)
**Outputs:** None (side effects: Elasticsearch documents)

**Key decisions:**
- Use `esutil.BulkIndexer` (not individual `Index` calls) — it internally batches requests, uses multiple worker goroutines, and flushes on byte threshold or interval.
- Two separate BulkIndexer instances: one for `logs-{YYYY.MM.DD}` (daily rollover index), one for `anomalies`.
- Index mapping defined up-front via index templates applied at startup.
- `OnFailure` callback on BulkIndexerItem logs the failure and increments an error metric. Do not retry individual documents in v1.

```go
type LogIndexer interface {
    IndexLog(ctx context.Context, entry LogEntry) error
    IndexAnomaly(ctx context.Context, anomaly Anomaly) error
    Close(ctx context.Context) error
}
```

---

### 6. Alerter

**Responsibility:** Receive `Anomaly` events, send email alerts via SMTP, and forward anomalies to the Indexer for persistence.

**Library:** `net/smtp` (standard library) for v1. Note: `net/smtp` is frozen and low-level; wrapping it in an `Alerter` interface means it can be swapped for `github.com/wneessen/go-mail` in v2 without changing callers.

**Inputs:** `<-chan Anomaly`
**Outputs:** SMTP email, `chan<- Anomaly` (forwarded to Indexer)

**Key decisions:**
- Single goroutine consuming the anomaly channel.
- In-process rate limiting per `(RuleID, Service)` pair — suppress duplicate alerts within a configurable window (default: 5 minutes).
- Email failures are non-fatal: log the error, continue processing. The anomaly is still persisted to Elasticsearch.
- Use a connection pool or reconnect-on-use pattern since SMTP connections are not persistent.

```go
type AlertChannel interface {
    Send(ctx context.Context, anomaly Anomaly) error
    Name() string
}

// Concrete implementations:
// SMTPAlerter implements AlertChannel
// NoopAlerter implements AlertChannel (for testing)
```

---

### 7. REST API

**Responsibility:** Expose HTTP endpoints for querying logs and anomalies stored in Elasticsearch.

**Library:** `github.com/gin-gonic/gin`

**Inputs:** HTTP requests
**Outputs:** JSON responses (read from Elasticsearch)

**Endpoints (v1):**
```
GET  /api/v1/logs                   - List logs, supports ?service=, ?level=, ?from=, ?to=, ?q= (full-text)
GET  /api/v1/logs/:id               - Get single log entry by ID
GET  /api/v1/anomalies              - List anomalies, supports ?severity=, ?rule=, ?service=, ?from=, ?to=
GET  /api/v1/anomalies/:id          - Get single anomaly by ID
GET  /api/v1/health                 - Health check (Kafka + ES connectivity)
```

**Key decisions:**
- API server runs in its own goroutine, independent of the pipeline goroutines.
- Elasticsearch queries use the `es.Search` API with structured query DSL bodies.
- Pagination via `from`/`size` parameters (Elasticsearch native). Deep pagination (>10k results) deferred to v2 via search_after.
- No write endpoints — the pipeline is the only data producer.

---

## Concurrency Model

The system uses a **staged pipeline** model where each stage communicates through buffered channels. This is the canonical Go pattern from the official pipelines blog post.

### Channel Sizing

| Channel | Buffer Size | Rationale |
|---------|-------------|-----------|
| `rawMessages` | 1000 | Absorb Kafka batch bursts; PollFetches returns batches |
| `parsedEntries` | 500 | Workers drain faster than consumer produces |
| `enrichedEntries` | 500 | After processor, two consumers read (fan-out via tee) |
| `anomalies` | 100 | Low volume — anomalies are rare relative to log volume |

At <1k logs/sec these buffers prevent back-pressure build-up during transient slowdowns (e.g., ES write latency spikes).

### Worker Pool Pattern (Parser)

```go
func startParserPool(ctx context.Context, in <-chan RawMessage, numWorkers int) <-chan LogEntry {
    out := make(chan LogEntry, 500)
    var wg sync.WaitGroup
    wg.Add(numWorkers)
    for i := 0; i < numWorkers; i++ {
        go func() {
            defer wg.Done()
            for {
                select {
                case msg, ok := <-in:
                    if !ok {
                        return
                    }
                    entry, err := parseRawMessage(msg)
                    if err != nil {
                        // log parse error, continue
                        continue
                    }
                    select {
                    case out <- entry:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
    go func() {
        wg.Wait()
        close(out)
    }()
    return out
}
```

### Fan-Out Tee (Processor → Detector + Indexer)

The enriched `LogEntry` stream must be consumed by two independent goroutines: the Detector and the Log Indexer. Use a tee function:

```go
func tee(ctx context.Context, in <-chan LogEntry) (<-chan LogEntry, <-chan LogEntry) {
    out1 := make(chan LogEntry, 500)
    out2 := make(chan LogEntry, 500)
    go func() {
        defer close(out1)
        defer close(out2)
        for {
            select {
            case entry, ok := <-in:
                if !ok {
                    return
                }
                // Send to both; block until both accept
                select {
                case out1 <- entry:
                }
                select {
                case out2 <- entry:
                }
            case <-ctx.Done():
                return
            }
        }
    }()
    return out1, out2
}
```

### Lifecycle Management with errgroup

All pipeline goroutines are started under a single `errgroup` with a shared context. The first goroutine to return an error cancels the context, which propagates shutdown through all channels.

```go
func (p *Pipeline) Run(ctx context.Context) error {
    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error { return p.consumer.Run(ctx) })
    g.Go(func() error { return p.parserPool.Run(ctx) })
    g.Go(func() error { return p.processor.Run(ctx) })
    g.Go(func() error { return p.detector.Run(ctx) })
    g.Go(func() error { return p.alerter.Run(ctx) })
    g.Go(func() error { return p.logIndexer.Run(ctx) })
    g.Go(func() error { return p.anomalyIndexer.Run(ctx) })

    return g.Wait()
}
```

### Shutdown Sequence

1. OS signal (SIGINT/SIGTERM) cancels root context.
2. Kafka consumer's `PollFetches` returns; consumer closes `rawMessages` channel.
3. Parser workers drain remaining messages, close `parsedEntries`.
4. Processor drains and closes its output channel.
5. Tee goroutine closes both fan-out channels.
6. Detector drains, closes `anomalies`. Indexer drains, calls `BulkIndexer.Close()`.
7. Alerter drains remaining anomalies.
8. `errgroup.Wait()` returns nil (or first error).

---

## Interface Design

All key extension points are behind interfaces. This enables:
- Swapping implementations without changing callers (e.g., SMTPAlerter → webhook alerter)
- Dependency injection for testing (NoopAlerter, in-memory indexer)

### Core Interfaces

```go
// Consumer reads from the message source
type MessageConsumer interface {
    Run(ctx context.Context) error
    Messages() <-chan RawMessage
}

// Parser converts raw bytes to structured log entries
type Parser interface {
    Parse(msg RawMessage) (LogEntry, error)
}

// Enricher modifies log entries in-place (no output channel needed)
type Enricher interface {
    Enrich(entry LogEntry) LogEntry
}

// Detector evaluates a log entry against a single rule
type Detector interface {
    Name() string
    Evaluate(entry LogEntry) (Anomaly, bool)
    Reset()
}

// AlertChannel sends anomaly notifications
type AlertChannel interface {
    Send(ctx context.Context, anomaly Anomaly) error
    Name() string
}

// LogStore persists and queries logs
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

### Detector Registry

Detectors are registered at startup and run sequentially per log entry in the Detector stage:

```go
type DetectorEngine struct {
    rules []Detector
}

func (e *DetectorEngine) Evaluate(entry LogEntry) []Anomaly {
    var results []Anomaly
    for _, rule := range e.rules {
        if anomaly, ok := rule.Evaluate(entry); ok {
            results = append(results, anomaly)
        }
    }
    return results
}
```

New detection rules are added by implementing `Detector` and registering in the wiring layer — no changes to the engine itself.

---

## Data Flow

### Log Entry Lifecycle

```
1. Kafka Record arrives
   └── kgo.Record{Value: []byte(`{"level":"error","service":"api","msg":"timeout"}`)}

2. RawMessage created
   └── RawMessage{Payload: bytes, Topic, Partition, Offset, Timestamp}

3. Parsed → LogEntry
   └── LogEntry{ID: uuid, Level: "error", Service: "api", Message: "timeout",
               Fields: {}, Timestamp: t, RawSource: original}

4. Enriched → LogEntry (same type)
   └── Fields enriched: HTTPStatusCode extracted, normalized Level

5a. Indexed → Elasticsearch logs-2026.03.21 index
    └── Document with all LogEntry fields + @timestamp

5b. Evaluated → Detector engine
    └── ErrorRateRule: error count for "api" in last 60s incremented
        └── Threshold exceeded → Anomaly{RuleID: "error-rate", Service: "api", ...}

6. Anomaly → Alerter
   └── Email sent to configured recipients
   └── Anomaly forwarded to Indexer

7. Anomaly → Indexed → Elasticsearch anomalies index
   └── Document with Anomaly fields + Evidence log IDs
```

### Elasticsearch Index Structure

**logs-{YYYY.MM.DD}** (daily index, covered by index template):
```json
{
  "id": "uuid",
  "@timestamp": "2026-03-21T10:00:00Z",
  "level": "error",
  "service": "api-gateway",
  "message": "connection timeout to upstream",
  "fields": { "http_status": 503, "duration_ms": 5000 },
  "raw_source": "<original log line>"
}
```

**anomalies**:
```json
{
  "id": "uuid",
  "rule_id": "error-rate-spike",
  "severity": "high",
  "service": "api-gateway",
  "description": "Error rate 45/min exceeds threshold 10/min",
  "evidence_log_ids": ["uuid1", "uuid2"],
  "detected_at": "2026-03-21T10:01:00Z"
}
```

---

## Configuration & Wiring

### Configuration Shape (YAML + env vars via Viper)

```yaml
kafka:
  brokers:
    - "localhost:9092"
  topic: "app-logs"
  consumer_group: "log-analytics"
  fetch_max_bytes: 10485760      # 10MB
  session_timeout: "30s"

elasticsearch:
  addresses:
    - "http://localhost:9200"
  username: ""
  password: ""
  log_index_prefix: "logs"       # final: logs-2026.03.21
  anomaly_index: "anomalies"
  bulk_flush_bytes: 5242880      # 5MB
  bulk_flush_interval: "10s"
  bulk_num_workers: 2

pipeline:
  parser_workers: 4
  raw_message_buffer: 1000
  parsed_entry_buffer: 500

detection:
  rules:
    error_rate:
      enabled: true
      window: "60s"
      threshold: 10              # errors/min per service
    latency_threshold:
      enabled: true
      field: "duration_ms"
      threshold_ms: 5000
    repeated_failure:
      enabled: true
      window: "300s"
      count: 5
    auth_failure:
      enabled: true
      window: "60s"
      count: 10
  alert_suppression_window: "5m"

alerting:
  email:
    smtp_host: "smtp.example.com"
    smtp_port: 587
    username: ""
    password: ""
    from: "alerts@example.com"
    to:
      - "oncall@example.com"

api:
  listen_addr: ":8080"
  read_timeout: "10s"
  write_timeout: "30s"
```

### Wiring Layer (main.go / wire.go)

The wiring layer is the only place that knows about concrete types. All other layers depend only on interfaces.

```go
func wire(cfg *Config) (*App, error) {
    // Infrastructure
    kafkaClient, err := newKafkaClient(cfg.Kafka)
    esClient, err := newESClient(cfg.Elasticsearch)

    // Stores (implement LogStore, AnomalyStore)
    logStore := elasticsearch.NewLogStore(esClient, cfg.Elasticsearch)
    anomalyStore := elasticsearch.NewAnomalyStore(esClient, cfg.Elasticsearch)

    // Alert channels (implement AlertChannel)
    emailAlerter := smtp.NewSMTPAlerter(cfg.Alerting.Email)

    // Detectors (implement Detector)
    detectors := []Detector{
        detection.NewErrorRateRule(cfg.Detection.Rules.ErrorRate),
        detection.NewLatencyRule(cfg.Detection.Rules.LatencyThreshold),
        detection.NewRepeatedFailureRule(cfg.Detection.Rules.RepeatedFailure),
        detection.NewAuthFailureRule(cfg.Detection.Rules.AuthFailure),
    }

    // Pipeline stages
    consumer := kafka.NewConsumer(kafkaClient, cfg.Pipeline)
    parserPool := pipeline.NewParserPool(cfg.Pipeline.ParserWorkers)
    processor := pipeline.NewProcessor(enrichers...)
    engine := detection.NewEngine(detectors)
    alerter := alert.NewAlerter([]AlertChannel{emailAlerter}, anomalyStore, cfg.Detection)

    // HTTP API
    api := api.New(logStore, anomalyStore, cfg.API)

    return &App{
        pipeline: pipeline.New(consumer, parserPool, processor, engine, alerter, logStore),
        api:      api,
    }, nil
}
```

### Application Entrypoint

```go
func main() {
    cfg := loadConfig()          // Viper: config.yaml + env vars
    app, err := wire(cfg)
    if err != nil { log.Fatal(err) }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    g, ctx := errgroup.WithContext(ctx)
    g.Go(func() error { return app.pipeline.Run(ctx) })
    g.Go(func() error { return app.api.Run(ctx) })

    if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
        log.Fatal(err)
    }
}
```

---

## Error Handling Strategy

| Error Type | Handling | Rationale |
|------------|----------|-----------|
| Parse failure | Log + drop message + continue | Malformed logs must not stop the pipeline |
| Elasticsearch index failure | Log + increment metric, BulkIndexer OnFailure callback | At <1k/s, dropped documents are acceptable in v1 |
| SMTP send failure | Log + continue (anomaly still indexed) | Alert delivery is best-effort; ES record is the source of truth |
| Kafka consumer error | Log + retry via PollFetches error handling | franz-go handles reconnect internally |
| Rule evaluation panic | `recover()` in engine loop, log stack trace, continue | Bugs in detection rules must not crash pipeline |
| Kafka partition rebalance | Drain in-flight work in `OnPartitionsRevoked` | Prevents duplicate processing after reassignment |

**At-least-once semantics:** Offsets are committed only after the Kafka record has been forwarded into the `rawMessages` channel. Combined with a graceful drain on shutdown, this minimizes (but does not eliminate) re-processing on restart. Idempotent document IDs (derived from Kafka offset + partition) prevent duplicate Elasticsearch documents.

---

## Package Structure

```
.
├── cmd/
│   └── server/
│       └── main.go             # Entrypoint, wiring
├── internal/
│   ├── config/                 # Config structs, Viper loading
│   ├── domain/                 # LogEntry, Anomaly, RawMessage types (no external deps)
│   ├── kafka/                  # MessageConsumer implementation (franz-go)
│   ├── pipeline/               # Parser pool, Processor, tee utilities
│   ├── detection/              # Detector interface + all rule implementations
│   ├── alert/                  # AlertChannel interface, Alerter orchestrator
│   ├── smtp/                   # SMTPAlerter (AlertChannel implementation)
│   ├── elasticsearch/          # LogStore, AnomalyStore implementations
│   └── api/                    # Gin router, handlers, query structs
└── pkg/
    └── (exported utilities if any)
```

**Key boundary:** `internal/domain` has zero external dependencies. All other packages depend inward on domain types, never outward on each other (except through wiring in `cmd/`).

---

## Sources

- Go Pipelines blog post (official): https://go.dev/blog/pipelines — HIGH confidence
- franz-go consumer API: https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo — HIGH confidence (verified against pkg.go.dev)
- go-elasticsearch v8 BulkIndexer: https://pkg.go.dev/github.com/elastic/go-elasticsearch/v8/esutil — HIGH confidence
- errgroup: https://pkg.go.dev/golang.org/x/sync/errgroup — HIGH confidence
- Gin framework: https://pkg.go.dev/github.com/gin-gonic/gin — HIGH confidence
- Viper configuration: https://pkg.go.dev/github.com/spf13/viper — HIGH confidence
- net/smtp standard library: https://pkg.go.dev/net/smtp — HIGH confidence (note: frozen, limited features)
