# Phase 1: Foundation and Ingestion - Research

**Researched:** 2026-03-21
**Domain:** Go module scaffold, franz-go Kafka consumer, log parser/normaliser, Zap structured logging, Prometheus metrics, testify/goleak unit tests
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INGEST-01 | System consumes log messages from a Kafka topic using a consumer group with at-least-once semantics | franz-go kgo.ConsumerGroup + kgo.PollFetches loop; AutoCommitMarks mode |
| INGEST-02 | Consumer commits offsets only after successful downstream processing | kgo.MarkCommitRecords after parse + channel forward; AutoCommitMarks commits only marked records |
| INGEST-03 | Consumer handles graceful shutdown — drains in-flight messages before closing on SIGTERM | signal.NotifyContext cancels root ctx; PollFetches returns; channel drain before Close() |
| INGEST-04 | Consumer supports configurable initial offset (oldest vs newest) | kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()) vs AtEnd(); config field selects at startup |
| PARSE-01 | Parser normalises structured JSON log entries into canonical LogEntry schema | encoding/json unmarshalling into LogEntry; uuid.NewString() for ID; time.Now() for ingest timestamp |
| PARSE-02 | Parser handles unstructured/plain-text log lines as fallback | First try JSON unmarshal; on error, treat full line as Message field with level=unknown |
| PARSE-03 | Log level variants normalised (err/ERR/FATAL -> ERROR; warn/WARNING -> WARN) | strings.ToUpper + switch/map normalisation function; pure function, easy to table-test |
| PARSE-04 | Parse errors logged and metered; pipeline continues | prometheus Counter increment + zap.Error log on parse failure; always send to output channel even if partially parsed |
| OBS-01 | Structured logging via Zap throughout the pipeline | zap.NewProduction() in main; pass *zap.Logger via constructor injection; kzap plugin for kafka client |
| OBS-02 | Prometheus metrics: logs consumed, processing latency, anomalies detected, ES write errors, alerts sent/failed, Kafka consumer lag | prometheus.NewCounterVec / NewHistogramVec / NewGaugeVec; MustRegister at init(); promhttp.Handler() on /metrics |
| TEST-01 | Unit tests for parser, detection rules, and alert deduplication logic (no external dependencies) | testify/assert + table-driven tests; go test -race; goleak.VerifyTestMain for goroutine leak detection |
</phase_requirements>

---

## Summary

Phase 1 establishes the project skeleton that all later phases build on. The three primary technical concerns are: (1) a correct franz-go Kafka consumer with at-least-once semantics and manual offset commits, (2) a pure-function log parser that handles both structured JSON and plain-text inputs with level normalisation, and (3) the observability wiring (Zap + Prometheus) that must be in place before any production metrics exist.

The project stack is fully decided and previously researched in `.planning/research/STACK.md` (verified March 2026). No library choices are open for this phase. The architecture is documented in `.planning/research/ARCHITECTURE.md` and directly shapes package layout, interface definitions, and the concurrency model used in this phase.

The key implementation risk is getting the franz-go offset commit semantics exactly right. Specifically, franz-go's `AutoCommitMarks` mode commits only records explicitly passed to `MarkCommitRecords` — this must happen after the record's bytes have been forwarded into the downstream channel, not before. Getting this wrong causes either silent data loss (commit before processing) or stalled consumers (never committing). The `OnPartitionsRevoked` callback must also flush committed offsets synchronously before returning to prevent duplicate processing after rebalance.

**Primary recommendation:** Wire franz-go with `AutoCommitMarks()` + `kgo.MarkCommitRecords(r)` after channel forward. Declare all domain types and interfaces in `internal/domain` with zero external imports. Use `go.uber.org/goleak` with `VerifyTestMain` — not `VerifyNone` per test — to avoid false failures from parallel test goroutines.

---

## Standard Stack

