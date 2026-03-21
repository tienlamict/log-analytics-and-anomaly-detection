# Phase 3: Storage and Alerting - Research

**Researched:** 2026-03-21
**Domain:** Elasticsearch Go client v9 (esutil.BulkIndexer, TypedAPI), go-mail v0.7.2 (SMTP), async fan-out wiring
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STORE-01 | Raw log entries indexed to `logs-{YYYY.MM.DD}` via bulk indexer | esutil.BulkIndexer with per-entry dynamic index name; BulkIndexerItem.Index field |
| STORE-02 | Detected anomalies indexed to `anomalies` index | Second BulkIndexer instance; fixed index name in config |
| STORE-03 | ES index mappings applied at startup via TypedAPI | TypedClient.Indices().PutIndexTemplate() called before consumer starts |
| STORE-04 | Bulk write failures logged and metered; pipeline continues | OnFailure callback per item increments metrics.ESWriteErrorsTotal; no propagated error |
| ALERT-01 | Email sent via SMTP when anomaly detected (post cooldown) | go-mail DialAndSendWithContext; cooldown already handled in DetectorEngine.Anomalies() channel |
| ALERT-02 | Email includes anomaly type, service, detection time, threshold, severity, sample logs | Msg.SetBodyString with formatted text; Evidence []LogEntry already on Anomaly struct |
| ALERT-03 | SMTP config (host, port, credentials, recipient) via config/env | New SMTPConfig block in Config struct; Viper env binding |
</phase_requirements>

---

## Summary

Phase 3 wires the two output legs of the detection engine: Elasticsearch persistence and SMTP alerting. All domain contracts are already defined — `LogStore` and `AnomalyStore` interfaces exist in `internal/domain/interfaces.go`, metrics counters `ESWriteErrorsTotal`, `EmailAlertsSentTotal`, and `EmailAlertsFailedTotal` are already registered in `internal/metrics/metrics.go`. The phase implements these interfaces and routes traffic through them.

The ES client decision was locked in Phase 1: `github.com/elastic/go-elasticsearch/v9` at the TypedClient level. The current latest release is **v9.3.1** (available in the Go module proxy). The `esutil.BulkIndexer` lives in the same module and accepts either `*elasticsearch.Client` or `*elasticsearch.TypedClient` as `BulkIndexerConfig.Client`, because both embed `BaseClient` which implements `esapi.Transport`.

The go-mail decision was locked in Phase 1: `github.com/wneessen/go-mail v0.7.2` (security patch for GO-2025-3988). This is confirmed as the latest release at research time. The async dispatch pattern is a buffered `chan domain.Anomaly` drained by a single goroutine, keeping SMTP latency fully out of the detection hot path.

Index mapping strategy: use index templates (`PUT _index_template/logs-template` and `PUT _index_template/anomalies-template`) applied at startup via `TypedClient.Indices().PutIndexTemplate()`. Templates use `dynamic: false` (via `dynamicmapping.False`) to prevent mapping explosion from unbounded `fields` keys. The `fields` map on `LogEntry` is mapped as `flattened` type — a single ES field type that stores the whole JSON object and exposes all leaf values for filtering without per-key sub-mappings.

**Primary recommendation:** Implement `internal/elasticsearch` (LogIndexer + AnomalyIndexer via two BulkIndexer instances) and `internal/smtp` (SMTPAlerter using go-mail) as separate packages behind the existing `LogStore` / `AnomalyStore` / `AlertChannel` domain interfaces. Wire in `cmd/server/main.go` using errgroup.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/elastic/go-elasticsearch/v9` | v9.3.1 | ES HTTP client, TypedAPI, index templates | Locked decision (STATE.md); TypedClient provides compile-time-checked schemas |
| `github.com/wneessen/go-mail` | v0.7.2 | SMTP email composition and dispatch | Locked decision; patched for GO-2025-3988; context-aware DialAndSendWithContext |
| `golang.org/x/sync` | already in module graph via upstream | errgroup for goroutine lifecycle management | Used in architecture patterns; already needed for main.go wiring |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `crypto/sha256` (stdlib) | stdlib | Deterministic document IDs for log entries | sha256(fmt.Sprintf("%d:%d", partition, offset)) prevents duplicate indexing on replay |
| `encoding/hex` (stdlib) | stdlib | Hex-encode sha256 digest for document ID string | Pair with sha256 |
| `fmt` / `strings` / `time` (stdlib) | stdlib | Index name generation (`logs-2026.03.21`), email body formatting | Always available |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `dynamicmapping.False` (typed enum) | Raw JSON body via `Raw(io.Reader)` | Raw JSON bypasses type safety; TypedAPI is the locked choice |
| `FlattenedProperty` for fields | `ObjectProperty` with nested keys | ObjectProperty causes mapping explosion with arbitrary field names |
| Single BulkIndexer with per-item index | Two separate BulkIndexers | Per-item index works but means both logs and anomalies share flush/worker state; separate instances are cleaner |

**Installation:**
```bash
go get github.com/elastic/go-elasticsearch/v9@v9.3.1
go get github.com/wneessen/go-mail@v0.7.2
```

**Version verification:** Both verified against the Go module proxy on 2026-03-21.
- `go-elasticsearch/v9`: latest = v9.3.1 (module proxy confirmed)
- `go-mail`: latest = v0.7.2 (module proxy confirmed, matches locked decision)

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── elasticsearch/
│   ├── client.go          # NewClient(cfg ESConfig) (*elasticsearch.TypedClient, error)
│   ├── setup.go           # ApplyIndexTemplates(ctx, client) error
│   ├── log_indexer.go     # LogIndexer struct; implements domain.LogStore (IndexLog only)
│   └── anomaly_indexer.go # AnomalyIndexer struct; implements domain.AnomalyStore (IndexAnomaly only)
├── smtp/
│   └── alerter.go         # SMTPAlerter struct; implements domain.AlertChannel
├── alert/
│   └── dispatcher.go      # Dispatcher: consumes chan Anomaly, fans out to AlertChannel + AnomalyStore
├── config/
│   └── config.go          # Add ESConfig, SMTPConfig to existing Config struct
cmd/server/
└── main.go                # Wire all components; start BulkIndexer flush goroutines; errgroup
```

