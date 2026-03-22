# Phase 5: Integration and Hardening - Research

**Researched:** 2026-03-22
**Domain:** testcontainers-go, graceful shutdown, Go integration test patterns, pipeline wiring
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TEST-02 | Integration tests using testcontainers (real Kafka + Elasticsearch) covering the end-to-end pipeline | Sections: Standard Stack, Architecture Patterns, Code Examples |
</phase_requirements>

---

## Summary

Phase 5 closes the loop on the project by proving that all components wired together produce observable outcomes: logs land in ES, anomalies appear when detection thresholds are crossed, offsets survive consumer restarts, and the process exits cleanly within 10 seconds of SIGTERM with no goroutine leaks.

The main.go in this project currently only starts a metrics HTTP server. It does NOT wire Kafka consumer, detection engine, log indexer, anomaly dispatcher, or the chi API server. Every component exists and is tested in isolation, but none are connected in main.go. Phase 5 must first complete the pipeline wiring in `cmd/server/main.go` using `errgroup`, then write integration tests that exercise the wired pipeline end-to-end.

The integration test suite uses `testcontainers-go` v0.41.0 (released 2026-03-10) with the `kafka` and `elasticsearch` modules. Containers are started once per package in `TestMain` and shared across all test functions — never started per individual test. The `//go:build integration` constraint gates these tests so `go test ./...` does not require Docker.

**Primary recommendation:** Wire `cmd/server/main.go` first in plan 05-03, then write integration tests that validate the fully-wired pipeline in plans 05-01 and 05-02.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| testcontainers-go | v0.41.0 | Spin up real Kafka and Elasticsearch in Docker | Specified in roadmap; confirmed current as of 2026-03-10 |
| testcontainers-go/modules/kafka | v0.41.0 | KRaft-mode Kafka container via `confluentinc/confluent-local` | Part of testcontainers-go module tree; separate go.mod entry |
| testcontainers-go/modules/elasticsearch | v0.41.0 | Elasticsearch container with TLS + credentials | Part of testcontainers-go module tree; separate go.mod entry |
| golang.org/x/sync/errgroup | latest | Pipeline goroutine coordination with context propagation | Standard for fan-out/fan-in shutdown; already in transitive deps |
| go.uber.org/goleak | v1.3.0 | Goroutine leak detection | Already in go.mod; verified working pattern in existing unit tests |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| franz-go/pkg/kgo | v1.20.7 | Produce test messages to Kafka in integration tests | Already in go.mod; use `ProduceSync` to guarantee delivery before asserting |
| go-elasticsearch/v9 | v9.3.1 | Build ES client from container settings in integration tests | Already in go.mod; client must use `CACert` when ES container uses HTTPS |
| testify/assert + require | v1.11.1 | Assertion helpers in test scenarios | Already in go.mod |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| testcontainers-go Kafka | kfake (in-memory fake) | kfake doesn't test real offset semantics or restart scenarios; testcontainers required by TEST-02 |
| errgroup for wiring | manual goroutine + WaitGroup | errgroup propagates context cancellation and collects first error; standard for Go pipeline wiring |

**Installation:**
```bash
go get github.com/testcontainers/testcontainers-go@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/kafka@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/elasticsearch@v0.41.0
golang.org/x/sync is already a transitive dependency; verify with: go get golang.org/x/sync
```

**Module note:** testcontainers-go uses a mono-repo structure where each module (`modules/kafka`, `modules/elasticsearch`) has its own `go.mod`. All three require separate `go get` invocations.

---

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/          # new package — all integration tests
├── main_test.go               # TestMain: start containers once, goleak.VerifyTestMain
├── pipeline_test.go           # Kafka→ES log indexing scenarios
├── detection_test.go          # anomaly detection end-to-end scenarios
├── offset_test.go             # offset commitment / restart scenario
└── shutdown_test.go           # graceful shutdown wall-clock timing

