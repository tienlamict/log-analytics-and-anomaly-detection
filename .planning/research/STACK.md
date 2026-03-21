# Stack Research: Log Analytics & Anomaly Detection (Go)

**Project:** Log Analytics & Anomaly Detection
**Researched:** 2026-03-21
**Overall Confidence:** HIGH (all recommendations verified against pkg.go.dev and official sources as of March 2026)

---

## Message Queue — Kafka Client

**Recommendation:** `github.com/twmb/franz-go` v1.20.7
**Confidence:** HIGH — verified via pkg.go.dev (published Feb 18, 2026)

### Rationale

franz-go is the correct choice for a new Go Kafka consumer in 2026:

- **Pure Go, no CGo.** Cross-compiles cleanly, Docker image stays minimal, no librdkafka dependency to manage.
- **Fastest Go Kafka client.** Benchmarks show 4x faster than confluent-kafka-go at producing, 10-20x faster consuming, 2.5x faster than Sarama producing. At <1k logs/sec this headroom is irrelevant operationally, but it reflects modern, efficient implementation quality.
- **Full EOS and idempotent producers.** Useful if the pipeline ever needs exactly-once guarantees on the processing side.
- **Actively developed.** v1.20.7 released February 2026 — this is not a dormant library.
- **Production pedigree.** Used by Redpanda, Unity, Hilton, Mux, among others.
- **Prometheus and Zap plugins ship as first-class sub-modules** (`plugin/kprom`, `plugin/kzap`) — no glue code needed for the observability layer.

### Key Sub-packages

```
github.com/twmb/franz-go/pkg/kgo      # Core client
github.com/twmb/franz-go/pkg/kadm     # Admin operations (topic creation in tests)
github.com/twmb/franz-go/plugin/kprom # Prometheus metrics plugin
github.com/twmb/franz-go/plugin/kzap  # Zap logging plugin
```

### Installation

```bash
go get github.com/twmb/franz-go@v1.20.7
go get github.com/twmb/franz-go/plugin/kprom
go get github.com/twmb/franz-go/plugin/kzap
```

---

## Storage — Elasticsearch Client

**Recommendation:** `github.com/elastic/go-elasticsearch/v9` v9.3.1
**Confidence:** HIGH — verified via pkg.go.dev (published Feb 11, 2026)

### Rationale

Use the official Elastic client at v9, not v8:

- **v9 is the current major.** v8 is still supported but v9.3.1 shipped February 2026, is production stable, and is where Elastic is actively investing.
- **TypedClient reduces mapping errors.** v9's `TypedClient` provides Go-typed request/response structs instead of raw `map[string]interface{}`. For log indexing and anomaly queries with known schemas, this eliminates an entire class of runtime errors.
- **Native OpenTelemetry support.** v9 has first-class OTel tracing for Elasticsearch requests — directly useful for the observability layer.
- **Official, Elastic-maintained.** Not a third-party wrapper; Elastic publishes this client, versioned to match the Elasticsearch server API.
- **Apache-2.0 license, 1,287+ known importers.** Broadly adopted, no licensing concerns.

### Client Selection

Use `TypedClient` for application code (schema-aware, compile-time checked):

```go
es, err := elasticsearch.NewTypedClient(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
})
```

Use the functional `Client` only for admin operations where typed API coverage is incomplete.

### Installation

```bash
go get github.com/elastic/go-elasticsearch/v9@v9.3.1
```

---

## HTTP Framework

**Recommendation:** `github.com/go-chi/chi/v5` v5.2.5
**Confidence:** HIGH — verified via pkg.go.dev (published Feb 5, 2026)

### Rationale

chi is the right choice for this project's REST API layer:

- **100% `net/http` compatible.** Any standard `http.Handler` works without adaptation. The stdlib is the interface — chi is just routing and middleware composition on top.
- **No framework lock-in.** When requirements change (add auth middleware, swap a handler, write a pure stdlib test), nothing is fighting you. Standard library patterns work exactly as documented.
- **Lightweight.** ~1000 lines of core router code. No magic, no hidden state, readable when you need to debug it.
- **Production-proven.** Used by Cloudflare, Heroku, Pressly. Not a niche library.
- **Composable middleware stack.** `middleware.Logger`, `middleware.Recoverer`, `middleware.RequestID`, and `middleware.Timeout` cover all standard needs for a log query API without additional dependencies.
- **Route parameter ergonomics suit a query API.** `/api/v1/logs/{id}`, `/api/v1/anomalies` with time-range query params — chi handles this naturally.

### Why Not Gin or Fiber

- **Gin** uses its own `Context` type, breaking standard `http.Handler` compatibility. More total API surface to learn. For a service this size it adds complexity without benefit.
- **Fiber** is built on fasthttp, not `net/http`. Incompatible with standard middleware ecosystem. Its performance advantage matters at millions of requests/sec — not at a log query API.

### Installation