Note: `SearchLogs`, `GetLog`, `SearchAnomalies`, `GetAnomaly` are part of `LogStore`/`AnomalyStore` interfaces but are implemented in Phase 4 (REST API). In Phase 3, only `IndexLog` and `IndexAnomaly` need real implementations; the search/get methods can return `errors.New("not implemented")` stubs.

### Pattern 1: TypedClient Construction

**What:** Create the ES typed client once at startup, inject everywhere.
**When to use:** Single process; share one connection pool.

```go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/elasticsearch.go
import (
    "net/http"
    elasticsearch "github.com/elastic/go-elasticsearch/v9"
)

func NewClient(cfg ESConfig) (*elasticsearch.TypedClient, error) {
    return elasticsearch.NewTypedClient(elasticsearch.Config{
        Addresses: cfg.Addresses,
        Username:  cfg.Username,
        Password:  cfg.Password,
        Transport: &http.Transport{
            MaxIdleConnsPerHost:   cfg.MaxIdleConns,    // default 10
            ResponseHeaderTimeout: cfg.ResponseTimeout, // default 10s
        },
        RetryOnStatus: []int{502, 503, 504, 429},
        MaxRetries:    3,
    })
}
```

### Pattern 2: Index Template Application at Startup

**What:** PUT index template before first document write; idempotent (upsert semantics).
**When to use:** Every startup; fail-fast if ES unreachable.

```go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/typedapi/indices/putindextemplate/request.go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/typedapi/types/typemapping.go
import (
    "github.com/elastic/go-elasticsearch/v9/typedapi/indices/putindextemplate"
    "github.com/elastic/go-elasticsearch/v9/typedapi/types"
    "github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/dynamicmapping"
)

func applyLogsTemplate(ctx context.Context, client *elasticsearch.TypedClient) error {
    falseMapping := dynamicmapping.False
    trueVal := true
    priority := int64(100)

    req := &putindextemplate.Request{
        IndexPatterns: []string{"logs-*"},
        Priority:      &priority,
        Template: &types.IndexTemplateMapping{
            Mappings: &types.TypeMapping{
                Dynamic: &falseMapping,
                Properties: map[string]types.Property{
                    "@timestamp": types.DateProperty{},
                    "level":      types.KeywordProperty{},
                    "service":    types.KeywordProperty{},
                    "message":    types.TextProperty{},
                    "fields":     types.FlattenedProperty{},  // prevents mapping explosion
                    "raw_source": types.KeywordProperty{Index: &trueVal},
                },
            },
        },
    }

    resp, err := client.Indices().PutIndexTemplate("logs-template").Request(req).Do(ctx)
    if err != nil {
        return fmt.Errorf("apply logs template: %w", err)
    }
    _ = resp
    return nil
}
```

**CRITICAL:** Call `ApplyIndexTemplates` before starting the Kafka consumer. If it fails, return the error so main exits — do not swallow it.

### Pattern 3: BulkIndexer Construction and Use

**What:** One BulkIndexer per logical index target; add items concurrently; close on shutdown.
**When to use:** Log indexing (per-entry dynamic index name) and anomaly indexing (fixed index).