### Core (Phase 1 subset)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/twmb/franz-go/pkg/kgo` | v1.20.7 | Kafka consumer group | Pure Go, no CGo, fastest Go Kafka client, modern at-least-once API |
| `github.com/twmb/franz-go/plugin/kzap` | v1.20.7 | Zap bridge for kafka client logs | Ships as first-class plugin; zero glue code |
| `go.uber.org/zap` | v1.27.1 | Structured logging | Industry standard; JSON output; zero-alloc hot path; AtomicLevel |
| `github.com/prometheus/client_golang` | v1.23.2 | Metrics instrumentation | Industry standard; 71k+ importers; native promhttp handler |
| `github.com/google/uuid` | v1.6.0 | UUID generation for LogEntry.ID | Minimal, correct, widely used |
| `github.com/spf13/viper` | v1.21.0 | YAML config + env var binding | Precedence model (env > file > default); mapstructure Unmarshal |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | assert.Equal + require.NoError pattern; table-driven test helper |
| `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection in tests | VerifyTestMain catches leaks across all tests in a package |

### Phase 1 Does NOT Need (deferred)

| Library | Phase |
|---------|-------|
| `github.com/elastic/go-elasticsearch/v9` | Phase 3 |
| `github.com/go-chi/chi/v5` | Phase 4 |
| `github.com/wneessen/go-mail` | Phase 3 |
| `github.com/testcontainers/testcontainers-go` | Phase 5 |

### Installation

```bash
go mod init github.com/your-org/log-analytics

# Kafka
go get github.com/twmb/franz-go@v1.20.7
go get github.com/twmb/franz-go/plugin/kzap@v1.20.7

# Observability
go get go.uber.org/zap@v1.27.1
go get github.com/prometheus/client_golang@v1.23.2

# Utilities
go get github.com/google/uuid@v1.6.0
go get github.com/spf13/viper@v1.21.0

# Testing
go get github.com/stretchr/testify@v1.11.1
go get go.uber.org/goleak@v1.3.0
```

---

## Architecture Patterns

### Recommended Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go                  # Entrypoint: signal handling, wiring, errgroup
├── internal/
│   ├── domain/                      # LogEntry, RawMessage, Anomaly, all interfaces — zero external deps
│   ├── config/                      # Config structs + Viper loading function
│   ├── kafka/                       # KafkaConsumer (MessageConsumer implementation)
│   ├── pipeline/                    # Parser, worker pool, tee utilities, EnrichFunc chain
│   └── metrics/                     # Prometheus metric vars (package-level, registered at init)
└── go.mod
```

The `internal/domain` package is the architectural keystone of Phase 1. It must have no external dependencies — only stdlib types. Every other package in the project imports `domain`; `domain` imports nothing from the project.

### Pattern 1: Domain Types Declaration (Plan 01-01)

All core types and interfaces live in `internal/domain`. The planner must not scatter types across packages.

```go
// Source: ARCHITECTURE.md (internal research)
// internal/domain/types.go

type RawMessage struct {
    Payload   []byte
    Topic     string
    Partition int32
    Offset    int64
    Timestamp time.Time
}

type LogEntry struct {
    ID        string
    Timestamp time.Time
    Level     string         // normalised: "error", "warn", "info", "debug", "unknown"
    Service   string
    Message   string
    Fields    map[string]any
    RawSource string
    Source    RawMessage     // carried for offset commit tracking
}

type Anomaly struct {
    ID          string
    RuleID      string
    Severity    string
    Service     string
    Description string
    Evidence    []LogEntry
    DetectedAt  time.Time
}
```

Core interfaces (all in `internal/domain`):