cmd/server/
└── main.go                    # MUST be fully wired before integration tests
```

### Pattern 1: TestMain-Scoped Shared Containers

**What:** Start one Kafka container and one Elasticsearch container in `TestMain`, share them across every test function in the package. Terminate only after all tests complete.

**When to use:** Always for integration tests — starting a container per test is 10–30x slower and the TCP port overhead causes flakiness.

**Example:**
```go
// Source: https://golang.testcontainers.org/modules/kafka/  (verified)
//go:build integration

package integration

import (
    "context"
    "log"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
    tces "github.com/testcontainers/testcontainers-go/modules/elasticsearch"
    "go.uber.org/goleak"
)

var (
    kafkaContainer *tckafka.KafkaContainer
    esContainer    *tces.ElasticsearchContainer
)

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

func init() {
    ctx := context.Background()
    var err error

    kafkaContainer, err = tckafka.Run(ctx,
        "confluentinc/confluent-local:7.5.0",
        tckafka.WithClusterID("test-cluster"),
    )
    if err != nil {
        log.Fatalf("failed to start kafka: %s", err)
    }

    esContainer, err = tces.Run(ctx,
        "docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
        tces.WithPassword("changeme"),
    )
    if err != nil {
        log.Fatalf("failed to start elasticsearch: %s", err)
    }
}
```

**CRITICAL NOTE:** `goleak.VerifyTestMain(m)` must be called in `TestMain`, not `init`. Container startup can go in a `TestMain` pre-`m.Run()` block or in `init`; the pattern above separates concerns. Container teardown via `defer testcontainers.TerminateContainer(...)` goes before `m.Run()`.

### Pattern 2: Correct TestMain with Container Teardown

**What:** Full TestMain handling — setup, run tests, teardown, goleak check.

```go
func TestMain(m *testing.M) {
    ctx := context.Background()

    var err error
    kafkaContainer, err = tckafka.Run(ctx, "confluentinc/confluent-local:7.5.0",
        tckafka.WithClusterID("test-cluster"),
    )
    if err != nil {
        log.Fatalf("kafka container: %s", err)
    }
    defer func() {
        if err := testcontainers.TerminateContainer(kafkaContainer); err != nil {
            log.Printf("terminate kafka: %s", err)
        }
    }()

    esContainer, err = tces.Run(ctx,
        "docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
        tces.WithPassword("changeme"),
    )
    if err != nil {
        log.Fatalf("elasticsearch container: %s", err)
    }
    defer func() {
        if err := testcontainers.TerminateContainer(esContainer); err != nil {
            log.Printf("terminate es: %s", err)
        }
    }()

    goleak.VerifyTestMain(m)
}
```

**PITFALL:** `goleak.VerifyTestMain(m)` calls `m.Run()` internally. You must NOT also call `os.Exit(m.Run())`. The function handles both running tests and checking leaks.

### Pattern 3: Getting Container Connection Details

```go
// Kafka broker addresses (host:port, randomly assigned)
brokers, err := kafkaContainer.Brokers(ctx)
// returns []string{"localhost:NNNN"}

// Elasticsearch settings
addr := esContainer.Settings.Address  // "https://localhost:NNNN" or "http://..."
user := esContainer.Settings.Username // "elastic"
pass := esContainer.Settings.Password // "changeme" (or WithPassword value)
cert := esContainer.Settings.CACert   // []byte, non-nil when HTTPS (ES 8+)
```

### Pattern 4: Building ES Client from Container in Tests

```go
// Source: https://golang.testcontainers.org/modules/elasticsearch/ (verified)
esCfg := elasticsearch.Config{
    Addresses: []string{esContainer.Settings.Address},
    Username:  esContainer.Settings.Username,
    Password:  esContainer.Settings.Password,
    CACert:    esContainer.Settings.CACert, // required for ES 8+ HTTPS
}
client, err := elasticsearch.NewTypedClient(esCfg)
```

**CRITICAL:** The existing `elasticsearch.NewClient` in `internal/elasticsearch/client.go` uses `http.Transport` with custom fields. For test client construction from container settings, bypass this constructor and build directly from `elasticsearch.Config{CACert: ...}` to avoid TLS certificate failures.

### Pattern 5: Producing to Kafka in Tests (franz-go ProduceSync)

```go
// Source: https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo (verified — already used in project)
producer, err := kgo.NewClient(
    kgo.SeedBrokers(brokers...),
)
require.NoError(t, err)
defer producer.Close()