```bash
go get github.com/go-chi/chi/v5@v5.2.5
```

---

## Email / Alerting

**Recommendation:** `github.com/wneessen/go-mail` v0.7.2
**Confidence:** MEDIUM — library is correct choice; v0.7.2 is verified clean (patch for GO-2025-3988 landed in v0.7.1, v0.7.2 is the current release as of Sep 2025)

### Rationale

go-mail is the best maintained dedicated SMTP library in the Go ecosystem today:

- **Security patch current.** Vulnerability GO-2025-3988 (CVE-2025-59937, insufficient address encoding) was fixed in v0.7.1. v0.7.2 is the patched current version. **Do not use v0.6.x.**
- **Full SMTP auth coverage.** PLAIN, LOGIN, CRAM-MD5, SCRAM-SHA-256, XOAUTH2 — supports any enterprise mail relay.
- **Explicit TLS + STARTTLS.** Configurable TLS policies are correct for a system sending anomaly alerts over potentially untrusted network paths.
- **Context-aware.** Supports `context.Context` for timeout/cancellation — essential when anomaly alert sending must not block the detection pipeline.
- **Minimal dependencies.** Relies on Go stdlib + net packages, not a heavy framework.
- **Active maintenance.** v0.7.0, v0.7.1, v0.7.2 all shipped in September 2025. Regular releases.

### Why Not Standard Library `net/smtp`

The stdlib `net/smtp` is frozen ("not accepting new features"). It lacks MIME/HTML support, requires raw RFC 5321 message construction, and has no context timeout integration. go-mail wraps the stdlib cleanly and adds everything needed for production alerting emails.

### Why Not `gopkg.in/gomail.v2`

Last update: April 2016. No go.mod. Not an option for a new project in 2026.

### Installation

```bash
go get github.com/wneessen/go-mail@v0.7.2
```

---

## Testing

**Primary:** `github.com/stretchr/testify` v1.11.1 (published Aug 2025)
**Integration:** `github.com/testcontainers/testcontainers-go` v0.41.0 with Kafka and Elasticsearch modules (published Mar 10, 2026)
**Confidence:** HIGH for testify; MEDIUM for testcontainers (pre-v1, API may shift in minor versions)

### Testify

```bash
go get github.com/stretchr/testify@v1.11.1
```

Use `assert` for non-fatal checks in table-driven tests. Use `require` for setup assertions where the test cannot continue if a precondition fails. Use `mock` for unit-testing components against interfaces (Kafka consumer interface, Elasticsearch writer interface, email sender interface). Avoid `suite` — it adds test lifecycle complexity that Go's standard `TestMain` + helpers handle cleanly.

### Testcontainers-Go

Integration tests for the pipeline need real Kafka and Elasticsearch. Testcontainers-go provides first-class modules for both:

```bash
go get github.com/testcontainers/testcontainers-go@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/kafka@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/elasticsearch@v0.41.0
```

The Kafka module uses `confluentinc/confluent-local` and exposes `Brokers(ctx)` to retrieve connection strings. The Elasticsearch module exposes `Settings.Address` and `Settings.Password`. Both containers are created per `TestMain` and shared across tests in a package — not per test function (container startup is slow).

### Testing Strategy

- **Unit tests:** No external dependencies. Mock all I/O interfaces. Fast, run in CI on every commit.
- **Integration tests:** Testcontainers (Docker required). Tagged with `//go:build integration`. Run separately in CI on PRs.
- **Standard library `testing`** drives everything — testify and testcontainers are helpers, not a framework.

---

## Observability / Metrics

**Recommendation:** `github.com/prometheus/client_golang` v1.23.2 for metrics; `go.uber.org/zap` v1.27.1 for structured logging
**Confidence:** HIGH — both verified via pkg.go.dev (Sep 2025 and Nov 2025 respectively)

### Prometheus Metrics

```bash
go get github.com/prometheus/client_golang@v1.23.2
```

Expose a `/metrics` HTTP endpoint via `promhttp.Handler()`. Key metrics for this pipeline:

- `logs_consumed_total` — Counter, partition-labeled
- `logs_processed_duration_seconds` — Histogram of processing latency
- `anomalies_detected_total` — Counter, rule-labeled
- `elasticsearch_write_errors_total` — Counter
- `email_alerts_sent_total` / `email_alerts_failed_total` — Counters
- `kafka_consumer_lag` — Gauge (available via franz-go `plugin/kprom`)

Apache-2.0 license. 71,290+ importers. Industry standard for Go service instrumentation.

### Structured Logging — Zap

```bash
go get go.uber.org/zap@v1.27.1
```

Use `zap.NewProduction()` for deployed instances (JSON output, Info level by default). Use `zap.NewDevelopment()` locally. Wire into franz-go via `plugin/kzap` so Kafka client events emit structured log lines.

Key properties that make zap correct for this project:

