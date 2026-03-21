# Pitfalls Research: Log Analytics & Anomaly Detection

**Domain:** Event-driven log analytics pipeline — Go, Kafka, Elasticsearch
**Researched:** 2026-03-21
**Overall confidence:** HIGH (Sarama docs verified via pkg.go.dev; Go pipeline/race patterns verified via go.dev; ES/Kafka patterns from well-established operational knowledge)

---

## Kafka Consumer Pitfalls

### Pitfall K1: Auto-Commit Commits Before Processing Completes (Silent Data Loss)

**Confidence:** HIGH (verified via Sarama docs)

**What goes wrong:** Sarama's consumer group does NOT auto-commit by default, but if `Consumer.Offsets.AutoCommit.Enable = true`, the commit fires on a timer — not after your handler returns. If the process crashes between the auto-commit firing and your handler finishing, those messages are silently lost. They are marked consumed but were never successfully processed.

**Why it happens:** Developers copy config snippets that enable auto-commit without understanding it is time-based, not completion-based.

**Consequences:** Silent message loss. No error is raised. The anomaly detection pipeline drops log entries with no indication. You find out when anomalies that should have been caught weren't.

**Prevention:**
- Set `Consumer.Offsets.AutoCommit.Enable = false`
- Call `session.MarkMessage(msg, "")` only after successful processing
- Let Sarama's `ConsumerGroupHandler` commit at session `Cleanup()` by relying on `MarkMessage` + the default commit-on-rebalance behavior

**Detection warning signs:** Consumer lag metric stays at zero even when processing failures occur; anomaly count drops suspiciously during restarts.

---

### Pitfall K2: max.poll.interval.ms / Session.Timeout Mismatch Causes Rebalance Storms

**Confidence:** HIGH (verified via Sarama docs + Kafka protocol knowledge)

**What goes wrong:** Sarama's `Consumer.Group.Session.Timeout` defaults to 10 seconds. If your handler processes a batch that takes longer than this — including calls to Elasticsearch bulk index — the broker considers the consumer dead, triggers a rebalance, and another consumer re-processes the same messages. This causes duplicate processing and can cause a rebalance storm if the same messages keep triggering slow processing.

**Why it happens:** Development uses tiny payloads and fast Elasticsearch. Production has real log volumes and occasional ES slowdowns. The timeout that was "fine" in dev causes constant rebalancing under load.

**Consequences:** Duplicate anomaly alerts. Elasticsearch gets duplicate documents. Consumer group thrashes. Logs stop being processed because all consumers are in rebalance state.

**Prevention:**
```go
config.Consumer.Group.Session.Timeout = 30 * time.Second  // Match to worst-case processing
config.Consumer.Group.Heartbeat.Interval = 3 * time.Second // Must be < Session.Timeout / 3
config.Consumer.MaxProcessingTime = 10 * time.Second        // Per-message limit
```
- Keep handler processing time bounded and predictable
- Use async or batched Elasticsearch indexing with a bounded timeout
- Monitor `consumer-group-join-failed` metric — sudden increase signals this problem

---

### Pitfall K3: Initial Offset at OffsetNewest Skips Historical Messages on First Deploy

**Confidence:** HIGH (verified via Sarama docs)

**What goes wrong:** Sarama defaults `Consumer.Offsets.Initial = sarama.OffsetNewest`. A new consumer group sees only messages produced after it first connects. If you deploy the system and Kafka has a backlog, all existing log messages are permanently skipped. In replay scenarios (re-running anomaly detection after a rule change), you also silently start from the wrong position.

**Why it happens:** The default is sensible for some use cases but is wrong for a log analytics system that needs to process all messages.

**Prevention:**
- Explicitly set `Consumer.Offsets.Initial = sarama.OffsetOldest` for the first deployment
- Document the intent: each consumer group name determines where it starts — changing the group name creates a new group that starts from OffsetOldest
- For replay: create a new named consumer group with `OffsetOldest` rather than mutating the existing group's committed offsets