results := producer.ProduceSync(ctx, &kgo.Record{
    Topic: "application-logs",
    Value: []byte(`{"level":"error","service":"api","message":"db connection failed"}`),
})
require.NoError(t, results.FirstErr())
```

**Why ProduceSync over Produce:** `ProduceSync` blocks until broker acknowledges. Integration tests must guarantee the message is in the topic before the consumer's poll cycle picks it up.

### Pattern 6: errgroup Pipeline Wiring in main.go

```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup (verified)
import "golang.org/x/sync/errgroup"

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

g, gCtx := errgroup.WithContext(ctx)

// Consumer goroutine
g.Go(func() error { return consumer.Run(gCtx) })

// Pipeline worker goroutine
g.Go(func() error {
    for msg := range consumer.Messages() {
        entry := pipeline.ProcessMessage(msg, logger)
        logIndexer.IndexLog(gCtx, entry)
        detector.Evaluate(entry)
    }
    return nil
})

// Dispatcher goroutine
g.Go(func() error { return dispatcher.Run(gCtx) })

// SMTP alerter goroutine
g.Go(func() error { alerter.Run(gCtx); return nil })

// HTTP API server goroutine
g.Go(func() error {
    if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return err
    }
    return nil
})

// Shutdown watcher goroutine
g.Go(func() error {
    <-gCtx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    apiServer.Shutdown(shutdownCtx)
    logIndexer.Close(shutdownCtx)
    anomalyIndexer.Close(shutdownCtx)
    detector.Stop()
    return nil
})

if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
    logger.Error("pipeline error", zap.Error(err))
    os.Exit(1)
}
```

### Pattern 7: build constraint for integration tests

```go
//go:build integration
// (blank line required after build constraint)
package integration
```

Run: `go test -race -tags integration ./internal/integration/...`

The modern `//go:build` syntax (Go 1.17+) is the correct form; `// +build integration` is deprecated.

### Anti-Patterns to Avoid

- **Starting containers per test function:** Each container start takes 5–20 seconds. Use TestMain-scoped containers only.
- **Forgetting `CACert` in ES 8+ client:** ES 8+ uses HTTPS by default; omitting `CACert` causes x509 certificate verification errors.
- **Using `goleak.VerifyNone(t)` in parallel tests:** Produces false positives. `goleak.VerifyTestMain(m)` is the only correct pattern when tests use goroutines.
- **Not calling `BulkIndexer.Close()` before asserting ES documents exist:** Bulk indexer buffers writes; documents are not visible in ES until the flush interval elapses or `Close()` is called. In tests, call `Close()` on indexers before querying ES.
- **Polling ES without retry:** ES indexing has a brief replication delay even after `Close()`. Use `assert.Eventually` or a short retry loop (e.g., 10 x 500ms) rather than a single assertion.
- **Calling `os.Exit(m.Run())` when using `goleak.VerifyTestMain`:** `VerifyTestMain` calls `m.Run()` internally; double-calling is a no-op but confusing; omit the `os.Exit` call.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Container lifecycle | Custom Docker SDK wrappers | testcontainers-go Run + TerminateContainer | Handles port mapping, wait strategies, cleanup on test failure |
| Kafka broker address discovery | Hardcoded "localhost:9092" | `kafkaContainer.Brokers(ctx)` | Container uses random ephemeral ports; hardcoded port fails in CI |
| ES test client TLS | Manual x509 pool building | `CACert` field in `elasticsearch.Config` | go-elasticsearch/v9 handles cert pool internally |
| Goroutine leak check | Inspecting runtime.NumGoroutine | `goleak.VerifyTestMain(m)` | goleak identifies specific leaked goroutines with stack traces |
| Retry-until-ES-visible | time.Sleep | `assert.Eventually(t, fn, 10*time.Second, 500*time.Millisecond)` | ES indexing is asynchronous; Eventually retries cleanly |
| Pipeline goroutine coordination | sync.WaitGroup + channels | `errgroup.WithContext` | errgroup propagates context cancellation to all goroutines on first error |