```go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/esutil/bulk_indexer.go
import (
    "github.com/elastic/go-elasticsearch/v9/esutil"
)

// TypedClient implements esapi.Transport via embedded BaseClient.Perform()
indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
    Client:        esClient,        // *elasticsearch.TypedClient satisfies esapi.Transport
    Index:         "",              // leave blank; set per-item via BulkIndexerItem.Index
    NumWorkers:    2,
    FlushBytes:    5_000_000,       // 5 MB
    FlushInterval: 5 * time.Second,
    OnError: func(ctx context.Context, err error) {
        logger.Error("bulk indexer error", zap.Error(err))
        metrics.ESWriteErrorsTotal.Inc()
    },
})

// Per-entry add (log indexing):
indexName := "logs-" + entry.Timestamp.UTC().Format("2006.01.02")
docID := deterministicLogID(entry.Source.Partition, entry.Source.Offset)
body, _ := json.Marshal(entry)

err = indexer.Add(ctx, esutil.BulkIndexerItem{
    Action:     "index",
    Index:      indexName,   // overrides BulkIndexerConfig.Index
    DocumentID: docID,
    Body:       bytes.NewReader(body),
    OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
        if err != nil {
            logger.Error("es index failure", zap.Error(err))
        } else {
            logger.Error("es index failure", zap.String("type", res.Error.Type), zap.String("reason", res.Error.Reason))
        }
        metrics.ESWriteErrorsTotal.Inc()
    },
})

// Deterministic ID for idempotent reprocessing:
func deterministicLogID(partition int32, offset int64) string {
    h := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", partition, offset)))
    return hex.EncodeToString(h[:])
}
```

**Shutdown:** Call `indexer.Close(ctx)` with a deadline (e.g., 10s) to flush remaining items before process exit.

### Pattern 4: go-mail Client and Message

**What:** Create one `*mail.Client` at startup; call `DialAndSendWithContext` per alert.
**When to use:** SMTP alerting from async goroutine.

```go
// Source: github.com/wneessen/go-mail@v0.7.2/client.go, msg.go, encoding.go
import (
    mail "github.com/wneessen/go-mail"
)

func NewSMTPClient(cfg SMTPConfig) (*mail.Client, error) {
    return mail.NewClient(cfg.Host,
        mail.WithPort(cfg.Port),
        mail.WithSMTPAuth(mail.SMTPAuthPlain),
        mail.WithUsername(cfg.Username),
        mail.WithPassword(cfg.Password),
        mail.WithTLSPortPolicy(mail.TLSMandatory),  // or TLSOpportunistic / NoTLS
        mail.WithTimeout(30*time.Second),
    )
}

func buildAlertMessage(anomaly domain.Anomaly, cfg SMTPConfig) (*mail.Msg, error) {
    m := mail.NewMsg()
    if err := m.From(cfg.From); err != nil { return nil, err }
    if err := m.To(cfg.Recipients...); err != nil { return nil, err }
    m.Subject(fmt.Sprintf("[%s] Anomaly: %s on %s", strings.ToUpper(anomaly.Severity), anomaly.RuleID, anomaly.Service))

    body := formatAlertBody(anomaly) // builds plain-text with all ALERT-02 fields
    m.SetBodyString(mail.TypeTextPlain, body)
    return m, nil
}

// Async dispatch pattern:
go func() {
    for anomaly := range anomalyCh {
        msg, err := buildAlertMessage(anomaly, cfg)
        if err != nil {
            logger.Error("build alert message", zap.Error(err))
            metrics.EmailAlertsFailedTotal.Inc()
            continue
        }
        if err := client.DialAndSendWithContext(ctx, msg); err != nil {
            logger.Error("smtp send failed", zap.Error(err))
            metrics.EmailAlertsFailedTotal.Inc()
            continue
        }
        metrics.EmailAlertsSentTotal.Inc()
    }
}()
```

### Pattern 5: Fan-Out Dispatcher Wiring

**What:** Read from `DetectorEngine.Anomalies()` channel; forward to both SMTP alerter and anomaly indexer.
**When to use:** `internal/alert/dispatcher.go` — the Alerter orchestrator from plan 03-04.

```go
// Cooldown is already handled INSIDE DetectorEngine.Evaluate() / silenceWatcher.
// The Anomalies() channel only emits post-cooldown anomalies.
// The dispatcher does NOT need its own cooldown check.

func (d *Dispatcher) Run(ctx context.Context) error {
    for {
        select {
        case anomaly, ok := <-d.anomalyCh:
            if !ok {
                return nil
            }
            // Both operations happen regardless of the other's outcome.
            if err := d.anomalyStore.IndexAnomaly(ctx, anomaly); err != nil {
                d.logger.Error("index anomaly failed", zap.Error(err))
                // do not return — continue dispatching
            }
            if err := d.alertChannel.Send(ctx, anomaly); err != nil {
                d.logger.Error("alert send failed", zap.Error(err))
                // do not return — continue dispatching
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

**Important design note:** `domain.AlertChannel.Send` is the synchronous interface. The async buffered channel pattern is internal to `SMTPAlerter.Send` — the method enqueues into the internal goroutine's channel and returns immediately. This keeps `Dispatcher` simple and decoupled from transport details.

### Pattern 6: Full Fan-Out Wiring in main.go

```go
// Tee pattern: single LogEntry source to two consumers
// Source: Architecture research (ARCHITECTURE.md, .planning/research/)

g, ctx := errgroup.WithContext(rootCtx)