---

### Pitfall K4: Errors Channel Not Consumed Causes Silent Goroutine Leak

**Confidence:** HIGH (verified via Sarama docs)

**What goes wrong:** If `Consumer.Return.Errors = true` but no goroutine is reading from `consumer.Errors()`, the errors channel fills up and the Sarama internal goroutine blocks. Eventually the consumer stalls silently — it stops fetching messages but reports no explicit error.

**Why it happens:** Developers enable `Return.Errors` for observability but forget to drain the channel, or drain it only during the happy path.

**Prevention:**
```go
config.Consumer.Return.Errors = true

// Always drain in a dedicated goroutine, not inline with processing
go func() {
    for err := range consumerGroup.Errors() {
        log.Printf("consumer error: %v", err)
        metrics.ConsumerErrors.Inc()
    }
}()
```

---

### Pitfall K5: Consumer Group Rebalance During Graceful Shutdown Loses In-Flight Messages

**Confidence:** HIGH

**What goes wrong:** On SIGTERM, if you close the consumer before calling `session.Commit()` (or before MarkMessage has been flushed), in-flight messages that were being processed are re-queued and reprocessed by the next consumer. Combined with K2 above, this causes duplicates.

**Why it happens:** Graceful shutdown sequences that close Kafka before waiting for the processing pipeline to drain.

**Prevention:**
- Signal shutdown to the processing pipeline first (close the `done` channel / cancel the context)
- Wait for all in-flight messages to be processed (`wg.Wait()`)
- Then close the consumer group
- Use `defer cancel()` + `defer wg.Wait()` + `defer consumer.Close()` in the correct order (defers execute in LIFO)

---

### Pitfall K6: Partition Balance Strategy Mismatch Causes Uneven Load

**Confidence:** MEDIUM (Sarama docs verified; production behavior from operational knowledge)

**What goes wrong:** Sarama defaults to `RangeBalanceStrategy`. With a small number of partitions, one consumer gets all or most partitions. A second consumer instance sits idle, providing no redundancy benefit.

**Prevention:**
- Use `StickyBalanceStrategy` in production: minimizes partition movement during rebalances while distributing more evenly
- Ensure partition count >= consumer instance count

---

## Elasticsearch Pitfalls

### Pitfall E1: Bulk API Returns HTTP 200 Even When Documents Are Rejected (Silent Data Loss)

**Confidence:** HIGH

**What goes wrong:** The ES bulk API always returns HTTP 200 if the request was received. Individual document failures (mapping conflicts, field type violations, size limits) appear only in the response body as `"errors": true` with per-item `status` codes (400, 429, etc.). Code that only checks the HTTP status code silently drops failed documents.

**Why it happens:** Developers test with a clean index. Mapping violations don't appear until a new log format arrives in production. The HTTP 200 check passes and the document is never indexed.

**Consequences:** Log entries are permanently lost. Anomaly detection runs on an incomplete dataset. No error metric increments.

**Prevention:**
```go
// Always check the body, not just the HTTP status
var bulkResponse map[string]interface{}
json.Decode(resp.Body, &bulkResponse)
if bulkResponse["errors"].(bool) {
    // Iterate items and inspect each item's status
    // Items with status >= 400 were rejected
    // Log them; consider a dead-letter queue
}
```
- Parse every bulk response and alert on per-item errors
- Implement a dead-letter queue (a separate Kafka topic or local file) for rejected documents

---

### Pitfall E2: Mapping Explosion from Dynamic Mapping on Unstructured Logs

**Confidence:** HIGH

**What goes wrong:** Elasticsearch's dynamic mapping creates a new field mapping entry for every unique field name it encounters. Log lines with dynamic keys — user-agent strings parsed into sub-fields, JSON payloads with arbitrary keys, numeric IDs used as field names — can create thousands of field mappings per index. The cluster metadata grows unbounded, eventually causing OOM on the master node and cluster-wide outages.