```go
type MessageConsumer interface {
    Run(ctx context.Context) error
    Messages() <-chan RawMessage
}

type Parser interface {
    Parse(msg RawMessage) (LogEntry, error)
}

type Enricher interface {
    Enrich(entry LogEntry) LogEntry
}

type Detector interface {
    Name() string
    Evaluate(entry LogEntry) (Anomaly, bool)
    Reset()
}

type AlertChannel interface {
    Send(ctx context.Context, anomaly Anomaly) error
    Name() string
}

type LogStore interface {
    IndexLog(ctx context.Context, entry LogEntry) error
    SearchLogs(ctx context.Context, query LogQuery) ([]LogEntry, int64, error)
    GetLog(ctx context.Context, id string) (LogEntry, error)
}

type AnomalyStore interface {
    IndexAnomaly(ctx context.Context, anomaly Anomaly) error
    SearchAnomalies(ctx context.Context, query AnomalyQuery) ([]Anomaly, int64, error)
    GetAnomaly(ctx context.Context, id string) (Anomaly, error)
}
```

### Pattern 2: Franz-Go Consumer Setup (Plan 01-02)

At-least-once semantics with manual offset commit via `AutoCommitMarks`:

```go
// Source: pkg.go.dev/github.com/twmb/franz-go/pkg/kgo (verified Mar 2026)
// internal/kafka/consumer.go

client, err := kgo.NewClient(
    kgo.SeedBrokers(cfg.Brokers...),
    kgo.ConsumerGroup(cfg.GroupID),
    kgo.ConsumeTopics(cfg.Topic),
    kgo.ConsumeResetOffset(initialOffset), // kgo.NewOffset().AtStart() or AtEnd()
    kgo.AutoCommitMarks(),                 // Only commits records passed to MarkCommitRecords
    kgo.OnPartitionsRevoked(onRevoked),    // Drain + commit before rebalance
    kgo.WithLogger(kzap.New(logger)),
)

// Consume loop
func (c *Consumer) Run(ctx context.Context) error {
    for {
        fetches := c.client.PollFetches(ctx)
        if fetches.IsClientClosed() {
            return nil
        }
        fetches.EachError(func(t string, p int32, err error) {
            c.logger.Error("fetch error", zap.String("topic", t), zap.Int32("partition", p), zap.Error(err))
        })
        fetches.EachRecord(func(r *kgo.Record) {
            msg := domain.RawMessage{
                Payload:   r.Value,
                Topic:     r.Topic,
                Partition: r.Partition,
                Offset:    r.Offset,
                Timestamp: r.Timestamp,
            }
            select {
            case c.out <- msg:
                c.client.MarkCommitRecords(r) // Mark AFTER forwarding to channel
            case <-ctx.Done():
                return
            }
        })
    }
}

// OnPartitionsRevoked: flush committed offsets synchronously
onRevoked := func(ctx context.Context, cl *kgo.Client, partitions map[string][]int32) {
    if err := cl.CommitMarkedOffsets(ctx); err != nil {
        logger.Error("commit on revoke failed", zap.Error(err))
    }
}
```

**Critical:** `MarkCommitRecords(r)` must be called only after the record has been placed in the downstream channel. Calling it before means a crash between mark and channel-send silently loses the message.

### Pattern 3: Log Parser (Plan 01-03)

Pure function — no state, no dependencies except stdlib + domain types:

```go
// internal/pipeline/parser.go

// Parse attempts JSON first; falls back to plain-text on any error.
// Always returns a LogEntry — never nil. Parse errors are logged and metered by the caller.
func Parse(raw []byte) (domain.LogEntry, error) {
    var structured struct {
        Timestamp string         `json:"timestamp"`
        Level     string         `json:"level"`
        Service   string         `json:"service"`
        Message   string         `json:"message"`
        Fields    map[string]any `json:"fields"`
    }
    if err := json.Unmarshal(raw, &structured); err == nil {
        ts, _ := time.Parse(time.RFC3339, structured.Timestamp)
        return domain.LogEntry{
            ID:        uuid.NewString(),
            Timestamp: ts,
            Level:     NormaliseLevel(structured.Level),
            Service:   structured.Service,
            Message:   structured.Message,
            Fields:    structured.Fields,
            RawSource: string(raw),
        }, nil
    }
    // Plain-text fallback
    return domain.LogEntry{
        ID:        uuid.NewString(),
        Timestamp: time.Now(),
        Level:     "unknown",
        Message:   string(raw),
        RawSource: string(raw),
    }, fmt.Errorf("not valid JSON, stored as plain-text")
}

// NormaliseLevel maps vendor log level variants to the four canonical values.
func NormaliseLevel(raw string) string {
    switch strings.ToUpper(strings.TrimSpace(raw)) {
    case "ERROR", "ERR", "FATAL", "CRITICAL", "CRIT":
        return "error"
    case "WARN", "WARNING":
        return "warn"
    case "INFO", "INFORMATION":
        return "info"
    case "DEBUG", "TRACE", "VERBOSE":
        return "debug"
    default:
        return "unknown"
    }
}
```