// --- Startup sequence (before consumer) ---
if err := elasticsearch.ApplyIndexTemplates(ctx, esClient); err != nil {
    return fmt.Errorf("startup: %w", err)
}

// --- Goroutines ---
g.Go(func() error { return consumer.Run(ctx) })
g.Go(func() error { return processorLoop(ctx, consumer.Messages(), logIndexer, detector) })
g.Go(func() error { return dispatcher.Run(ctx) })
// BulkIndexer workers are internal to esutil; no separate goroutine needed here.

return g.Wait()
```

### Anti-Patterns to Avoid

- **Checking only HTTP status from BulkIndexer:** The HTTP status is 200 even when individual document operations fail. Parse per-item `OnFailure` callbacks — that is where real write errors appear.
- **Using `dynamic: true` (default) on logs-* template:** Log entries have an unbounded `Fields map[string]any`. Without `dynamic: false`, each unique key creates a new mapping entry, exhausting the cluster's `index.mapping.total_fields.limit` (default 1000).
- **Calling ApplyIndexTemplates after consumer starts:** A brief window exists where log documents arrive before the template is applied. Documents without a matching template use the default dynamic mapping.
- **Sending SMTP in the detection hot path:** A slow or unavailable SMTP server would back-pressure the entire pipeline. SMTP dispatch must be asynchronous.
- **Calling `indexer.Close` without a deadline context:** `Close` waits for all pending flushes. Without a timeout it can block shutdown indefinitely.
- **Using `WithTLSPolicy` instead of `WithTLSPortPolicy`:** `WithTLSPolicy` does not auto-adjust the port. `WithTLSPortPolicy(mail.TLSMandatory)` correctly sets port 587 for STARTTLS. Use `WithTLSPolicy(mail.TLSMandatory)` only if `WithPort` is called explicitly.
- **Re-implementing cooldown in the Dispatcher:** `DetectorEngine.Anomalies()` already emits only post-cooldown anomalies. A second cooldown check would introduce a race and double-suppress valid alerts.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parallel bulk document submission | Custom batch queue | `esutil.BulkIndexer` | Handles multi-worker goroutines, flush-by-bytes, flush-by-time, retry, per-item callbacks, stats |
| Bulk failure detection | Check HTTP 200 → assume success | `BulkIndexerItem.OnFailure` callback | ES returns 200 with `errors: true`; per-item failure requires response body parsing |
| Email composition and TLS negotiation | `net/smtp` directly | `go-mail` | go-mail handles EHLO/HELO negotiation, STARTTLS, auth mechanism selection, MIME encoding, header encoding |
| Dynamic mapping prevention | Custom ingest pipeline | `dynamic: false` + `flattened` type | Ingest pipeline adds complexity; `flattened` type is the ES-native solution |
| Deterministic document IDs | UUID per document | `sha256(partition:offset)` | UUID prevents idempotent reprocessing on Kafka offset replay |

**Key insight:** `esutil.BulkIndexer` is the correct abstraction. It handles all the subtle bulk API requirements (response parsing, worker coordination, flushing) that are easy to get wrong in a custom implementation.

---

## Common Pitfalls

### Pitfall 1: Bulk Response Body Not Parsed (Pitfall E1)
**What goes wrong:** Code checks HTTP status code only; ES returns 200 with `"errors": true` for document-level failures.
**Why it happens:** The Bulk API is always 200 unless the request itself is malformed.
**How to avoid:** Use `BulkIndexerItem.OnFailure` callback — it fires per document when `res.Error.Type != ""`.
**Warning signs:** `ESWriteErrorsTotal` never increments but documents are missing from ES.

### Pitfall 2: Transport Without Bounded Connection Pool (Pitfall E5)
**What goes wrong:** Default `http.DefaultTransport` has `MaxIdleConnsPerHost: 2`; under load, new connections are created for every request, exhausting file descriptors.
**Why it happens:** ES client uses `http.DefaultTransport` if none is provided.
**How to avoid:** Always pass an explicit `http.Transport` with `MaxIdleConnsPerHost: 10` and `ResponseHeaderTimeout: 10s` in `elasticsearch.Config.Transport`.
**Warning signs:** "connection reset by peer" errors under load; high open-file-descriptor count.

### Pitfall 3: Index Template Not Applied Before First Document (Pitfall E6)
**What goes wrong:** If the consumer starts before `ApplyIndexTemplates`, the first batch of documents creates the index with default dynamic mapping. The template has no effect on existing indices.
**Why it happens:** Startup ordering is wrong.
**How to avoid:** Apply templates synchronously in startup before `g.Go(func() error { return consumer.Run(ctx) })`. Return early on template application error.
**Warning signs:** ES mapping shows `"dynamic": "true"` or unexpected field mappings after first run.

### Pitfall 4: SMTP Latency in Detection Hot Path (Pitfall O4)
**What goes wrong:** Synchronous `DialAndSendWithContext` in the anomaly fan-out goroutine stalls the whole pipeline if the SMTP server is slow.
**Why it happens:** SMTP dial + TLS + auth + data transfer is 100-500ms under normal conditions; worse if the server is unreachable (timeout).
**How to avoid:** `SMTPAlerter.Send` must enqueue into an internal buffered channel and return immediately. A separate goroutine drains the channel and calls `DialAndSendWithContext`.
**Warning signs:** Anomaly processing rate drops when SMTP server is slow; `ctx.Done()` trips before email is sent.

### Pitfall 5: Mapping Explosion from `fields` Map (Pitfall E2 / O5)
**What goes wrong:** `LogEntry.Fields map[string]any` indexed with `dynamic: true` creates a new ES field mapping for every unique key. At 1000 unique keys the index hits `total_fields.limit` and rejects further documents.
**Why it happens:** Default ES behaviour; no explicit mapping constraint.
**How to avoid:** Map `fields` as `FlattenedProperty` in the index template. This stores the entire JSON object as a single field, still queryable by leaf values, but with only one mapping entry.
**Warning signs:** ES error `"mapper_parsing_exception"` with `"reason": "The limit of total fields ... has been reached"`.

### Pitfall 6: go-mail Default TLS Policy is TLSMandatory on port 25
**What goes wrong:** `mail.NewClient(host)` uses `DefaultTLSPolicy = TLSMandatory` and port 25. Port 25 usually does not support STARTTLS; handshake fails.
**Why it happens:** Default policy + default port mismatch for modern SMTP submissions.
**How to avoid:** Use `WithTLSPortPolicy(mail.TLSMandatory)` which sets port 587 for STARTTLS automatically. Or use `WithPort(587)` + `WithTLSPolicy(mail.TLSMandatory)` explicitly.
**Warning signs:** "tls: first record does not look like a TLS handshake" in SMTP errors.

### Pitfall 7: Closing BulkIndexer Without Deadline
**What goes wrong:** `indexer.Close(context.Background())` waits forever for inflight HTTP calls if ES is unreachable at shutdown.
**Why it happens:** `Close` flushes remaining items; no timeout on the context.
**How to avoid:** Use `context.WithTimeout(ctx, 10*time.Second)` when calling `indexer.Close`.
**Warning signs:** Process does not exit within expected shutdown window; goroutine leak reported by goleak.

---

## Code Examples

### Full BulkIndexer Setup (verified from source)

```go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/esutil/bulk_indexer.go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/esutil/bulk_indexer_example_test.go

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "time"

    elasticsearch "github.com/elastic/go-elasticsearch/v9"
    "github.com/elastic/go-elasticsearch/v9/esutil"
    "go.uber.org/zap"

    "github.com/log-analytics/server/internal/domain"
    "github.com/log-analytics/server/internal/metrics"
)