**Why it happens:** Developers use default dynamic mapping because it "just works." Works fine on 100 log varieties; explodes at 10,000.

**Consequences:** Elasticsearch master node OOM. Entire cluster goes read-only or down. All indexing and querying stops.

**Prevention:**
- Define an explicit index template with `dynamic: false` or `dynamic: strict` before any index is created
- Map only the fields you know you need (timestamp, level, service, message, trace_id)
- For the unstructured "message" body, store as a single `text` field — do not allow nested dynamic mapping of parsed fields
- Set `index.mapping.total_fields.limit` to a sensible cap (e.g., 500) as a safety net

**This is an upfront architectural decision.** Fixing mapping explosion after data exists requires reindexing all historical data.

---

### Pitfall E3: Index Shard Count Fixed at Creation Time; Wrong Count Causes Permanent Perf Problems

**Confidence:** HIGH

**What goes wrong:** Shard count is set at index creation and cannot be changed (only split/shrink, which requires downtime). Too few shards: a single shard becomes a hot spot, limits parallel indexing, causes query slowdowns as it grows. Too many shards: each shard has overhead (~few MB heap), thousands of tiny indices kill the master node.

**Why it happens:** Developers use the default (1 primary shard in ES 7+) without thinking about time-based index rotation.

**Prevention:**
- For a log analytics system, use time-based indices: `logs-2026.03.21` with ILM (Index Lifecycle Management) to roll over daily or by size
- Use 1-2 primary shards per day index for <1k logs/sec — this scale does not need many shards
- Define an index template that enforces shard count before the first index is auto-created
- Use ILM to automatically delete indices older than retention window (prevents unbounded storage growth)

---

### Pitfall E4: refresh_interval Default Causes Query Lag; Setting to -1 Causes Invisible Data

**Confidence:** HIGH

**What goes wrong:** Default `refresh_interval = 1s` means indexed documents become searchable within 1 second. This is fine for most use cases but developers sometimes set `refresh_interval = -1` (disable automatic refresh) to maximize indexing throughput without realizing that queries will never see newly indexed documents until a manual refresh or the index is closed.

In the other direction: if you need real-time anomaly queries ("show me the last 30 seconds"), a 1-second refresh interval means the most recent second of data is always invisible to queries.

**Prevention:**
- Leave the default `refresh_interval = 1s` unless you have measured throughput problems
- For the anomaly detection pipeline at <1k logs/sec, the default is fine
- Do not set `refresh_interval = -1` in production unless you have explicit refresh calls after each bulk batch

---

### Pitfall E5: Connection Pool Exhaustion Under Backpressure

**Confidence:** HIGH

**What goes wrong:** The official `go-elasticsearch` client uses `http.DefaultTransport` unless configured otherwise. Under sustained load or Elasticsearch slowdowns, goroutines block waiting for connections. The default `MaxIdleConnsPerHost` is 2, causing a queue of goroutines waiting for a connection while ES recovers. In worst case: all goroutines in the pipeline are blocked on ES connections, Kafka consumer stops polling, session timeout fires, rebalance storm begins (see K2).

**Prevention:**
```go
transport := &http.Transport{
    MaxIdleConnsPerHost:   10,
    MaxConnsPerHost:       20,
    IdleConnTimeout:       90 * time.Second,
    TLSHandshakeTimeout:   10 * time.Second,
    ResponseHeaderTimeout: 30 * time.Second,
}
client, _ := elasticsearch.NewClient(elasticsearch.Config{
    Transport: transport,
})
```
- Always configure the HTTP transport explicitly
- Set a `ResponseHeaderTimeout` to bound how long ES calls can block

---

### Pitfall E6: Index Template Not Created Before First Document (Race on First Boot)

**Confidence:** HIGH