### Pattern 4: Prometheus Metric Registration (Plan 01-04)

Register all metrics at package init so no call site can forget them:

```go
// Source: pkg.go.dev/github.com/prometheus/client_golang/prometheus (verified Mar 2026)
// internal/metrics/metrics.go

var (
    LogsConsumedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "logs_consumed_total", Help: "Total Kafka records consumed."},
        []string{"topic", "partition"},
    )
    LogsProcessedDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "logs_processed_duration_seconds",
        Help:    "End-to-end processing latency per log entry.",
        Buckets: prometheus.DefBuckets,
    })
    AnomaliesDetectedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "anomalies_detected_total", Help: "Anomalies detected by rule."},
        []string{"rule"},
    )
    ESWriteErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "elasticsearch_write_errors_total", Help: "ES bulk indexer failures.",
    })
    EmailAlertsSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "email_alerts_sent_total", Help: "Successful email alert deliveries.",
    })
    EmailAlertsFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "email_alerts_failed_total", Help: "Failed email alert deliveries.",
    })
    KafkaConsumerLag = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{Name: "kafka_consumer_lag", Help: "Kafka consumer group lag per partition."},
        []string{"topic", "partition"},
    )
)

func init() {
    prometheus.MustRegister(
        LogsConsumedTotal,
        LogsProcessedDuration,
        AnomaliesDetectedTotal,
        ESWriteErrorsTotal,
        EmailAlertsSentTotal,
        EmailAlertsFailedTotal,
        KafkaConsumerLag,
    )
}
```

Expose `/metrics` via:
```go
// cmd/server/main.go
mux.Handle("/metrics", promhttp.Handler())
```

### Pattern 5: Goroutine Leak Detection in Tests (Plan 01-05)

Use `VerifyTestMain` — not `VerifyNone` per test — to avoid false positives from parallel tests:

```go
// Source: pkg.go.dev/go.uber.org/goleak (verified Mar 2026)
// internal/pipeline/parser_test.go

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

`VerifyNone(t)` is incompatible with `t.Parallel()` and will produce false failures. `VerifyTestMain` checks after ALL tests in the package complete and is the recommended pattern.

### Pattern 6: Table-Driven Parser Tests (Plan 01-05)

```go
func TestNormaliseLevel(t *testing.T) {
    cases := []struct {
        input string
        want  string
    }{
        {"error", "error"},
        {"ERR", "error"},
        {"FATAL", "error"},
        {"warn", "warn"},
        {"WARNING", "warn"},
        {"info", "info"},
        {"DEBUG", "debug"},
        {"", "unknown"},
        {"CUSTOM_LEVEL", "unknown"},
    }
    for _, tc := range cases {
        t.Run(tc.input, func(t *testing.T) {
            assert.Equal(t, tc.want, NormaliseLevel(tc.input))
        })
    }
}
```

### Pattern 7: Errgroup Pipeline Lifecycle (wiring, Plan 01-02)

```go
// Source: pkg.go.dev/golang.org/x/sync/errgroup (stable API)
// cmd/server/main.go

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return consumer.Run(ctx) })
g.Go(func() error { return parserPool.Run(ctx) })
// ... other stages