**Key insight:** The BulkIndexer is the most important "don't hand-roll" item — its internal flush interval means that integration test assertions MUST trigger an explicit flush via `Close()` or wait for the flush interval to elapse. A test that asserts "document exists" immediately after `IndexLog()` will flake without this.

---

## Graceful Shutdown Sequence (LIFO)

The correct LIFO shutdown order for this pipeline is:

```
signal.NotifyContext cancels root context
    ↓
consumer.Run(ctx) returns (PollFetches returns when ctx cancelled)
    ↓ (consumer.out channel is closed by defer in consumer.Run)
pipeline worker goroutine exits (range over closed channel terminates)
    ↓
detector.Stop() is called (closes evictStop, stops eviction goroutine)
    ↓
dispatcher.Run(ctx) returns when ctx.Done() fires
    ↓
alerter.Run(ctx) drains queue then returns when ctx.Done() fires
    ↓
logIndexer.Close(shutdownCtx) — flushes BulkIndexer buffer
    ↓
anomalyIndexer.Close(shutdownCtx) — flushes BulkIndexer buffer
    ↓
apiServer.Shutdown(shutdownCtx) — drains HTTP connections
    ↓
errgroup.Wait() returns
    ↓
Process exits
```

**PITFALL K5 (from roadmap):** If `BulkIndexer.Close()` is not called before process exit, buffered documents that haven't reached the flush interval are silently dropped. The shutdown integration test asserting < 10 seconds forces a bounded deadline on this.

**Shutdown integration test approach:**
```go
// Send SIGTERM to the process under test and measure wall-clock time to exit.
// Use exec.Command to start the server as a subprocess, then cmd.Process.Signal(syscall.SIGTERM).
// Measure time.Since(start) after cmd.Wait() returns.
// Assert elapsed < 10 * time.Second.
```

---

## Common Pitfalls

### Pitfall 1: ES BulkIndexer asynchronous flush
**What goes wrong:** Test calls `IndexLog(ctx, entry)` then immediately queries ES and finds 0 documents.
**Why it happens:** BulkIndexer batches writes; flush happens at interval (5 seconds for logs) or at 5MB.
**How to avoid:** Call `logIndexer.Close(ctx)` after producing test data and before asserting. Use `assert.Eventually` with a timeout of 10 seconds and 500ms poll interval.
**Warning signs:** Tests pass locally with a `time.Sleep` and fail in CI.

### Pitfall 2: Warm-up period suppresses anomalies in integration tests
**What goes wrong:** Integration test triggers error-rate rule threshold but no anomaly appears in the `anomalies` index.
**Why it happens:** `DetectorEngine` has a warm-up period of `2 × maxWindow` (default 2×5m = 10 minutes). All anomalies are suppressed during warm-up.
**How to avoid:** Either (a) set `warmup_multiplier: 0` in integration test config, or (b) inject a `DetectorEngine` with a zero `warmupUntil` timestamp. Option (a) is simpler.
**Warning signs:** Anomaly channel receives nothing despite threshold being crossed; no error in logs.

### Pitfall 3: Container port collision in CI
**What goes wrong:** Two test runs start simultaneously; both try to bind port 9092.
**Why it happens:** CI runners execute multiple test processes in parallel.
**How to avoid:** Never hardcode Kafka port. Always use `kafkaContainer.Brokers(ctx)` which returns the randomly-assigned ephemeral port.
**Warning signs:** `connection refused` on port 9092 in CI but not locally.