**What goes wrong:** If the application starts, receives its first Kafka message, and attempts to index before the index template has been applied, Elasticsearch creates the index with default settings. Dynamic mapping is active, shard count uses the cluster default, and the carefully designed mapping is never applied. This is a one-way door — the index must be deleted and recreated.

**Why it happens:** Application code creates the ES client and immediately starts consuming. Template creation is done "somewhere else" or assumed to already exist.

**Prevention:**
- On application startup, before starting the Kafka consumer, verify (and create if missing) the index template via the ES API
- Make startup fail-fast if the template cannot be applied
- Use `PUT _index_template/logs` with `create: false` (upsert) so it is idempotent

---

## Go Concurrency Pitfalls

### Pitfall G1: Goroutine Leak from Blocked Channel Sender (Pipeline Abandonment)

**Confidence:** HIGH (verified via go.dev/blog/pipelines)

**What goes wrong:** When a downstream consumer exits (due to an error or context cancellation) without draining the upstream channel, the upstream goroutine blocks forever on a channel send. It is not garbage collected. In a long-running service, each incident accumulates a new leaked goroutine, slowly growing memory until OOM.

**Why it happens:** Error handling that returns early from the consumer side without signaling upstream stages to stop.

**Prevention:**
```go
// Every pipeline stage must check done
go func() {
    defer close(out)
    for item := range in {
        select {
        case out <- process(item):
        case <-ctx.Done():
            return // Unblocks upstream sender
        }
    }
}()
```
- Use `context.Context` (not bare `done` channels) for cancellation — composes correctly with external signals
- Use `goleak` in tests to assert no goroutines outlive the test

---

### Pitfall G2: Race on Shared Anomaly State (Window Counters Without Mutex)

**Confidence:** HIGH (verified via go.dev race detector docs)

**What goes wrong:** Anomaly detection windows (e.g., "count errors in the last 60 seconds per service") require shared state — a map from service name to error counts with timestamps. If multiple goroutines process partitions concurrently and write to this shared map without synchronization, the race detector fires and in practice the counts are wrong, causing missed or phantom anomalies.

**Why it happens:** Maps in Go are explicitly not goroutine-safe. The race is invisible without `-race` flag because map corruption is non-deterministic.

**Prevention:**
- Protect window state with `sync.RWMutex` (read lock for threshold checks, write lock for increments)
- Or use `sync.Map` for simple key-value counters (lower contention for high read / low write ratio)
- Or partition detection state by Kafka partition so each goroutine owns its state exclusively (preferred — eliminates contention entirely)
- Run CI with `go test -race ./...`

---

### Pitfall G3: Context Cancellation Not Propagated to Elasticsearch Calls (Stuck Goroutines on Shutdown)

**Confidence:** HIGH (verified via go.dev context docs)

**What goes wrong:** The Kafka consumer context is cancelled on shutdown, but the goroutine making the Elasticsearch bulk request is using `context.Background()` instead of the propagated context. Shutdown blocks for the entire ES request timeout (potentially 30 seconds) before completing.

**Why it happens:** Copy-pasted ES call code that hardcodes `context.Background()`.

**Prevention:**
- Every function in the call chain accepts `ctx context.Context` as its first parameter
- All HTTP calls to Elasticsearch use the propagated context: `req = req.WithContext(ctx)`
- Verify with a shutdown integration test that measures time-to-exit

---

### Pitfall G4: Loop Variable Capture in Goroutine Closures

**Confidence:** HIGH (verified via go.dev race detector docs)

**What goes wrong:** Classic Go pitfall. Processing Kafka messages in a loop and launching goroutines that close over the loop variable:

```go
// BUG: All goroutines see the last value of msg
for _, msg := range messages {
    go func() {
        process(msg) // msg is captured by reference
    }()
}
```

**Prevention:**
```go
for _, msg := range messages {
    msg := msg // Shadow with local copy
    go func() {
        process(msg)
    }()
}
// Or pass as parameter:
go func(m *sarama.ConsumerMessage) { process(m) }(msg)
```
Note: In Go 1.22+, loop variable semantics changed — each iteration gets its own variable. If targeting Go 1.22+, this pitfall is eliminated by default.