if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
    logger.Fatal("pipeline error", zap.Error(err))
}
```

### Anti-Patterns to Avoid

- **Placing types in the package that first needs them** — `LogEntry` must be in `internal/domain`, not `internal/pipeline`. Putting types in the consuming package creates import cycles when later packages need the same type.
- **Committing offsets before channel send** — calling `MarkCommitRecords(r)` before `c.out <- msg` means a crash between those two lines silently drops the record.
- **Using `context.Background()` inside pipeline goroutines** — every function in the pipeline must accept and use the passed `ctx`. Hardcoding `context.Background()` blocks shutdown for the duration of the call's timeout.
- **Calling `prometheus.MustRegister` in test setup** — panics on the second test run in the same binary. Register once at `init()` in `internal/metrics/metrics.go`.
- **Starting the Kafka consumer before the downstream channel is ready** — causes the consumer to block on the first record send. Create channels, start all goroutines, then start the consumer.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Kafka at-least-once offset management | Custom offset tracking map | `kgo.AutoCommitMarks()` + `MarkCommitRecords` | franz-go handles rebalance races, epoch tracking, and commit retries correctly |
| Structured log level from franz-go client | fmt.Printf Kafka diagnostics | `plugin/kzap` with `kzap.New(logger)` | First-class plugin; logs Kafka events as structured zap fields |
| UUID generation | `fmt.Sprintf("%d-%d", ts, rand)` | `uuid.NewString()` | UUID v4 is cryptographically random; hand-rolled IDs have collision risk |
| Prometheus metric exposition | Custom `/metrics` handler | `promhttp.Handler()` | Handles content negotiation, gzip, OpenMetrics format; 1 line |
| Config file + env var precedence | Layered if-else config loading | Viper with `AutomaticEnv()` | Handles precedence (env > file > default), mapstructure unmarshal, and type coercion |
| Goroutine leak detection in tests | Manual goroutine count assertions | `go.uber.org/goleak` | Inspects goroutine stack traces; distinguishes known-good goroutines from leaks |

**Key insight:** The franz-go offset commit path has subtle correctness constraints around rebalance epochs and concurrent commits. The `AutoCommitMarks` mode was specifically designed for the "mark after processing" pattern — implementing this manually would require reimplementing epoch tracking.

---

## Common Pitfalls

### Pitfall 1: Offset Committed Before Downstream Confirmation (K1)
**What goes wrong:** Calling `MarkCommitRecords(r)` before the record's payload is forwarded to the parser channel. A crash between mark and channel-send causes that record to be permanently skipped on restart.
**Why it happens:** Developers call mark as the first thing in the handler loop, then do the channel send.
**How to avoid:** The select statement that sends to `c.out` and the `MarkCommitRecords(r)` call must be in the same branch — only reachable after the send succeeds.
**Warning signs:** Consumer lag stays at zero during failures; records vanish with no error log.

### Pitfall 2: OnPartitionsRevoked Does Not Flush Commits (K5)
**What goes wrong:** During a rebalance (or normal shutdown), `OnPartitionsRevoked` returns without committing the last batch of marked offsets. The next consumer starts from the previously committed position and re-processes already-processed records.
**Why it happens:** Forgetting to call `CommitMarkedOffsets` in the revoke callback.
**How to avoid:** Always call `cl.CommitMarkedOffsets(ctx)` in `OnPartitionsRevoked` before returning.
**Warning signs:** Duplicate log entries in Elasticsearch after consumer restart or scaling event.

### Pitfall 3: prometheus.MustRegister Panics on Double Registration (OBS)
**What goes wrong:** Prometheus metrics registered in a function called by multiple tests panic with "already registered".
**Why it happens:** Test files call the registration function in `TestMain` or `init`, and it runs twice.
**How to avoid:** Register all metrics exactly once in `init()` in `internal/metrics/metrics.go`. Test files import the package; they do not re-register.
**Warning signs:** `panic: duplicate metrics collector registration` on `go test ./...`.

### Pitfall 4: context.Background() Inside Pipeline Stages (G3)
**What goes wrong:** A goroutine in the pipeline makes an HTTP call (e.g., future ES call) using `context.Background()`. On SIGTERM, the root context is cancelled, but this goroutine blocks for the full HTTP timeout (30s+) before completing.
**Why it happens:** Copy-pasted code using `context.Background()` as a placeholder.
**How to avoid:** Every pipeline function signature is `func(ctx context.Context, ...)`. The passed `ctx` is always used for any blocking call.
**Warning signs:** Shutdown takes longer than 10 seconds; logs show goroutines still running after context cancel.

### Pitfall 5: goleak.VerifyNone with Parallel Tests (TEST-01)
**What goes wrong:** Using `defer goleak.VerifyNone(t)` in tests that call `t.Parallel()` causes false positives — goroutines from other parallel tests appear as leaks.
**Why it happens:** `VerifyNone` is designed for sequential tests only. The goleak docs document this limitation.
**How to avoid:** Use `goleak.VerifyTestMain(m)` in `TestMain`. This checks after all tests in the package have completed.
**Warning signs:** Intermittent goleak failures that vary by test execution order.

### Pitfall 6: Loop Variable Capture in Parser Worker Pool (G4)
**What goes wrong:** Launching goroutines in a loop with closures capturing the loop variable: all goroutines process the last iteration's record.
**Why it happens:** Classic Go closure trap. In Go < 1.22, loop variables are shared across iterations.
**How to avoid:** Check `go.mod` `go` directive version. If 1.22+, loop variable semantics fix this automatically. If < 1.22, shadow the variable: `msg := msg` before the goroutine launch.
**Warning signs:** Race detector fires; multiple goroutines report processing the same record.

---

## Code Examples

### Wiring kzap into franz-go

```go
// Source: pkg.go.dev/github.com/twmb/franz-go/plugin/kzap (verified Mar 2026)
import "github.com/twmb/franz-go/plugin/kzap"