### Pitfall 4: Missing X-Elastic-Product header (already known from existing tests)
**What goes wrong:** go-elasticsearch/v9 client silently rejects responses from ES mock.
**Why it happens:** v9 client validates `X-Elastic-Product: Elasticsearch` header before parsing.
**How to avoid:** This pitfall is already documented in STATE.md. Use real ES container (not mock) for integration tests — the real container always returns the correct header.
**Warning signs:** Tests pass unit but fail integration with "unexpected response" or empty results.

### Pitfall 5: goleak false positives with testcontainers goroutines
**What goes wrong:** `goleak.VerifyTestMain` reports goroutine leaks from testcontainers internal goroutines.
**Why it happens:** testcontainers library starts background goroutines for container lifecycle management.
**How to avoid:** Use `goleak.IgnoreTopFunction("...")` filters or call `TerminateContainer` before `VerifyTestMain` returns. Ensure all containers are terminated before `m.Run()` returns.
**Warning signs:** goleak reports goroutines from `testcontainers-go` package path.

### Pitfall 6: main.go is not wired — integration tests have nothing to test
**What goes wrong:** Integration tests try to start the server pipeline but `main.go` only runs a metrics HTTP server.
**Why it happens:** main.go was built incrementally per plan; full wiring deferred to Phase 5.
**How to avoid:** Plan 05-03 (pipeline wiring and README) must complete before or alongside integration tests in plans 05-01 and 05-02. The integration test can either import and invoke the wiring function directly, or start the server as a subprocess.
**Warning signs:** Test can't import the wired pipeline because it doesn't exist yet.

---

## Code Examples

Verified patterns from official sources:

### Starting Kafka Container
```go
// Source: https://golang.testcontainers.org/modules/kafka/ (verified)
kafkaContainer, err := tckafka.Run(ctx,
    "confluentinc/confluent-local:7.5.0",
    tckafka.WithClusterID("test-cluster"),
)
brokers, err := kafkaContainer.Brokers(ctx)
// brokers = ["localhost:NNNNN"] — ephemeral port
```

### Starting Elasticsearch Container
```go
// Source: https://golang.testcontainers.org/modules/elasticsearch/ (verified)
esContainer, err := tces.Run(ctx,
    "docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
    tces.WithPassword("changeme"),
)
// esContainer.Settings.Address = "https://localhost:NNNNN"
// esContainer.Settings.CACert = []byte{...} (TLS cert for v8+)
```

### Building go-elasticsearch TypedClient from Container Settings
```go
esCfg := elasticsearch.Config{
    Addresses: []string{esContainer.Settings.Address},
    Username:  esContainer.Settings.Username,
    Password:  esContainer.Settings.Password,
    CACert:    esContainer.Settings.CACert,
}
esClient, err := elasticsearch.NewTypedClient(esCfg)
```

### Applying Index Templates and Running Pipeline in Integration Test
```go
err = esutil.ApplyIndexTemplates(ctx, esClient) // existing function in internal/elasticsearch/setup.go
require.NoError(t, err)

logIndexer, err := esutil.NewLogIndexer(esClient, logger)
require.NoError(t, err)
defer logIndexer.Close(context.Background())

anomalyIndexer, err := esutil.NewAnomalyIndexer(esClient, logger)
require.NoError(t, err)
defer anomalyIndexer.Close(context.Background())
```

### Asserting ES Document Exists with Retry
```go
assert.Eventually(t, func() bool {
    entries, total, err := logIndexer.SearchLogs(ctx, domain.LogQuery{Service: "api"})
    return err == nil && total > 0 && len(entries) > 0
}, 10*time.Second, 500*time.Millisecond, "expected log document in ES within 10s")
```