---

### Pitfall G5: WaitGroup Counter Goes Negative (Panic on Shutdown)

**Confidence:** HIGH

**What goes wrong:** Calling `wg.Done()` more times than `wg.Add()` panics with "sync: negative WaitGroup counter." This happens when error handling paths call `Done()` after an early return that skips the expected work, or when goroutines are started without a corresponding `Add(1)` before the goroutine runs.

**Prevention:**
- Always call `wg.Add(1)` immediately before `go func()`, not inside the goroutine
- Use `defer wg.Done()` as the first statement inside the goroutine

---

### Pitfall G6: Unbounded Goroutine Fan-Out Under Message Spikes

**Confidence:** HIGH

**What goes wrong:** Processing each Kafka message in its own goroutine (`go processMessage(msg)`) with no limit. During a log spike (e.g., an application error storm producing thousands of messages per second), thousands of goroutines are spawned simultaneously. Each allocates stack space. Memory spikes. ES connection pool is slammed. The system degrades or crashes — exactly when anomaly detection is most needed.

**Prevention:**
- Use a worker pool with a fixed-size channel as a semaphore:
```go
sem := make(chan struct{}, workerCount) // e.g., 10 workers
for msg := range messages {
    sem <- struct{}{} // Blocks when pool is full
    go func(m *sarama.ConsumerMessage) {
        defer func() { <-sem }()
        processMessage(m)
    }(msg)
}
```
- Or use a pipeline with a buffered channel between stages and a fixed number of processing goroutines reading from it

---

## Anomaly Detection Pitfalls

### Pitfall A1: Cold Start Problem — First N Minutes Always Anomalous

**Confidence:** HIGH

**What goes wrong:** Rule-based detectors using sliding time windows ("error rate > X per minute") have no baseline when the service first starts. Every log in the first window is compared against a zero baseline, producing false positive anomalies on startup. If alerts are sent via email, the first minute of operation floods inboxes and trains operators to ignore alerts.

**Prevention:**
- Implement a warm-up period: suppress alerting for the first N minutes (e.g., 2x the window size) after startup
- Mark anomalies generated during warm-up as `confidence: low` rather than suppressing them completely — they are still stored in ES for review
- Use a configurable `WarmupPeriod` flag to tune per deployment

---

### Pitfall A2: Threshold Drift — Static Thresholds Become Stale

**Confidence:** HIGH

**What goes wrong:** A rule "alert if error rate > 50/min" is calibrated for current traffic. Six months later, traffic doubles. Normal operation now produces 80 errors/min. The threshold fires continuously and operators turn off the alert. The detector is now silent.

**Prevention:**
- Document that all thresholds are in a configuration file (not hardcoded) and must be reviewed when traffic patterns change significantly
- For v1 (rule-based only): add a `last_reviewed` timestamp to each rule definition and a startup warning if any rule is older than 90 days
- Log the baseline metrics alongside anomaly records so reviewers can see how far from normal each event was

---

### Pitfall A3: Counting Raw Log Volume Instead of Rate (Window Boundary Artifacts)

**Confidence:** HIGH

**What goes wrong:** A "time window" based on wall clock intervals has a boundary artifact: if a burst of errors spans a window boundary (e.g., 30 errors at 59:59 and 30 errors at 00:01), each window sees 30 errors — below threshold. A burst of 60 errors that starts at 00:00 triggers an alert. The detector's behavior depends on when bursts happen relative to arbitrary window boundaries.

**Prevention:**
- Use a sliding window (circular buffer of events with timestamps) rather than fixed tumbling windows
- For each incoming log, count events in the last N seconds by looking backward in the buffer, not by which bucket the event falls in
- This is more complex to implement but eliminates boundary artifacts

---

### Pitfall A4: Alert Storms — Repeated Alerting for the Same Ongoing Anomaly