logger, _ := zap.NewProduction()
client, err := kgo.NewClient(
    kgo.SeedBrokers("localhost:9092"),
    kgo.WithLogger(kzap.New(logger)),
)
```

### Configurable Initial Offset

```go
// Source: pkg.go.dev/github.com/twmb/franz-go/pkg/kgo (verified Mar 2026)
var initialOffset kgo.Offset
switch cfg.InitialOffset {
case "oldest":
    initialOffset = kgo.NewOffset().AtStart()
case "newest":
    initialOffset = kgo.NewOffset().AtEnd()
default:
    initialOffset = kgo.NewOffset().AtStart() // safe default for log analytics
}
kgo.ConsumeResetOffset(initialOffset)
```

`ConsumeResetOffset` applies only when no committed offset exists for the consumer group. Once offsets are committed, the group resumes from the committed position regardless of this setting.

### Zap Logger Bootstrap

```go
// Source: pkg.go.dev/go.uber.org/zap (verified Mar 2026)
logger, err := zap.NewProduction()
if err != nil {
    log.Fatalf("failed to initialise logger: %v", err)
}
defer logger.Sync() // flush buffer on shutdown

// Child logger with service context
pipelineLogger := logger.With(zap.String("component", "pipeline"))
```

### Viper Config Loading

```go
// Source: pkg.go.dev/github.com/spf13/viper (verified Mar 2026)
viper.SetConfigName("config")
viper.SetConfigType("yaml")
viper.AddConfigPath(".")
viper.AddConfigPath("/etc/log-analytics")
viper.AutomaticEnv()  // LOG_ANALYTICS_KAFKA_BROKERS overrides kafka.brokers

if err := viper.ReadInConfig(); err != nil {
    if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
        return fmt.Errorf("read config: %w", err)
    }
    // Config file not found is OK if all values come from env
}