type LogIndexer struct {
    indexer esutil.BulkIndexer
    logger  *zap.Logger
}

func NewLogIndexer(client *elasticsearch.TypedClient, logger *zap.Logger) (*LogIndexer, error) {
    indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
        Client:        client,
        NumWorkers:    2,
        FlushBytes:    5_000_000,
        FlushInterval: 5 * time.Second,
        OnError: func(_ context.Context, err error) {
            logger.Error("bulk indexer error", zap.Error(err))
            metrics.ESWriteErrorsTotal.Inc()
        },
    })
    if err != nil {
        return nil, fmt.Errorf("new log indexer: %w", err)
    }
    return &LogIndexer{indexer: indexer, logger: logger}, nil
}

func (li *LogIndexer) IndexLog(ctx context.Context, entry domain.LogEntry) error {
    indexName := "logs-" + entry.Timestamp.UTC().Format("2006.01.02")
    docID := sha256HexID(fmt.Sprintf("%d:%d", entry.Source.Partition, entry.Source.Offset))
    body, err := json.Marshal(entry)
    if err != nil {
        return fmt.Errorf("marshal log entry: %w", err)
    }
    return li.indexer.Add(ctx, esutil.BulkIndexerItem{
        Action:     "index",
        Index:      indexName,
        DocumentID: docID,
        Body:       bytes.NewReader(body),
        OnFailure: func(_ context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
            if err != nil {
                li.logger.Error("log index item failure", zap.Error(err))
            } else {
                li.logger.Error("log index item failure",
                    zap.String("type", res.Error.Type),
                    zap.String("reason", res.Error.Reason))
            }
            metrics.ESWriteErrorsTotal.Inc()
        },
    })
}

func (li *LogIndexer) Close(ctx context.Context) error {
    return li.indexer.Close(ctx)
}

func sha256HexID(input string) string {
    h := sha256.Sum256([]byte(input))
    return hex.EncodeToString(h[:])
}
```

### Full go-mail Alert Message (verified from source)

```go
// Source: github.com/wneessen/go-mail@v0.7.2/client.go, msg.go, encoding.go, auth.go

import (
    "context"
    "fmt"
    "strings"
    "time"

    mail "github.com/wneessen/go-mail"
    "go.uber.org/zap"

    "github.com/log-analytics/server/internal/domain"
    "github.com/log-analytics/server/internal/metrics"
)