**Confidence:** HIGH

**What goes wrong:** An anomaly condition persists for 10 minutes. The detector fires an alert every 30 seconds (once per processed batch). The oncall receives 20 identical emails. Alert fatigue sets in; future alerts are ignored.

**Prevention:**
- Implement alert deduplication: once an anomaly for (service, type) is detected, do not re-alert until the condition has cleared and re-triggered
- Track anomaly state in memory: `map[anomalyKey]time.Time` stores when each anomaly was last alerted
- Configurable `RenotifyInterval`: re-alert only after N minutes of sustained condition (e.g., 15 minutes)
- Persist ongoing anomaly state to Elasticsearch so restarts don't re-send storm

---

### Pitfall A5: Security Event Detector Produces Excessive False Positives from Monitoring/Healthchecks

**Confidence:** HIGH

**What goes wrong:** "Auth failure" detectors fire on load balancer health checks, monitoring probes, and internal service calls that use unauthenticated endpoints. A Kubernetes readiness probe hitting `/health` every 5 seconds looks like a repeated auth failure from an external IP.

**Prevention:**
- Add an allowlist of source IPs / user-agents that are excluded from security rule evaluation
- Filter out log entries where `path` matches known health check endpoints before they enter the anomaly detection stage
- Make the allowlist configurable — do not hardcode infrastructure IP ranges

---

### Pitfall A6: Anomaly Detector Not Partition-Aware (State Spread Across Instances)

**Confidence:** HIGH

**What goes wrong:** With multiple consumer group instances, each instance handles a subset of Kafka partitions. If anomaly detection state (error counts, windows) is in-process memory, each instance only sees a fraction of the traffic. An error rate of 100/min split across 4 instances looks like 25/min to each — below every threshold. Real anomalies are silently missed.

**Why it happens:** The system works correctly in development with one consumer instance.

**Prevention:**
- For v1 single-instance: document this as a known constraint
- Design the detector interface to accept a "total window count" that could later be aggregated from multiple sources
- Long-term: use a centralized state store (Redis, or a dedicated Kafka topic for aggregation) if multi-instance is needed
- This is an architectural decision that is expensive to change after the fact — design the interface correctly even if the single-instance implementation is simple

---

## Operational Pitfalls

### Pitfall O1: No Dead-Letter Queue — Unprocessable Messages Block the Pipeline

**Confidence:** HIGH

**What goes wrong:** A malformed log message (invalid JSON, unexpected encoding, message too large) causes the processing goroutine to return an error. Without a dead-letter queue, three outcomes occur: (a) the message is skipped and the offset committed — silent data loss; (b) the message is retried forever — pipeline stalls; (c) the consumer panics — restarts and reprocesses from the last committed offset.

**Prevention:**
- Define explicit "what happens on parse failure" behavior: always route failures to a dead-letter Kafka topic (`logs.dlq`)
- Never commit the offset of a message that caused a panic — use `recover()` in the message handler and route to DLQ
- Alert on DLQ depth — a non-zero DLQ is a signal that a new log format needs a parser update

---

### Pitfall O2: Elasticsearch Disk Full Silently Stops Indexing

**Confidence:** HIGH

**What goes wrong:** When Elasticsearch disk usage exceeds the high watermark (default 90%), ES sets all indices to read-only. Bulk indexing calls return HTTP 429 or 403 with `cluster_block_exception`. If the application treats this as a transient error and retries indefinitely, the Kafka consumer falls behind. Consumer lag grows. Eventually session timeout fires (K2). In worst case, the pipeline deadlocks silently.

**Prevention:**
- Implement ILM policies that delete old indices before the disk fills (retention window)
- Monitor ES disk usage and alert before it reaches 85%
- Handle `cluster_block_exception` (HTTP 403) as a distinct error case — not a retry, but an alert requiring operator intervention
- Use a circuit breaker: if ES fails for more than N consecutive seconds, stop consuming from Kafka (preserve offset) rather than losing messages