- **Structured fields, not printf.** Log entries for anomaly detections need consistent machine-readable fields (`rule_id`, `log_source`, `count`, `window_seconds`) — zap enforces this without convention.
- **AtomicLevel.** Log level changeable at runtime via HTTP without restart — useful for investigating production issues.
- **Performance.** 118,717+ importers; effectively an industry standard. The library will not be a bottleneck.

### OpenTelemetry (Optional)

`go.opentelemetry.io/otel` v1.42.0 is production stable for traces and metrics (logs are in beta). For v1 of this project, Prometheus + Zap is sufficient and simpler to operate. Add OTel tracing in a later phase when distributed tracing (across the Kafka consumer → detection → storage path) becomes valuable.

---

## What NOT to Use

| Library | Category | Why Not |
|---------|----------|---------|
| `github.com/confluentinc/confluent-kafka-go/v2` | Kafka | Uses CGo / librdkafka. Complicates cross-compilation, Docker images, and builds. Pure-Go franz-go is faster and simpler. |
| `github.com/IBM/sarama` | Kafka | Older API design, less idiomatic Go. Still maintained (v1.47.0, Feb 2026) but franz-go is the modern replacement. 2,649 importers vs franz-go's broader industry adoption. Use sarama only if you need compatibility with an existing codebase that already uses it. |
| `github.com/elastic/go-elasticsearch/v8` | Elasticsearch | v8 is not the current major. v9 has TypedClient, native OTel, and is where Elastic is actively developing. No reason to start on v8 for a greenfield project. |
| `github.com/gin-gonic/gin` | HTTP | Custom `Context` type breaks `net/http` compatibility. More API surface for no benefit at this scale. chi is lighter and more composable. |
| `github.com/gofiber/fiber/v2` | HTTP | Built on fasthttp, not `net/http`. Incompatible middleware ecosystem. Performance gains irrelevant at <1k events/sec log query API volume. |
| `gopkg.in/gomail.v2` | Email | Abandoned since 2016. No go.mod. Not suitable for any new Go project. |
| `net/smtp` (stdlib) | Email | Frozen, no new features. No MIME, no HTML email, no context cancellation. go-mail wraps it correctly; use go-mail instead. |
| `github.com/sirupsen/logrus` | Logging | Logrus is in maintenance mode (no new features). zap and zerolog are the modern replacements. logrus has a reflection-based API that is slower and less type-safe. |
| OTel logging (beta) | Logging | `go.opentelemetry.io/otel/log` is in beta as of March 2026. Do not use beta signal APIs in production pipeline code. Use zap for application logging. |

---

## Full Dependency Summary

```bash
# Kafka
go get github.com/twmb/franz-go@v1.20.7
go get github.com/twmb/franz-go/plugin/kprom
go get github.com/twmb/franz-go/plugin/kzap

# Elasticsearch
go get github.com/elastic/go-elasticsearch/v9@v9.3.1

# HTTP
go get github.com/go-chi/chi/v5@v5.2.5

# Email
go get github.com/wneessen/go-mail@v0.7.2

# Observability
go get github.com/prometheus/client_golang@v1.23.2
go get go.uber.org/zap@v1.27.1

# Testing
go get github.com/stretchr/testify@v1.11.1
go get github.com/testcontainers/testcontainers-go@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/kafka@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/elasticsearch@v0.41.0
```

---

## Sources

- franz-go: https://pkg.go.dev/github.com/twmb/franz-go (verified Mar 2026)
- go-elasticsearch v9: https://pkg.go.dev/github.com/elastic/go-elasticsearch/v9 (verified Mar 2026)
- chi v5: https://pkg.go.dev/github.com/go-chi/chi/v5 (verified Mar 2026)
- go-mail: https://pkg.go.dev/github.com/wneessen/go-mail (verified Mar 2026)
- go-mail vuln GO-2025-3988: https://pkg.go.dev/vuln/GO-2025-3988 (patch confirmed in v0.7.1)
- prometheus/client_golang: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus (verified Mar 2026)
- uber-go/zap: https://pkg.go.dev/go.uber.org/zap (verified Mar 2026)
- testify: https://pkg.go.dev/github.com/stretchr/testify (verified Mar 2026)
- testcontainers-go: https://pkg.go.dev/github.com/testcontainers/testcontainers-go (verified Mar 2026)
- testcontainers kafka module: https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/kafka (verified Mar 2026)
- testcontainers elasticsearch module: https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/elasticsearch (verified Mar 2026)
- confluent-kafka-go (considered, rejected): https://pkg.go.dev/github.com/confluentinc/confluent-kafka-go/v2/kafka
- IBM/sarama (considered, rejected): https://pkg.go.dev/github.com/IBM/sarama
- gomail (considered, rejected): https://pkg.go.dev/gopkg.in/gomail.v2
- net/smtp (considered, rejected): https://pkg.go.dev/net/smtp
- OpenTelemetry Go: https://pkg.go.dev/go.opentelemetry.io/otel (verified Mar 2026)