### Shutdown Integration Test (wall-clock timing)
```go
func TestShutdownUnder10Seconds(t *testing.T) {
    // Start server subprocess with test config
    cmd := exec.Command("go", "run", "./cmd/server",
        "-config", "testdata/integration-config.yaml",
    )
    require.NoError(t, cmd.Start())

    // Give server time to initialize
    time.Sleep(2 * time.Second)

    // Signal SIGTERM and measure
    start := time.Now()
    cmd.Process.Signal(syscall.SIGTERM)
    err := cmd.Wait()
    elapsed := time.Since(start)

    assert.Less(t, elapsed, 10*time.Second,
        "server must exit within 10 seconds of SIGTERM")
    // context.Canceled is acceptable exit via signal
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `RunContainer(ctx, opts...)` | `Run(ctx, img, opts...)` | testcontainers-go v0.28+ | `RunContainer` deprecated, will be removed in next major |
| `// +build integration` | `//go:build integration` | Go 1.17 | Old form deprecated; both still work but go vet warns on old form |
| `testcontainers.CleanupContainer(t, c)` | `testcontainers.TerminateContainer(c)` | v0.36+ | `CleanupContainer` deprecated in favor of `TerminateContainer` |

**Deprecated/outdated:**
- `RunContainer`: Use `Run` instead for all testcontainers modules
- `goleak.VerifyNone(t)`: Fine for non-parallel tests, but `VerifyTestMain(m)` is the correct form for any test package with goroutines

---

## Pipeline Wiring Gap (Critical Finding)

**Current state of `cmd/server/main.go`:** Only wires config, logger, signal handling, and a bare metrics HTTP server. Zero pipeline components are started.

**What Phase 5 must add to main.go:**
1. Elasticsearch client construction (`elasticsearch.NewClient`)
2. `elasticsearch.ApplyIndexTemplates` (fail-fast if ES unreachable)
3. `NewLogIndexer` + `NewAnomalyIndexer`
4. `kafka.New` consumer
5. `detection.NewDetectorEngine` with all 7 rules
6. `smtp.NewSMTPAlerter` + `alert.NewDispatcher`
7. `api.NewServer` (chi router + all handlers)
8. `errgroup.WithContext` to run all components concurrently
9. Graceful shutdown coordination — LIFO via shutdown watcher goroutine

**Wiring function approach:** Extract wiring logic into `cmd/server/wire.go` as a `func wire(cfg Config, logger *zap.Logger) error` function. This makes the integration tests able to import and call the wiring function directly rather than spawning a subprocess. It also keeps `main()` small.

The integration tests can either:
- **Option A:** Import the wiring function and run the full pipeline in-process, passing test-configured ES/Kafka URLs from containers. This is cleaner and avoids subprocess management.
- **Option B:** Start `go run ./cmd/server` as a subprocess. Heavier but tests the real binary.

Option A is recommended for plans 05-01 and 05-02 (functional scenarios). Option B is recommended for plan 05-02 (shutdown wall-clock test), since measuring in-process shutdown timing is unreliable.

---

## Open Questions

1. **ES image version for integration tests**
   - What we know: testcontainers ES module docs use `docker.elastic.co/elasticsearch/elasticsearch:8.9.0`. The project uses `go-elasticsearch/v9` client (ES 9.x API).
   - What's unclear: ES 8.x containers work with the v9 Go client (the Go client version is independent of the ES server version for basic operations). However, using an ES 9.x Docker image (`9.0.0`) would be more consistent.
   - Recommendation: Use `docker.elastic.co/elasticsearch/elasticsearch:8.9.0` for stability. ES 9.x Docker images are new (April 2025) and less battle-tested in CI. The v9 Go client communicates with ES 8.x without issue for the operations used in this project.

2. **Warm-up bypass for integration tests**
   - What we know: `DetectorEngine.warmupUntil` is set at construction time and cannot be changed externally. Default is 10 minutes.
   - What's unclear: Whether to add a test-only constructor option or set `warmup_multiplier: 0` in integration test config.
   - Recommendation: Add `detection.WithWarmupUntil(t time.Time)` functional option to `NewDetectorEngine`, allowing integration tests to pass `time.Time{}` (zero value = no warmup). This is minimal code and avoids config coupling.