type SMTPAlerter struct {
    client     *mail.Client
    cfg        SMTPConfig
    logger     *zap.Logger
    queue      chan domain.Anomaly
    done       chan struct{}
}

func (a *SMTPAlerter) Name() string { return "smtp" }

func (a *SMTPAlerter) Send(_ context.Context, anomaly domain.Anomaly) error {
    select {
    case a.queue <- anomaly:
        return nil
    default:
        a.logger.Warn("smtp alert queue full, dropping anomaly",
            zap.String("rule_id", anomaly.RuleID))
        metrics.EmailAlertsFailedTotal.Inc()
        return nil // non-fatal: alerting must not block the pipeline
    }
}

func (a *SMTPAlerter) Run(ctx context.Context) {
    for {
        select {
        case anomaly := <-a.queue:
            a.dispatch(ctx, anomaly)
        case <-ctx.Done():
            // drain remaining
            for {
                select {
                case anomaly := <-a.queue:
                    a.dispatch(ctx, anomaly)
                default:
                    return
                }
            }
        }
    }
}

func (a *SMTPAlerter) dispatch(ctx context.Context, anomaly domain.Anomaly) {
    m := mail.NewMsg()
    _ = m.From(a.cfg.From)
    _ = m.To(a.cfg.Recipients...)
    m.Subject(fmt.Sprintf("[%s] %s — %s", strings.ToUpper(anomaly.Severity), anomaly.RuleID, anomaly.Service))
    m.SetBodyString(mail.TypeTextPlain, formatAlertBody(anomaly))

    if err := a.client.DialAndSendWithContext(ctx, m); err != nil {
        a.logger.Error("smtp alert failed", zap.Error(err), zap.String("rule_id", anomaly.RuleID))
        metrics.EmailAlertsFailedTotal.Inc()
        return
    }
    metrics.EmailAlertsSentTotal.Inc()
}

func formatAlertBody(a domain.Anomaly) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("Anomaly Detected\n\n"))
    sb.WriteString(fmt.Sprintf("Type:         %s\n", a.RuleID))
    sb.WriteString(fmt.Sprintf("Service:      %s\n", a.Service))
    sb.WriteString(fmt.Sprintf("Severity:     %s\n", a.Severity))
    sb.WriteString(fmt.Sprintf("Detected At:  %s\n", a.DetectedAt.UTC().Format(time.RFC3339)))
    sb.WriteString(fmt.Sprintf("Description:  %s\n", a.Description))
    if len(a.Evidence) > 0 {
        sb.WriteString("\nSample Log Lines:\n")
        for i, entry := range a.Evidence {
            if i >= 5 { break } // cap at 5 sample lines per ALERT-02
            sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", entry.Timestamp.UTC().Format(time.RFC3339), entry.Level, entry.Message))
        }
    }
    return sb.String()
}
```

### Index Template for anomalies (verified types)

```go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/typedapi/indices/putindextemplate/request.go
// Source: github.com/elastic/go-elasticsearch/v9@v9.3.1/typedapi/types/typemapping.go

func applyAnomaliesTemplate(ctx context.Context, client *elasticsearch.TypedClient) error {
    falseMapping := dynamicmapping.False
    priority := int64(100)

    req := &putindextemplate.Request{
        IndexPatterns: []string{"anomalies"},
        Priority:      &priority,
        Template: &types.IndexTemplateMapping{
            Mappings: &types.TypeMapping{
                Dynamic: &falseMapping,
                Properties: map[string]types.Property{
                    "id":          types.KeywordProperty{},
                    "rule_id":     types.KeywordProperty{},
                    "severity":    types.KeywordProperty{},
                    "service":     types.KeywordProperty{},
                    "description": types.TextProperty{},
                    "detected_at": types.DateProperty{},
                    // Evidence []LogEntry stored as flattened nested object
                    "evidence": types.FlattenedProperty{},
                },
            },
        },
    }

    _, err := client.Indices().PutIndexTemplate("anomalies-template").Request(req).Do(ctx)
    return err
}
```

---

## Config Additions

The existing `Config` struct in `internal/config/config.go` needs two new sections.

### ESConfig

```go
type ESConfig struct {
    Addresses       []string      `mapstructure:"addresses"`
    Username        string        `mapstructure:"username"`
    Password        string        `mapstructure:"password"`
    MaxIdleConns    int           `mapstructure:"max_idle_conns"`
    ResponseTimeout time.Duration `mapstructure:"response_timeout"`
}
```

Viper defaults:
```go
viper.SetDefault("elasticsearch.addresses", []string{"http://localhost:9200"})
viper.SetDefault("elasticsearch.max_idle_conns", 10)
viper.SetDefault("elasticsearch.response_timeout", "10s")
```

### SMTPConfig

```go
type SMTPConfig struct {
    Host       string   `mapstructure:"host"`
    Port       int      `mapstructure:"port"`
    Username   string   `mapstructure:"username"`
    Password   string   `mapstructure:"password"`
    From       string   `mapstructure:"from"`
    Recipients []string `mapstructure:"recipients"`
    TLSPolicy  string   `mapstructure:"tls_policy"` // "mandatory", "opportunistic", "none"
}
```

Viper defaults:
```go
viper.SetDefault("smtp.host", "localhost")
viper.SetDefault("smtp.port", 587)
viper.SetDefault("smtp.tls_policy", "mandatory")
```

### config.yaml additions

```yaml
elasticsearch:
  addresses:
    - "http://localhost:9200"
  username: ""
  password: ""
  max_idle_conns: 10
  response_timeout: "10s"