---

### Pitfall O3: Log Volume Spikes Saturate Bulk Buffer, Causing Memory Pressure

**Confidence:** HIGH

**What goes wrong:** The bulk indexer buffers messages to send to Elasticsearch in batches. During an incident (which is exactly when you need accurate anomaly detection), log volume spikes 10-100x. The in-memory bulk buffer grows unbounded until the Go process OOMs.

**Prevention:**
- Set a hard cap on the bulk buffer size (number of documents OR bytes, whichever is smaller)
- When the buffer is full, apply backpressure: block the Kafka consumer goroutine until the batch is flushed
- This is preferable to dropping messages — it increases Kafka consumer lag but preserves data

---

### Pitfall O4: SMTP Failures Block the Processing Pipeline (Synchronous Alerting)

**Confidence:** HIGH

**What goes wrong:** Email alerting via SMTP is synchronous in a naive implementation. When the SMTP server is slow or unavailable, the processing goroutine blocks on the email send. This blocks the Kafka consumer from processing new messages. A transient email outage causes the entire pipeline to stall.

**Prevention:**
- All alerting must be asynchronous: the detection pipeline pushes alerts to a buffered channel; a separate goroutine drains the channel and sends emails
- The buffered channel acts as a local queue — if SMTP is down, alerts queue (up to buffer size)
- If the buffer is full, log the dropped alert (accept that a severe outage may lose alerts in exchange for pipeline liveness)
- Set a connection timeout and deadline on all SMTP calls

---

### Pitfall O5: Structured Logging Fields Used as Dynamic ES Fields (Mapping Explosion Vector)

**Confidence:** HIGH

**What goes wrong:** Application logs include arbitrary key-value fields in JSON (`"user_id": "abc123"`, `"request_id": "xyz789"`, `"trace_context": {...}`). If these are indexed dynamically in ES, mapping explosion begins. The trace_context field alone, if it contains nested arbitrary keys, can generate hundreds of field mappings per index.

**Prevention:**
- In the log normalization stage, extract a fixed set of known fields into top-level mapped fields
- Store all remaining key-value pairs as a single `keyword` or `text` field (e.g., `extra_fields_json`) — this is indexed as a blob and is searchable via match queries, but does not create individual field mappings
- Never let arbitrary application fields become ES document fields

---

### Pitfall O6: No Idempotent Document IDs — Reprocessing Creates Duplicates in Elasticsearch

**Confidence:** HIGH

**What goes wrong:** On consumer restart or rebalance, messages that were processed but not committed are reprocessed. If the ES bulk indexer uses auto-generated document IDs, the same log message is indexed twice. Anomaly counts are inflated. Query results show duplicate records.

**Prevention:**
- Generate deterministic document IDs from message content: `sha256(topic + partition + offset)` or use a field from the log itself (e.g., trace_id + timestamp)
- Use `PUT /index/_doc/{id}` (upsert) semantics in bulk requests rather than POST (auto-id)
- This turns reprocessing from "creates duplicates" into "idempotent upsert" — safe to replay

---

## Mitigation Strategies Summary