3. **Kafka topic creation**
   - What we know: `confluentinc/confluent-local:7.5.0` in KRaft mode auto-creates topics on first produce by default.
   - What's unclear: Whether the consumer group offset assignment works correctly before the topic exists.
   - Recommendation: Produce one "seed" message to create the topic before starting the consumer in integration tests, or use `AdminClient` to pre-create the topic.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 + goleak v1.3.0 |
| Config file | none — standard go test flags |
| Quick run command | `go test -race ./...` (unit tests only) |
| Full suite command | `go test -race -tags integration ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-02 | Log message published to Kafka appears in ES `logs-*` index | integration | `go test -race -tags integration ./internal/integration/... -run TestLogIndexing` | Wave 0 |
| TEST-02 | Error burst triggers anomaly in `anomalies` ES index | integration | `go test -race -tags integration ./internal/integration/... -run TestAnomalyDetection` | Wave 0 |
| TEST-02 | Consumer resumes from correct offset after restart | integration | `go test -race -tags integration ./internal/integration/... -run TestOffsetResume` | Wave 0 |
| TEST-02 | No duplicate documents for replayed offsets | integration | `go test -race -tags integration ./internal/integration/... -run TestIdempotentReplay` | Wave 0 |
| TEST-02 | Process exits within 10s of SIGTERM | integration | `go test -race -tags integration ./internal/integration/... -run TestShutdownTiming` | Wave 0 |
| TEST-02 | No goroutine leaks after shutdown | integration | `go test -race -tags integration ./internal/integration/...` (goleak in TestMain) | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race ./...` (unit tests — seconds)
- **Per wave merge:** `go test -race -tags integration ./...` (integration suite — minutes, requires Docker)
- **Phase gate:** Full integration suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/integration/main_test.go` — TestMain with container startup, teardown, goleak
- [ ] `internal/integration/pipeline_test.go` — Kafka→ES log indexing + anomaly detection scenarios
- [ ] `internal/integration/shutdown_test.go` — SIGTERM wall-clock timing test
- [ ] `cmd/server/wire.go` — wiring function (dependency for integration tests)
- [ ] `testdata/integration-config.yaml` — test config with short windows, zero warmup

---

## Sources

### Primary (HIGH confidence)
- `golang.testcontainers.org/modules/kafka/` — Kafka module API, `Run()` signature, `Brokers()` method, KRaft image requirement
- `golang.testcontainers.org/modules/elasticsearch/` — ES module API, `Run()` signature, `Settings` struct fields, TLS/CACert usage
- `pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/kafka@v0.41.0` — verified version v0.41.0 published 2026-03-10
- `pkg.go.dev/go.uber.org/goleak` — `VerifyTestMain` vs `VerifyNone` pattern documented
- Project source code in `internal/` — all component APIs verified directly
- `go.mod` in project root — all existing dependency versions confirmed

### Secondary (MEDIUM confidence)
- `github.com/testcontainers/testcontainers-go/releases` — v0.41.0 release date confirmed
- `victoriametrics.com/blog/go-graceful-shutdown/` — errgroup + signal.NotifyContext LIFO pattern
- `pkg.go.dev/github.com/twmb/franz-go/pkg/kgo` — ProduceSync API (project already uses franz-go v1.20.7)

### Tertiary (LOW confidence)
- None — all critical claims verified via official sources

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — testcontainers-go v0.41.0 verified on pkg.go.dev and release page; all other libraries already in go.mod
- Architecture: HIGH — TestMain pattern verified from official docs; wiring gap identified by reading actual main.go
- Pitfalls: HIGH — warm-up pitfall from reading DetectorEngine source; BulkIndexer flush pitfall from reading log_indexer.go; others from official docs

**Research date:** 2026-03-22
**Valid until:** 2026-06-22 (testcontainers-go releases ~every 2 months; check for new version before implementation)