smtp:
  host: "localhost"
  port: 587
  username: ""
  password: ""
  from: "alerts@log-analytics.local"
  recipients:
    - "ops@example.com"
  tls_policy: "mandatory"
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `github.com/elastic/go-elasticsearch/v8` raw API | `v9` TypedClient | v9 released 2023 | Compile-time checked schemas; no stringly-typed index calls |
| `net/smtp` directly for SMTP | `go-mail` | go-mail v0.1+ | TLS negotiation, auth, MIME encoding handled |
| Template per index vs index template | Index template with wildcard pattern | ES 7.8+ (composite templates) | One template handles all `logs-*` daily indices |
| `PUT /logs-2026.03.21/_mapping` per-index | `PUT _index_template/logs-template` | ES 7.8+ | Template applied automatically to new indices |

**Deprecated/outdated:**
- `go-elasticsearch/v8`: still maintained but v9 is the locked decision for this project; v9 supports ES 8.x and ES 9.x.
- Legacy index templates (`PUT /_template/{name}`): replaced by composable index templates (`PUT /_index_template/{name}`). Use the composable API — that is what the TypedClient exposes.
- `net/smtp` standard library: functional but lacks EHLO negotiation abstraction, STARTTLS auto-detection, and modern auth mechanisms. go-mail wraps all of this.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `testing` stdlib + testify (already in go.mod) |
| Config file | none — no separate test runner config |
| Quick run command | `go test -race ./internal/elasticsearch/... ./internal/smtp/... ./internal/alert/...` |
| Full suite command | `go test -race ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STORE-01 | Log entries indexed with correct daily index name | unit (mock transport) | `go test -race ./internal/elasticsearch/... -run TestLogIndexer` | No — Wave 0 |
| STORE-02 | Anomaly entries indexed to `anomalies` index | unit (mock transport) | `go test -race ./internal/elasticsearch/... -run TestAnomalyIndexer` | No — Wave 0 |
| STORE-03 | Index templates applied at startup; idempotent | unit (mock transport + verify PUT request) | `go test -race ./internal/elasticsearch/... -run TestApplyIndexTemplates` | No — Wave 0 |
| STORE-04 | OnFailure callback increments ESWriteErrorsTotal | unit (mock transport returning error body) | `go test -race ./internal/elasticsearch/... -run TestBulkIndexerOnFailure` | No — Wave 0 |
| ALERT-01 | DialAndSendWithContext called for each dispatched anomaly | unit (mock SMTP server or interface mock) | `go test -race ./internal/smtp/... -run TestSMTPAlerterDispatch` | No — Wave 0 |
| ALERT-02 | Email body contains all required fields | unit (inspect Msg body) | `go test -race ./internal/smtp/... -run TestAlertEmailBody` | No — Wave 0 |
| ALERT-03 | SMTPConfig values flow through to go-mail client | unit (constructor + config verification) | `go test -race ./internal/smtp/... -run TestSMTPConfig` | No — Wave 0 |

### Mock Transport Pattern (for ES unit tests without a live cluster)

```go
// Source pattern: github.com/elastic/go-elasticsearch/v9@v9.3.1/esutil/bulk_indexer_internal_test.go
type mockTransport struct {
    RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    return t.RoundTripFunc(req)
}

// Use in test:
es, _ := elasticsearch.NewClient(elasticsearch.Config{
    Transport: &mockTransport{
        RoundTripFunc: func(req *http.Request) (*http.Response, error) {
            // Return crafted bulk response body
            body := `{"errors":false,"items":[{"index":{"_id":"abc","status":200}}]}`
            return &http.Response{
                StatusCode: 200,
                Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
                Body:       io.NopCloser(strings.NewReader(body)),
            }, nil
        },
    },
})
```

### Sampling Rate

- **Per task commit:** `go test -race ./internal/elasticsearch/... ./internal/smtp/... ./internal/alert/...`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/elasticsearch/log_indexer_test.go` — covers STORE-01, STORE-04
- [ ] `internal/elasticsearch/anomaly_indexer_test.go` — covers STORE-02
- [ ] `internal/elasticsearch/setup_test.go` — covers STORE-03
- [ ] `internal/smtp/alerter_test.go` — covers ALERT-01, ALERT-02, ALERT-03
- [ ] `internal/alert/dispatcher_test.go` — covers fan-out wiring (ALERT-01, STORE-02 together)