var cfg Config
if err := viper.Unmarshal(&cfg); err != nil {
    return fmt.Errorf("unmarshal config: %w", err)
}
```

### Parse Error Metering (PARSE-04)

```go
// internal/pipeline/worker.go
entry, err := Parse(msg.Payload)
if err != nil {
    metrics.ParseErrorsTotal.Inc()
    logger.Warn("parse error — storing as plain-text",
        zap.String("topic", msg.Topic),
        zap.Int64("offset", msg.Offset),
        zap.Error(err),
    )
    // entry is still valid (plain-text fallback) — do NOT drop it
}
// Forward entry regardless of parse error
select {
case out <- entry:
case <-ctx.Done():
    return
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Sarama consumer group | franz-go kgo.Client | 2022-2024 | No CGo, 10-20x faster consuming, idiomatic context support |
| go-elasticsearch v8 | go-elasticsearch v9 with TypedClient | Feb 2026 (v9.3.1) | TypedClient compile-time schema checking; native OTel |
| logrus | go.uber.org/zap | 2019-2021 | Logrus in maintenance mode; zap zero-alloc structured logging |
| Global prometheus.DefaultRegisterer | Custom prometheus.NewRegistry() | Ongoing | Prefer custom registry to avoid global state in tests |

**Deprecated/outdated:**
- `github.com/IBM/sarama`: Still maintained (v1.47.0, Feb 2026) but franz-go is the modern replacement for new projects.
- `gopkg.in/gomail.v2`: Abandoned since 2016. Not relevant to Phase 1 but noting for later phases.
- `github.com/uber-go/goleak` vs `go.uber.org/goleak`: The canonical import path is `go.uber.org/goleak` v1.3.0, not the legacy `github.com/uber-go/goleak` path.

---

## Open Questions

1. **Go module path**
   - What we know: No `go.mod` exists yet. The project is at the repo root.
   - What's unclear: The module path (e.g., `github.com/your-org/log-analytics-and-anomaly-detection`).
   - Recommendation: Planner should use a placeholder like `github.com/log-analytics/server` and document that it must match the actual GitHub repo URL when pushed. This affects no Phase 1 tests.

2. **Go version directive in go.mod**
   - What we know: Go 1.22+ fixes loop variable capture (Pitfall 6). The current stable release is 1.24.x (March 2026).
   - What's unclear: Whether the deployment environment constrains the Go version.
   - Recommendation: Use `go 1.22` minimum in `go.mod` to eliminate the loop variable pitfall without requiring latest Go. This is conservative and broadly compatible.

3. **Kafka consumer lag metric implementation**
   - What we know: franz-go ships `plugin/kprom` for Prometheus metrics including consumer lag. The plan specifies `plugin/kzap` (for logging) but the `kafka_consumer_lag` gauge is normally provided by `plugin/kprom`.
   - What's unclear: Plan 01-04 lists `kafka_consumer_lag` as a manually registered metric, but `kprom` would provide it automatically.
   - Recommendation: Add `plugin/kprom` to Phase 1 dependencies alongside `plugin/kzap`. Wire `kprom.NewMetrics()` into the franz-go client. This provides `kafka_consumer_lag` without manual gauge updates and aligns with OBS-02.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `github.com/stretchr/testify` v1.11.1 |
| Config file | None required — stdlib testing |
| Quick run command | `go test -race ./internal/pipeline/...` |
| Full suite command | `go test -race ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PARSE-01 | JSON log parsed to correct LogEntry fields | unit | `go test -race ./internal/pipeline/... -run TestParse` | Wave 0 |
| PARSE-02 | Plain-text fallback — non-JSON bytes produce LogEntry with Message set | unit | `go test -race ./internal/pipeline/... -run TestParseFallback` | Wave 0 |
| PARSE-03 | All level variants normalise to canonical four values | unit | `go test -race ./internal/pipeline/... -run TestNormaliseLevel` | Wave 0 |
| PARSE-04 | Parse error increments metric, does not drop entry | unit | `go test -race ./internal/pipeline/... -run TestParseErrorMetering` | Wave 0 |
| INGEST-01 | Consumer group ID set correctly (config test) | unit | `go test -race ./internal/kafka/... -run TestConsumerConfig` | Wave 0 |
| INGEST-02 | MarkCommitRecords called after channel send, not before | unit | `go test -race ./internal/kafka/... -run TestOffsetCommitOrder` | Wave 0 |
| INGEST-03 | Graceful shutdown: PollFetches returns on ctx cancel | unit | `go test -race ./internal/kafka/... -run TestShutdown` | Wave 0 |
| INGEST-04 | Initial offset config maps to AtStart/AtEnd correctly | unit | `go test -race ./internal/kafka/... -run TestInitialOffset` | Wave 0 |
| OBS-01 | Zap logger flows through pipeline without fmt.Printf | unit (code review via grep) | `go vet ./...` + manual | Wave 0 |
| OBS-02 | All six metric names present in /metrics output | unit (registry check) | `go test -race ./internal/metrics/...` | Wave 0 |
| TEST-01 | No goroutine leaks after test completion | unit | `go test -race ./...` (goleak via TestMain) | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -race ./internal/pipeline/...` (parser tests only, < 5s)
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/pipeline/parser_test.go` — covers PARSE-01, PARSE-02, PARSE-03, PARSE-04
- [ ] `internal/kafka/consumer_test.go` — covers INGEST-01, INGEST-02, INGEST-03, INGEST-04
- [ ] `internal/metrics/metrics_test.go` — covers OBS-02
- [ ] `TestMain` with `goleak.VerifyTestMain(m)` in each test package — covers TEST-01

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/twmb/franz-go/pkg/kgo` — consumer group API, PollFetches, MarkCommitRecords, offset options, partition revoke callback (verified Mar 2026)
- `pkg.go.dev/github.com/twmb/franz-go/plugin/kzap` — kzap.New API, WithLogger wiring (verified Mar 2026)
- `pkg.go.dev/go.uber.org/zap` — NewProduction, NewDevelopment, field types (verified Mar 2026, v1.27.1)
- `pkg.go.dev/github.com/prometheus/client_golang/prometheus` — Counter, Gauge, Histogram, MustRegister (verified Mar 2026, v1.23.2)
- `pkg.go.dev/go.uber.org/goleak` — VerifyTestMain, VerifyNone, parallel test limitation (verified Mar 2026, v1.3.0)
- `pkg.go.dev/github.com/spf13/viper` — SetConfigName, ReadInConfig, AutomaticEnv, Unmarshal (verified Mar 2026, v1.21.0)
- `pkg.go.dev/github.com/google/uuid` — NewString, NewRandom (verified Mar 2026, v1.6.0)
- `.planning/research/STACK.md` — full stack decision record (project research, verified Mar 2026)
- `.planning/research/PITFALLS.md` — K1/K5/G1/G3/G4 pitfalls directly applicable to Phase 1 (project research, verified Mar 2026)
- `.planning/research/ARCHITECTURE.md` — package structure, interface definitions, concurrency model, errgroup wiring (project research, verified Mar 2026)

### Secondary (MEDIUM confidence)
- `.planning/research/FEATURES.md` — log level normalisation variants, parser fallback behaviour (project research derived from ELK/Splunk/Loki patterns)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all library versions verified against pkg.go.dev as of March 2026
- Architecture: HIGH — domain type definitions and package structure drawn directly from pre-existing ARCHITECTURE.md research
- Franz-go API: HIGH — PollFetches, MarkCommitRecords, AutoCommitMarks, OnPartitionsRevoked all verified against official pkg.go.dev documentation
- Parser patterns: HIGH — pure stdlib; encoding/json behaviour is stable and well-documented
- Pitfalls: HIGH — K1/K5 verified against franz-go docs; G3/G4 verified against go.dev docs
- Validation architecture: HIGH — testing packages verified; goleak parallel-test limitation explicitly documented in official goleak docs

**Research date:** 2026-03-21
**Valid until:** 2026-04-21 (stable libraries; re-verify if franz-go or prometheus client has a major version bump)