| Pitfall | Severity | Mitigation |
|---------|----------|------------|
| K1: Auto-commit before processing | CRITICAL | Disable auto-commit; use MarkMessage after processing |
| K2: Session timeout < processing time | CRITICAL | Set Session.Timeout > max processing time; bounded ES calls |
| K3: OffsetNewest skips backlog | HIGH | Explicit OffsetOldest for first deploy; document group naming |
| K4: Errors channel not drained | HIGH | Always drain errors channel in dedicated goroutine |
| K5: Shutdown before pipeline drains | HIGH | LIFO defer order: cancel → wait → close consumer |
| K6: Partition imbalance | MEDIUM | Use StickyBalanceStrategy |
| E1: Bulk API HTTP 200 on failure | CRITICAL | Parse every bulk response body; implement DLQ |
| E2: Mapping explosion | CRITICAL | Explicit template with dynamic:false before first index |
| E3: Wrong shard count at creation | HIGH | Time-based indices with ILM; template before first doc |
| E4: refresh_interval misconfiguration | MEDIUM | Leave default unless measured; never set to -1 in production |
| E5: Connection pool exhaustion | HIGH | Explicit http.Transport with bounded pool and timeout |
| E6: Template not created before first doc | CRITICAL | Startup verification with fail-fast |
| G1: Goroutine leak from blocked sender | HIGH | ctx.Done() in all pipeline select statements |
| G2: Race on shared anomaly state | CRITICAL | Mutex protection; or partition-local state |
| G3: Context not propagated to ES calls | HIGH | ctx as first param everywhere; no context.Background() in pipeline |
| G4: Loop variable capture | HIGH | Shadow variable or pass as param; Go 1.22+ eliminates this |
| G5: WaitGroup counter goes negative | MEDIUM | wg.Add(1) before goroutine; defer wg.Done() first line |
| G6: Unbounded goroutine fan-out | HIGH | Fixed-size worker pool with semaphore channel |
| A1: Cold start false positives | HIGH | Warm-up period before alerting |
| A2: Threshold drift | MEDIUM | Configurable rules with reviewed timestamps |
| A3: Window boundary artifacts | MEDIUM | Sliding windows not tumbling windows |
| A4: Alert storms | HIGH | Deduplication with renotify interval |
| A5: False positives from healthchecks | HIGH | Allowlist for known monitoring sources |
| A6: State not partition-aware | CRITICAL (multi-instance) | Document single-instance constraint; design interface for future aggregation |
| O1: No DLQ for unprocessable messages | HIGH | DLQ topic; never commit offset of panicked message |
| O2: ES disk full stops indexing | HIGH | ILM retention; circuit breaker on cluster_block_exception |
| O3: Bulk buffer OOM during spike | HIGH | Hard cap on buffer; backpressure to consumer |
| O4: Synchronous SMTP blocks pipeline | HIGH | Async alerting via buffered channel + separate goroutine |
| O5: Structured log fields as dynamic ES fields | CRITICAL | Normalize to fixed fields; blob remaining as keyword |
| O6: No idempotent document IDs | HIGH | Deterministic ID from partition+offset; upsert semantics |

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Kafka consumer setup | K1, K3, K4 | Set explicit offset config; drain errors; disable auto-commit |
| Log parsing / normalization | O5, E2 | Fixed field schema before first ES write |
| Elasticsearch integration | E6, E1, E2 | Template first; parse bulk response; no dynamic mapping |
| Anomaly detection implementation | A1, A3, A6, G2 | Sliding windows; warm-up; single-instance documented; mutex |
| Alerting integration | A4, O4 | Async send; deduplication |
| Graceful shutdown | K5, G1, G3 | LIFO defer order; ctx propagation; drain before close |
| Testing | G2, G4 | `-race` flag required in CI; goleak for goroutine cleanup |
| Production deployment | O2, O3, K2 | ILM policies; buffer caps; session timeout tuning |

---

## Sources

- Sarama (IBM/sarama) pkg.go.dev documentation — verified via WebFetch (HIGH confidence)
- Go pipeline patterns: go.dev/blog/pipelines — verified via WebFetch (HIGH confidence)
- Go context patterns: go.dev/blog/context — verified via WebFetch (HIGH confidence)
- Go race detector documentation: go.dev/doc/articles/race_detector — verified via WebFetch (HIGH confidence)
- Elasticsearch bulk API behavior, mapping explosion, ILM — training data (HIGH confidence; these are stable, well-documented behaviors)
- Kafka consumer configuration semantics (session.timeout, auto.offset.reset, auto.commit) — training data (HIGH confidence; core protocol behaviors)
- Anomaly detection operational patterns — training data (MEDIUM confidence; standard industry knowledge)