---

## Open Questions

1. **Anomaly JSON serialization of Evidence []LogEntry**
   - What we know: `Anomaly.Evidence` is `[]LogEntry`; `LogEntry` has `Source RawMessage` (includes raw bytes).
   - What's unclear: Should `Source RawMessage.Payload []byte` be excluded from the indexed anomaly document? Raw bytes inflate document size and are already stored in the log index.
   - Recommendation: Add `json:"-"` tag to `RawMessage.Payload` in the `RawMessage` struct (or create a separate `AnomalyDoc` serialization struct that excludes it). Decide at plan time.

2. **TLS policy configuration mapping**
   - What we know: go-mail provides `TLSMandatory`, `TLSOpportunistic`, `NoTLS` constants; config is a string `"mandatory"/"opportunistic"/"none"`.
   - What's unclear: The mapping from config string to go-mail constant is simple but needs a helper function with validation.
   - Recommendation: Implement `parseTLSPolicy(s string) (mail.TLSPolicy, error)` in `internal/smtp/alerter.go`.

3. **ES client `Close()` method in v9**
   - What we know: `elasticsearch.Client` does not have a `Close()` method in the traditional sense; the `BaseClient` has a `Close(ctx context.Context) error` visible in the example test file.
   - What's unclear: The exact semantics of `BaseClient.Close` — whether it drains in-flight requests or just sets a flag.
   - Recommendation: Rely on `BulkIndexer.Close(ctx)` for graceful flush; the ES client itself does not need explicit close at shutdown.

---

## Sources

### Primary (HIGH confidence)

- `github.com/elastic/go-elasticsearch/v9@v9.3.1` — source code read directly from Go module proxy:
  - `elasticsearch.go`: TypedClient, Config, NewTypedClient
  - `esutil/bulk_indexer.go`: BulkIndexer interface, BulkIndexerConfig, BulkIndexerItem, BulkIndexerStats
  - `esutil/bulk_indexer_example_test.go`: canonical usage pattern
  - `esutil/bulk_indexer_internal_test.go`: mockTransport pattern for unit tests
  - `typedapi/indices/putindextemplate/request.go`: Request struct fields
  - `typedapi/indices/putindextemplate/put_index_template.go`: Method signature
  - `typedapi/types/indextemplatemapping.go`: IndexTemplateMapping struct
  - `typedapi/types/typemapping.go`: TypeMapping struct, Dynamic field
  - `typedapi/types/enums/dynamicmapping/dynamicmapping.go`: False, Strict, True constants
  - `typedapi/types/property.go`: Property union type (FlattenedProperty included)
  - `typedapi/api._.go`: MethodIndices.PutIndexTemplate method

- `github.com/wneessen/go-mail@v0.7.2` — source code read directly from Go module proxy:
  - `client.go`: NewClient, WithPort, WithTLSPortPolicy, WithSMTPAuth, WithUsername, WithPassword, DialAndSendWithContext
  - `auth.go`: SMTPAuthPlain, SMTPAuthLogin, SMTPAuthNoAuth constants
  - `msg.go`: NewMsg, From, To, Subject, SetBodyString
  - `encoding.go`: TypeTextPlain, TypeTextHTML ContentType constants
  - `tls.go`: TLSMandatory, TLSOpportunistic, NoTLS constants

- `D:/Project/Golang/log-analytics-and-anomaly-detection/internal/domain/types.go` — LogEntry, Anomaly, RawMessage structs
- `D:/Project/Golang/log-analytics-and-anomaly-detection/internal/domain/interfaces.go` — LogStore, AnomalyStore, AlertChannel interfaces
- `D:/Project/Golang/log-analytics-and-anomaly-detection/internal/metrics/metrics.go` — ESWriteErrorsTotal, EmailAlertsSentTotal, EmailAlertsFailedTotal already registered
- `D:/Project/Golang/log-analytics-and-anomaly-detection/internal/detection/engine.go` — DetectorEngine.Anomalies() channel; cooldown already baked in
- `D:/Project/Golang/log-analytics-and-anomaly-detection/internal/config/config.go` — Existing Config shape, Viper patterns
- `D:/Project/Golang/log-analytics-and-anomaly-detection/go.mod` — Current dependency state; ES and go-mail not yet added

### Secondary (MEDIUM confidence)

- `.planning/research/ARCHITECTURE.md` — errgroup wiring pattern, fan-out channel design (project research, 2026-03-21)
- `.planning/STATE.md` — Locked library decisions (Elasticsearch v9 TypedClient, go-mail v0.7.2)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions confirmed from module proxy; source code read directly
- Architecture: HIGH — TypedAPI and BulkIndexer patterns read from library source; go-mail API verified from source
- Pitfalls: HIGH — derived from reading actual source code (bulk response parsing, TLS defaults, dynamic mapping enum)

**Research date:** 2026-03-21
**Valid until:** 2026-06-21 (stable libraries; go-elasticsearch v9 and go-mail follow semantic versioning)
