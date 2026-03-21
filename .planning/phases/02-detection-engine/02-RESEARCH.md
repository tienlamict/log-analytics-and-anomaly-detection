# Phase 2: Detection Engine - Research

**Researched:** 2026-03-21
**Domain:** Go sliding-window anomaly detection, rule engine patterns, Viper YAML config, per-rule cooldown, timer-based heartbeat goroutines
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DETECT-01 | Detection engine evaluates incoming log entries against registered rules using a sliding time window | `DetectorEngine` registry pattern — runs each `Detector.Evaluate` in sequence; sliding-window counter via circular timestamp buffer |
| DETECT-02 | Rule: Error Rate Spike — fires when error-level log count for a service exceeds threshold within a window | `ErrorRateRule`: per-service circular timestamp deque; count entries within `[now-window, now]`; fire when count > threshold |
| DETECT-03 | Rule: Latency Threshold Breach — fires when `duration_ms` field exceeds threshold for >N% of requests in window | `LatencyThresholdRule`: track (total_requests, breach_count) per service in window; skip entries missing `duration_ms` field |
| DETECT-04 | Rule: Repeated Failure — fires when same error fingerprint repeats >N times in window | `RepeatedFailureRule`: normalise message via regex (strip numerics/UUIDs); count per-fingerprint in window |
| DETECT-05 | Rule: Auth Failure Burst — fires when auth failure events for a single IP or user exceed threshold in window | `AuthFailureBurstRule`: key = IP or username extracted from `Fields`; per-key circular timestamp buffer |
| DETECT-06 | Rule: Off-Hours Access — fires when access to configured sensitive endpoints occurs outside business hours | `OffHoursAccessRule`: check `Fields["path"]` against allowlist; check `entry.Timestamp` against configured hour range; stateless — no window counter needed |
| DETECT-07 | Rule: Service Silence — fires when a known-active service emits no logs for longer than configurable threshold | `ServiceSilenceRule`: timer-based heartbeat goroutine; `last_seen map[service]time.Time`; cold-start guard: require N prior logs before entering active tracking |
| DETECT-08 | Alert deduplication — per-rule cooldown window prevents repeated alerts for same ongoing condition | `map[cooldownKey]time.Time` where `cooldownKey = (ruleID, service)`; suppress fire if `now - lastFired < cooldown`; stored separately from window counters |
| DETECT-09 | Detection rules and thresholds are configurable via YAML/JSON config file | Extend `internal/config.Config` with `Detection DetectionConfig`; Viper `mapstructure` tags; no code changes needed to tune thresholds |
| DETECT-10 | Window state is cleaned up for inactive services to prevent unbounded memory growth | Background eviction goroutine in `DetectorEngine`; removes per-service state when `last_seen > eviction_ttl`; runs on `ticker` interval |
</phase_requirements>

---

## Summary

Phase 2 implements a rule-based anomaly detection engine in `internal/detection`. The engine receives `domain.LogEntry` values from the Phase 1 pipeline and evaluates each entry against seven `domain.Detector` implementations sequentially. All window state lives under single-goroutine ownership inside each rule — the engine calls rules from one goroutine, eliminating locking complexity. A background eviction goroutine (DETECT-10) and a heartbeat goroutine for `ServiceSilenceRule` (DETECT-07) are the only exceptions; they communicate state changes through the same owner goroutine via channels.

The critical implementation concerns are: (1) using a circular/sliding window (not tumbling/fixed-boundary windows) to avoid false-positive spikes at boundary crossings, (2) the warm-up guard that suppresses all alerts for 2× the longest window duration after startup, (3) the cold-start guard in `ServiceSilenceRule` to avoid firing on services seen only once, (4) correct separation of cooldown state from window counters so deduplication does not corrupt event counts, and (5) Viper config loading that maps the full detection config tree with correct `mapstructure` tags.

No new library dependencies are required for Phase 2 beyond those already installed in Phase 1. Viper (already in `go.mod`) handles all YAML config loading. All detection logic uses only stdlib (`time`, `regexp`, `sync`, `container/ring` or manual slice-based circular buffer). The `domain.Detector` interface is already defined in `internal/domain/interfaces.go`.

**Primary recommendation:** Implement `DetectorEngine` with a `[]Detector` registry, single-goroutine dispatch, channel-based anomaly output, and a background eviction ticker. Store cooldown state in the engine layer (not inside individual rules) so the engine can consult it before forwarding an anomaly to callers.

---

## Standard Stack

### Core (Phase 2 — no new external dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/viper` | v1.21.0 (already in go.mod) | Load detection config from `config.yaml` | Already used for Kafka config; extend same `Config` struct |
| `go.uber.org/zap` | v1.27.0 (already in go.mod) | Structured logging in detection engine | Already used; pass `*zap.Logger` via constructor injection |
| `github.com/prometheus/client_golang` | v1.23.2 (already in go.mod) | `AnomaliesDetectedTotal` counter per rule | Already registered in `internal/metrics/metrics.go` |
| `github.com/stretchr/testify` | v1.11.1 (already in go.mod) | Table-driven test assertions | Already used in Phase 1 |
| `go.uber.org/goleak` | v1.3.0 (already in go.mod) | Goroutine leak detection | Already used; `VerifyTestMain` in every test package |

### Supporting (stdlib only — no `go get` required)

| Package | Purpose | When to Use |
|---------|---------|-------------|
| `regexp` | Message fingerprinting for `RepeatedFailureRule` | Strip numerics, UUIDs, hex strings from error messages to produce a stable fingerprint |
| `time` | All window arithmetic, cooldown checks, heartbeat ticker | Universal; `time.Since`, `time.NewTicker`, `time.After` |
| `sync` | `sync.Mutex` only if goroutine boundaries must be crossed | Avoid: prefer single-goroutine ownership. Use only in `ServiceSilenceRule` heartbeat → engine boundary |
| `container/ring` | Alternative circular buffer for timestamp windows | Usable but a plain `[]time.Time` slice with head/tail indices is simpler and equally correct |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Plain `[]time.Time` circular buffer | `container/ring` | `container/ring` has an opaque `Value interface{}` element type requiring casts; plain slice is clearer and avoids allocations per entry |
| Single-goroutine window ownership | `sync.RWMutex` per rule | Mutex approach requires careful lock granularity; single-goroutine ownership eliminates the class of bug entirely |
| Separate `DetectorEngine` goroutine | Inline evaluation in pipeline worker | Inline evaluation couples detection to the parse goroutine; a dedicated engine goroutine allows future buffering and back-pressure |

**Installation:** No new packages required. All dependencies are present in `go.mod`.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── detection/
│   ├── engine.go            # DetectorEngine: registry, dispatch, eviction goroutine, cooldown
│   ├── engine_test.go       # Engine integration: registry, cooldown, warm-up, eviction
│   ├── window.go            # slidingWindow helper: circular []time.Time buffer + evict
│   ├── rules/
│   │   ├── error_rate.go         # ErrorRateRule (DETECT-02)
│   │   ├── latency.go            # LatencyThresholdRule (DETECT-03)
│   │   ├── repeated_failure.go   # RepeatedFailureRule (DETECT-04)
│   │   ├── auth_burst.go         # AuthFailureBurstRule (DETECT-05)
│   │   ├── off_hours.go          # OffHoursAccessRule (DETECT-06)
│   │   ├── service_silence.go    # ServiceSilenceRule (DETECT-07)
│   │   └── rules_test.go         # Table-driven tests for all six event-driven rules
│   └── silence_test.go      # ServiceSilenceRule timer tests (separate file — uses fake clock)
internal/
├── config/
│   └── config.go            # Extend with DetectionConfig (mapstructure tags)
```

The `internal/detection/rules/` sub-package keeps individual rule files small and independently testable. The `engine.go` file imports the rules package and composes all seven detectors.

### Pattern 1: DetectorEngine Core

```go
// Source: domain.Detector interface (internal/domain/interfaces.go, already implemented)

type cooldownKey struct {
    ruleID  string
    service string
}

type DetectorEngine struct {
    detectors  []domain.Detector
    cooldowns  map[cooldownKey]time.Time
    cooldownDur time.Duration
    warmupUntil time.Time
    logger     *zap.Logger
    out        chan domain.Anomaly
    evictStop  chan struct{}
}

func NewDetectorEngine(detectors []domain.Detector, cfg DetectionConfig, logger *zap.Logger) *DetectorEngine {
    maxWindow := maxWindowDuration(cfg)  // find longest configured window
    e := &DetectorEngine{
        detectors:   detectors,
        cooldowns:   make(map[cooldownKey]time.Time),
        cooldownDur: cfg.CooldownDuration,
        warmupUntil: time.Now().Add(2 * maxWindow),  // Pitfall A1 guard
        logger:      logger,
        out:         make(chan domain.Anomaly, 256),
        evictStop:   make(chan struct{}),
    }
    go e.evictLoop(cfg.EvictionInterval)
    return e
}

func (e *DetectorEngine) Evaluate(entry domain.LogEntry) {
    if time.Now().Before(e.warmupUntil) {
        return  // warm-up period: suppress all alerts (DETECT-08 / Pitfall A1)
    }
    for _, d := range e.detectors {
        anomaly, fired := d.Evaluate(entry)
        if !fired {
            continue
        }
        key := cooldownKey{ruleID: anomaly.RuleID, service: anomaly.Service}
        if last, ok := e.cooldowns[key]; ok && time.Since(last) < e.cooldownDur {
            continue  // within cooldown — suppress (DETECT-08)
        }
        e.cooldowns[key] = time.Now()
        metrics.AnomaliesDetectedTotal.WithLabelValues(anomaly.RuleID).Inc()
        select {
        case e.out <- anomaly:
        default:
            e.logger.Warn("anomaly channel full, dropping alert", zap.String("rule", anomaly.RuleID))
        }
    }
}

func (e *DetectorEngine) Anomalies() <-chan domain.Anomaly { return e.out }
```

**Why single-goroutine ownership:** `Evaluate` is called from the pipeline worker goroutine. All window state inside each rule is owned by that same calling goroutine. No locks needed on the hot path.

### Pattern 2: Sliding Window Helper

```go
// internal/detection/window.go

// slidingWindow maintains a circular buffer of event timestamps.
// All methods must be called from a single goroutine.
type slidingWindow struct {
    timestamps []time.Time
    head       int
    count      int
}

func newSlidingWindow(capacity int) *slidingWindow {
    return &slidingWindow{timestamps: make([]time.Time, capacity)}
}

// Add records a new event timestamp, evicting the oldest if at capacity.
func (w *slidingWindow) Add(t time.Time) {
    w.timestamps[w.head%len(w.timestamps)] = t
    w.head++
    if w.count < len(w.timestamps) {
        w.count++
    }
}

// CountWithin returns how many events fall within [now-duration, now].
func (w *slidingWindow) CountWithin(duration time.Duration) int {
    cutoff := time.Now().Add(-duration)
    n := 0
    for i := 0; i < w.count; i++ {
        idx := (w.head - 1 - i + len(w.timestamps)) % len(w.timestamps)
        if w.timestamps[idx].After(cutoff) {
            n++
        }
    }
    return n
}

// Evict removes timestamps older than `olderThan` by compacting the buffer.
func (w *slidingWindow) Evict(olderThan time.Duration) {
    cutoff := time.Now().Add(-olderThan)
    newTs := w.timestamps[:0]
    for i := 0; i < w.count; i++ {
        idx := (w.head - w.count + i + len(w.timestamps)) % len(w.timestamps)
        if w.timestamps[idx].After(cutoff) {
            newTs = append(newTs, w.timestamps[idx])
        }
    }
    copy(w.timestamps, newTs)
    w.count = len(newTs)
    w.head = w.count
}
```

**Why this over `container/ring`:** No per-element interface{} allocation, no type assertions, direct slice indexing, easier to unit test.

### Pattern 3: Error Rate Rule (DETECT-02)

```go
// internal/detection/rules/error_rate.go

type ErrorRateRule struct {
    windows   map[string]*slidingWindow  // keyed by service
    threshold int
    window    time.Duration
    capacity  int
}

func (r *ErrorRateRule) Name() string { return "error_rate_spike" }

func (r *ErrorRateRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
    if entry.Level != "error" {
        return domain.Anomaly{}, false
    }
    svc := entry.Service
    if _, ok := r.windows[svc]; !ok {
        r.windows[svc] = newSlidingWindow(r.capacity)
    }
    w := r.windows[svc]
    w.Add(entry.Timestamp)
    if w.CountWithin(r.window) >= r.threshold {
        return domain.Anomaly{
            ID:          uuid.NewString(),
            RuleID:      r.Name(),
            Severity:    "high",
            Service:     svc,
            Description: fmt.Sprintf("error rate spike: %d errors in %s", w.CountWithin(r.window), r.window),
            Evidence:    []domain.LogEntry{entry},
            DetectedAt:  time.Now(),
        }, true
    }
    return domain.Anomaly{}, false
}

func (r *ErrorRateRule) Reset() { r.windows = make(map[string]*slidingWindow) }
```

**Key detail:** `Evaluate` uses `entry.Timestamp` (log event time), not `time.Now()` (ingest time). This ensures tests injecting synthetic timestamps work correctly.

### Pattern 4: Repeated Failure Fingerprinting (DETECT-04)

```go
// internal/detection/rules/repeated_failure.go

// Compile once at init time — avoids per-call allocation.
var fingerprintNoise = regexp.MustCompile(`\b[\da-f]{4,}\b|\d+`)

func fingerprint(msg string) string {
    return fingerprintNoise.ReplaceAllLiteralString(strings.ToLower(msg), "X")
}
```

**Why regex normalisation:** Error messages like `"connection to 10.0.0.1:5432 failed after 3 retries"` and `"connection to 10.0.0.2:5432 failed after 7 retries"` are the same class of failure. The regex strips numeric literals and short hex strings to produce a stable fingerprint `"connection to X:X failed after X retries"`.

### Pattern 5: ServiceSilenceRule (DETECT-07)

```go
// internal/detection/rules/service_silence.go

type ServiceSilenceRule struct {
    lastSeen    map[string]time.Time  // service -> last log timestamp
    seenCount   map[string]int        // cold-start guard: N logs before active tracking
    minCount    int                   // minimum prior logs before active tracking
    threshold   time.Duration         // silence threshold
    checkCh     chan silenceCheck      // heartbeat goroutine sends checks here
    anomalyCh   chan domain.Anomaly    // detected silences sent here
    stopCh      chan struct{}
}

// ServiceSilenceRule does NOT implement domain.Detector directly.
// Instead, it has two methods:
//   Observe(entry LogEntry)    — called from Evaluate goroutine to update last_seen
//   Anomalies() <-chan Anomaly — heartbeat goroutine fires on silence threshold

// The DetectorEngine calls Observe instead of Evaluate for this rule,
// and drains its Anomalies() channel alongside the engine's own out channel.
```

**Why special treatment for ServiceSilenceRule:** All other rules are event-driven — they fire when a log entry arrives. `ServiceSilenceRule` fires on ABSENCE of events, so it requires a separate timer goroutine. The rule cannot implement `domain.Detector.Evaluate` the same way. Two design options exist:

1. **Adapter option:** Implement `Evaluate` as a no-op that always returns `false`; add an `Observe` method called directly by the engine; expose a separate `Anomalies()` channel. The engine merges this channel with its own anomaly output. This keeps the `domain.Detector` interface unchanged.

2. **Interface extension option:** Add an optional `Observer` interface alongside `Detector`; the engine type-asserts at registration time. This is slightly cleaner but adds interface complexity.

**Recommended:** Adapter option. The engine calls a `d.Evaluate(entry)` for all rules (ServiceSilenceRule's `Evaluate` only updates `last_seen` and returns false), and the engine separately starts the heartbeat goroutine that drains `silenceRule.SilenceAnomalies()`. This requires zero interface changes.

```go
// In DetectorEngine.Evaluate:
for _, d := range e.detectors {
    anomaly, fired := d.Evaluate(entry)  // ServiceSilenceRule.Evaluate updates last_seen, returns false
    if fired {
        // ... cooldown check and emit
    }
}

// Heartbeat goroutine (started in NewDetectorEngine):
func (e *DetectorEngine) silenceWatcher(sr *ServiceSilenceRule) {
    ticker := time.NewTicker(sr.checkInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            if anomaly, fired := sr.CheckSilence(); fired {
                // apply cooldown, emit to e.out
            }
        case <-e.evictStop:
            return
        }
    }
}
```

### Pattern 6: YAML Config Extension

```go
// internal/config/config.go — extend existing Config struct

type Config struct {
    Kafka     KafkaConfig     `mapstructure:"kafka"`
    Metrics   MetricsConfig   `mapstructure:"metrics"`
    Log       LogConfig       `mapstructure:"log"`
    Detection DetectionConfig `mapstructure:"detection"`  // NEW
}

type DetectionConfig struct {
    WindowDuration   time.Duration              `mapstructure:"window_duration"`
    EvictionInterval time.Duration              `mapstructure:"eviction_interval"`
    CooldownDuration time.Duration              `mapstructure:"cooldown_duration"`
    WarmupMultiplier int                        `mapstructure:"warmup_multiplier"` // default 2
    Rules            DetectionRulesConfig       `mapstructure:"rules"`
}

type DetectionRulesConfig struct {
    ErrorRate       ErrorRateConfig       `mapstructure:"error_rate"`
    Latency         LatencyConfig         `mapstructure:"latency"`
    RepeatedFailure RepeatedFailureConfig `mapstructure:"repeated_failure"`
    AuthBurst       AuthBurstConfig       `mapstructure:"auth_burst"`
    OffHours        OffHoursConfig        `mapstructure:"off_hours"`
    ServiceSilence  ServiceSilenceConfig  `mapstructure:"service_silence"`
}

type ErrorRateConfig struct {
    Enabled   bool          `mapstructure:"enabled"`
    Threshold int           `mapstructure:"threshold"`  // e.g. 10 errors
    Window    time.Duration `mapstructure:"window"`     // e.g. 5m
    Severity  string        `mapstructure:"severity"`   // "high"
}

// ... similar structs for each rule

type OffHoursConfig struct {
    Enabled            bool     `mapstructure:"enabled"`
    BusinessHoursStart int      `mapstructure:"business_hours_start"` // 9 (9am)
    BusinessHoursEnd   int      `mapstructure:"business_hours_end"`   // 17 (5pm)
    SensitivePaths     []string `mapstructure:"sensitive_paths"`
    Severity           string   `mapstructure:"severity"`
}

type ServiceSilenceConfig struct {
    Enabled       bool          `mapstructure:"enabled"`
    SilenceAfter  time.Duration `mapstructure:"silence_after"` // e.g. 5m
    CheckInterval time.Duration `mapstructure:"check_interval"` // e.g. 30s
    MinLogCount   int           `mapstructure:"min_log_count"`  // cold-start guard: e.g. 5
    Severity      string        `mapstructure:"severity"`
}
```

**Viper `time.Duration` mapping:** Viper reads duration strings (e.g. `"5m"`, `"30s"`) via `mapstructure` when using `go-viper/mapstructure/v2` with `DecodeHook: mapstructure.StringToTimeDurationHookFunc()`. This is already the default Viper hook, so `time.Duration` fields unmarshal correctly without custom code.

### Pattern 7: Background Eviction Goroutine (DETECT-10)

```go
// internal/detection/engine.go

func (e *DetectorEngine) evictLoop(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            for _, d := range e.detectors {
                d.Reset()  // each rule evicts stale state internally
            }
            // Also evict stale cooldown entries
            now := time.Now()
            for k, last := range e.cooldowns {
                if now.Sub(last) > e.cooldownDur*2 {
                    delete(e.cooldowns, k)
                }
            }
        case <-e.evictStop:
            return
        }
    }
}

func (e *DetectorEngine) Stop() {
    close(e.evictStop)
}
```

**Note:** `Reset()` is defined on `domain.Detector`. Each rule's `Reset()` should evict window entries older than its configured window duration — not clear all state (clearing all state would cause a cold-start spike after every eviction cycle, Pitfall A2).

### Anti-Patterns to Avoid

- **Fixed tumbling windows (Pitfall A3):** A window that resets every N minutes has a sharp boundary artefact — 9 errors in minute 4:59 and 9 errors in minute 5:01 never fire despite 18 errors in 2 seconds. Use a sliding window that asks "how many events in the last N minutes from now?"
- **Startup spike false positives (Pitfall A1):** At startup, the window is empty and fills quickly. The first N entries can immediately satisfy a threshold that represents normal startup chatter, not an anomaly. Guard with `warmupUntil = startTime + 2*maxWindow`.
- **Reset() clears all state (Pitfall A2):** Calling `Reset()` (e.g. from the eviction goroutine) should evict OLD entries, not wipe the entire window. Wiping causes a brief blind spot and potential false spike after the next few entries arrive.
- **Storing cooldown state inside rules (architecture flaw):** Cooldown is a cross-cutting concern — if two different callers call `Evaluate` (e.g. in tests), shared cooldown inside the rule means test 2 is affected by test 1's state. Keep cooldown in the engine layer, not inside individual rules.
- **Lock contention on per-rule maps (Pitfall G2):** Taking a `sync.Mutex` per `Evaluate` call in each rule on every log entry creates hot-path lock contention at high throughput. Single-goroutine ownership (all rules called from the same goroutine) eliminates this entirely.
- **ServiceSilenceRule firing on first log (cold-start):** A service emits exactly one log on startup, then the deployment fails silently. The silence rule would fire almost immediately. The `minLogCount` guard ensures a service must emit N logs before it enters active silence tracking.
- **Calling `time.Now()` for event timestamps in rules (test anti-pattern):** Rules that use `time.Now()` internally for window logic cannot be tested with injected synthetic timestamps. Always use `entry.Timestamp` for the event time in window operations. This allows deterministic unit tests.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YAML config with env override | Custom `os.Getenv` chains | Viper `AutomaticEnv()` + `mapstructure` | Already in use; handles env precedence, defaults, file + env merge |
| String duration parsing (`"5m"`) | `strconv.ParseFloat` + switch | `time.ParseDuration` (stdlib) or Viper's built-in `StringToTimeDurationHookFunc` | stdlib covers all duration units including `h`, `m`, `s`, `ms`, `µs`, `ns` |
| Message deduplication fingerprinting | Levenshtein distance, ML clustering | `regexp.MustCompile` + `ReplaceAllLiteralString` | Simple regex strips dynamic tokens; good enough for v1; no external dep |
| Prometheus counter increment | Manual `atomic.AddInt64` | `metrics.AnomaliesDetectedTotal.WithLabelValues(ruleID).Inc()` | Already registered in `internal/metrics/metrics.go` at `init()` |
| Circular buffer | `container/ring` | Plain `[]time.Time` with head/count indices | Simpler, no type assertions, measurably easier to unit test |
| Timer-based goroutine lifecycle | `time.Sleep` loop | `time.NewTicker` + `select { case <-stopCh }` | Ticker is precise; sleep loop drifts; select enables clean shutdown |

**Key insight:** All the "hard" problems in this phase (config loading, metrics, test isolation) are already solved by libraries used in Phase 1. The novel work is purely algorithmic: sliding-window logic, fingerprinting, and the silence-detection timer pattern.

---

## Common Pitfalls

### Pitfall A1: Startup Spike False Positives
**What goes wrong:** The detection engine starts processing a burst of catch-up messages from Kafka (replaying a backlog). The sliding window fills instantly and fires every rule. On-call gets paged for conditions that existed before the deployment.
**Why it happens:** The window is empty at startup and the first entries immediately cross any threshold.
**How to avoid:** Set `warmupUntil = startTime + 2 * maxWindowDuration`. The engine's `Evaluate` method returns early without calling any rule during the warm-up period.
**Warning signs:** All rules fire within the first few seconds of startup; anomaly rate is highest right after deployment.

### Pitfall A2: Eviction Destroys Live State
**What goes wrong:** The eviction goroutine calls `rule.Reset()` which wipes ALL window state (including recent events). Immediately after, the window appears empty, so the rule requires a fresh build-up before it can fire again. A real ongoing spike is missed for one full window duration.
**Why it happens:** Mistaking "evict stale" for "reset all."
**How to avoid:** `Reset()` should call `window.Evict(windowDuration)` on each per-service window — dropping only entries older than the window, preserving recent events.
**Warning signs:** Rules stop firing during high-error periods that coincide with eviction cycles; anomaly gaps visible in Prometheus at regular `eviction_interval` intervals.

### Pitfall A3: Fixed Tumbling Window Boundary Artefact
**What goes wrong:** Window resets on the minute boundary. 9 errors at :59 and 9 errors at :01 never trigger a threshold of 10/minute, despite 18 errors in 2 seconds.
**Why it happens:** Using `time.Now().Truncate(window)` to bucket events into fixed intervals instead of maintaining per-event timestamps.
**How to avoid:** Use a circular timestamp buffer; fire when `CountWithin(windowDuration)` exceeds threshold.
**Warning signs:** Tests that inject two bursts of events straddling a window boundary pass, but production monitoring misses real spikes at boundary crossings.

### Pitfall G2: Lock Contention on Per-Rule Maps
**What goes wrong:** Each rule guards its per-service state with a `sync.Mutex`. Under load (thousands of log entries/sec), every `Evaluate` call acquires and releases a lock. Goroutine scheduling overhead dominates; processing latency spikes.
**Why it happens:** Reflex to add a mutex to any shared map, even one owned by a single caller.
**How to avoid:** Dispatch all rule evaluations from a single goroutine (the pipeline worker). No lock needed if no concurrent access exists.
**Warning signs:** Race detector flags the map; profiling shows `sync.(*Mutex).Lock` in the hot path.

### Pitfall C1: Cooldown State Inside Individual Rules
**What goes wrong:** Two test cases for `ErrorRateRule` run sequentially. Test 1 fires the rule and sets a cooldown timestamp inside the rule. Test 2 tries to fire the same rule but is suppressed because the cooldown from test 1 is still active.
**Why it happens:** Storing cooldown in the rule struct makes rule state encode both "detection history" and "alert suppression history."
**How to avoid:** Rules only track window state. Cooldown lives in `DetectorEngine.cooldowns`. Rules always return `(Anomaly, true)` when conditions are met; the engine decides whether to forward or suppress.
**Warning signs:** Unit tests for individual rules require `rule.Reset()` between test cases to clear cooldown; tests become order-dependent.

### Pitfall S1: ServiceSilenceRule Cold-Start False Positive
**What goes wrong:** A new service deploys, emits one log (proof-of-life), then its deployment fails and it emits nothing further. Within `silenceAfter` duration, the rule fires. But this is not a meaningful anomaly — the service was never established as "active."
**Why it happens:** The heartbeat starts tracking any service the moment it emits its first log.
**How to avoid:** Require `minLogCount` (e.g. 5) logs from a service before entering active tracking. On the first log, increment `seenCount[service]`; only add to `lastSeen` tracking once `seenCount >= minLogCount`.
**Warning signs:** Silence alerts fire frequently for short-lived services, batch jobs, or services under development.

### Pitfall V1: Viper `time.Duration` Not Decoding
**What goes wrong:** `config.Detection.WindowDuration` is 0 despite `config.yaml` setting `window_duration: "5m"`. Duration fields silently default to zero.
**Why it happens:** Viper uses `mapstructure` with `DecodeHook` options. Without the `StringToTimeDurationHookFunc`, string `"5m"` is not decoded into `time.Duration`.
**How to avoid:** Viper's default `Unmarshal` includes `mapstructure.StringToTimeDurationHookFunc()` automatically. Verify by checking the Viper source for `defaultDecoderConfig`. If using `viper.UnmarshalKey` instead of `viper.Unmarshal`, ensure `DecoderConfigOption` includes the hook.
**Warning signs:** Window durations read as 0; all rules fire immediately (threshold count 1 in 0-second window always true).

---

## Code Examples

### Sliding Window Count Within Duration

```go
// Source: internal/detection/window.go (project implementation)

func (w *slidingWindow) CountWithin(d time.Duration) int {
    cutoff := time.Now().Add(-d)
    n := 0
    for i := 0; i < w.count; i++ {
        // Walk backwards from most recent entry
        idx := (w.head - 1 - i + len(w.timestamps)) % len(w.timestamps)
        if w.timestamps[idx].After(cutoff) {
            n++
        } else {
            // Entries are in insertion order; once we hit one outside window,
            // all earlier ones are also outside (assuming monotonic Add calls)
            break
        }
    }
    return n
}
```

**Note:** The `break` optimisation is only valid if `Add` is called with monotonically increasing timestamps. For `entry.Timestamp` from Kafka this is nearly always true (log timestamps are produced in order), but for robustness against out-of-order delivery, remove the `break` and scan all entries.

### Off-Hours Rule Logic (DETECT-06)

```go
// Source: internal/detection/rules/off_hours.go (project implementation)

func (r *OffHoursAccessRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
    // Check if path is in the sensitive list
    path, ok := entry.Fields["path"].(string)
    if !ok {
        return domain.Anomaly{}, false
    }
    if !r.isSensitivePath(path) {
        return domain.Anomaly{}, false
    }
    // Check if access is outside business hours (using entry timestamp's hour)
    hour := entry.Timestamp.Hour()
    if hour >= r.cfg.BusinessHoursStart && hour < r.cfg.BusinessHoursEnd {
        return domain.Anomaly{}, false  // within business hours — OK
    }
    return domain.Anomaly{
        ID:          uuid.NewString(),
        RuleID:      r.Name(),
        Severity:    r.cfg.Severity,
        Service:     entry.Service,
        Description: fmt.Sprintf("off-hours access to %s at %02d:00", path, hour),
        Evidence:    []domain.LogEntry{entry},
        DetectedAt:  time.Now(),
    }, true
}
```

**Note:** `entry.Timestamp.Hour()` returns the hour in the timezone embedded in the `time.Time` value. If log timestamps are UTC and business hours are in a local timezone, apply `entry.Timestamp.In(loc).Hour()`. The timezone must be configurable — add a `Timezone string` field to `OffHoursConfig` and load via `time.LoadLocation`.

### Latency Breach Rule (DETECT-03)

```go
// internal/detection/rules/latency.go

type latencyWindow struct {
    totalRequests int
    breachCount   int
    timestamps    []time.Time  // one per request (for window eviction)
    breachFlags   []bool       // parallel to timestamps
}

func (r *LatencyThresholdRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
    durRaw, ok := entry.Fields["duration_ms"]
    if !ok {
        return domain.Anomaly{}, false  // skip entries without duration_ms (DETECT-03 requirement)
    }
    // ... update window, count breach rate, fire if rate > r.cfg.BreachRatePercent
}
```

**Key detail:** Entries without `duration_ms` are silently skipped. This prevents the rule from misinterpreting non-HTTP log entries (which have no latency field) as non-breaching requests that dilute the breach rate.

### Config YAML Shape

```yaml
# config.yaml additions for Phase 2
detection:
  window_duration: "5m"
  eviction_interval: "1m"
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
      severity: "high"
    off_hours:
      enabled: true
      business_hours_start: 9
      business_hours_end: 17
      timezone: "UTC"
      sensitive_paths:
        - "/admin"
        - "/api/v1/users"
        - "/internal"
      severity: "medium"
    service_silence:
      enabled: true
      silence_after: "5m"
      check_interval: "30s"
      min_log_count: 5
      severity: "critical"
```

### DetectorEngine Wiring in main.go

```go
// cmd/server/main.go (addition to Phase 1 wiring)

silenceRule := rules.NewServiceSilenceRule(cfg.Detection.Rules.ServiceSilence, logger)
detectors := []domain.Detector{
    rules.NewErrorRateRule(cfg.Detection.Rules.ErrorRate),
    rules.NewLatencyThresholdRule(cfg.Detection.Rules.Latency),
    rules.NewRepeatedFailureRule(cfg.Detection.Rules.RepeatedFailure),
    rules.NewAuthFailureBurstRule(cfg.Detection.Rules.AuthBurst),
    rules.NewOffHoursAccessRule(cfg.Detection.Rules.OffHours),
    silenceRule,  // Evaluate() updates last_seen; CheckSilence() called by heartbeat
}
engine := detection.NewDetectorEngine(detectors, cfg.Detection, logger)
defer engine.Stop()

// Pipeline: for each parsed entry
entry := pipeline.ProcessMessage(msg, logger)
engine.Evaluate(entry)

// Drain anomalies (separate goroutine, Phase 3 wires this to alerter)
g.Go(func() error {
    for {
        select {
        case anomaly := <-engine.Anomalies():
            logger.Info("anomaly detected",
                zap.String("rule", anomaly.RuleID),
                zap.String("service", anomaly.Service),
            )
            // Phase 3: forward to alerter channel
        case <-ctx.Done():
            return nil
        }
    }
})
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed tumbling windows | Sliding window with per-event timestamp buffer | Standard practice | No boundary artefacts; more accurate rate detection |
| Hardcoded thresholds in source | YAML-driven config with Viper | Standard in mature ops tooling | Operators tune without redeployment |
| Single global rule state | Per-service window maps keyed by service name | Standard for multi-tenant log analytics | Rules correctly track per-service rates independently |
| Mutex-per-rule concurrency | Single-goroutine ownership of all window state | Go-idiomatic channel/ownership model | Eliminates lock contention on hot path; race-clean by design |

**Deprecated/outdated:**
- `sync.Map` for per-service state: `sync.Map` is optimised for the case of many reads, few writes per key. Detection windows have frequent writes. A plain `map[string]*window` owned by one goroutine is better.
- `time.Sleep` in heartbeat loops: `time.NewTicker` is the correct abstraction; `Sleep` drifts under load and cannot be cleanly stopped without goroutine leaks.

---

## Open Questions

1. **Timezone for OffHoursAccessRule**
   - What we know: `entry.Timestamp` is parsed from the log's `timestamp` field via `time.Parse(time.RFC3339, ...)` which preserves the timezone offset.
   - What's unclear: Whether all services in the target environment emit UTC timestamps or local-timezone timestamps.
   - Recommendation: Make timezone configurable (`off_hours.timezone: "America/New_York"`). Parse via `time.LoadLocation`; fall back to UTC if not set. Document in config.yaml comments.

2. **`duration_ms` field type in LogEntry.Fields**
   - What we know: `LogEntry.Fields` is `map[string]any`. JSON numbers decode to `float64` via `encoding/json`.
   - What's unclear: Whether upstream services emit duration as integer or float; whether the field is named `duration_ms` or `latency_ms`.
   - Recommendation: Latency rule should accept both `duration_ms` and `latency_ms` as field names (check both). Type-assert `float64` first; if that fails, try `int` and `int64`. Document the supported field names in config.

3. **Auth field extraction for AuthFailureBurstRule**
   - What we know: Auth failure events must be keyed by IP or username. These values come from `LogEntry.Fields`.
   - What's unclear: What field names are used by the target services (`"ip"`, `"source_ip"`, `"remote_addr"`, `"user"`, `"username"`).
   - Recommendation: Make source field names configurable (`auth_burst.ip_field: "source_ip"`, `auth_burst.user_field: "username"`). Rule checks configured field name first; falls back to common alternatives. If neither is found, skip the entry.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `github.com/stretchr/testify` v1.11.1 |
| Config file | None required — stdlib testing |
| Quick run command | `go test -race ./internal/detection/...` |
| Full suite command | `go test -race ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DETECT-01 | Engine calls all registered detectors for each log entry | unit | `go test -race ./internal/detection/... -run TestEngine_EvaluatesAllDetectors` | Wave 0 |
| DETECT-02 | ErrorRateRule fires when error count exceeds threshold in window | unit | `go test -race ./internal/detection/... -run TestErrorRateRule` | Wave 0 |
| DETECT-03 | LatencyThresholdRule fires when breach rate exceeds N% in window | unit | `go test -race ./internal/detection/... -run TestLatencyThresholdRule` | Wave 0 |
| DETECT-04 | RepeatedFailureRule fires on same fingerprint >N times in window | unit | `go test -race ./internal/detection/... -run TestRepeatedFailureRule` | Wave 0 |
| DETECT-05 | AuthFailureBurstRule fires when per-key count exceeds threshold | unit | `go test -race ./internal/detection/... -run TestAuthFailureBurstRule` | Wave 0 |
| DETECT-06 | OffHoursAccessRule fires for sensitive path access outside hours | unit | `go test -race ./internal/detection/... -run TestOffHoursAccessRule` | Wave 0 |
| DETECT-07 | ServiceSilenceRule fires when active service emits no logs for silence_after | unit | `go test -race ./internal/detection/... -run TestServiceSilenceRule` | Wave 0 |
| DETECT-07 | ServiceSilenceRule does NOT fire during cold-start (min_log_count not met) | unit | `go test -race ./internal/detection/... -run TestServiceSilenceRule_ColdStart` | Wave 0 |
| DETECT-08 | Cooldown suppresses repeated alerts for same (rule, service) pair | unit | `go test -race ./internal/detection/... -run TestEngine_Cooldown` | Wave 0 |
| DETECT-08 | Warm-up period suppresses all alerts regardless of input volume | unit | `go test -race ./internal/detection/... -run TestEngine_WarmupSuppression` | Wave 0 |
| DETECT-09 | All thresholds and parameters loaded from config struct (no hardcoded values) | unit | `go test -race ./internal/detection/... -run TestEngine_ConfigDriven` | Wave 0 |
| DETECT-10 | Stale window state for inactive services is evicted; map size bounded | unit | `go test -race ./internal/detection/... -run TestEngine_StaleWindowEviction` | Wave 0 |

### Testing Injected Timestamps Pattern

All rule unit tests must inject synthetic timestamps rather than relying on `time.Now()`:

```go
// Source: pattern established from Phase 1 parser tests (internal/pipeline/parser_test.go)

func TestErrorRateRule_Fires(t *testing.T) {
    rule := NewErrorRateRule(ErrorRateConfig{Threshold: 3, Window: 5 * time.Minute})
    base := time.Now()
    entries := []domain.LogEntry{
        {Level: "error", Service: "api", Timestamp: base.Add(-4 * time.Minute)},
        {Level: "error", Service: "api", Timestamp: base.Add(-3 * time.Minute)},
        {Level: "error", Service: "api", Timestamp: base.Add(-2 * time.Minute)},
    }
    for _, e := range entries[:2] {
        _, fired := rule.Evaluate(e)
        assert.False(t, fired, "should not fire before threshold")
    }
    _, fired := rule.Evaluate(entries[2])
    assert.True(t, fired, "should fire at threshold")
}
```

### Sampling Rate

- **Per task commit:** `go test -race ./internal/detection/...` (detection tests only, < 10s)
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/detection/engine.go` — DetectorEngine implementation
- [ ] `internal/detection/window.go` — slidingWindow helper
- [ ] `internal/detection/rules/error_rate.go` — ErrorRateRule
- [ ] `internal/detection/rules/latency.go` — LatencyThresholdRule
- [ ] `internal/detection/rules/repeated_failure.go` — RepeatedFailureRule
- [ ] `internal/detection/rules/auth_burst.go` — AuthFailureBurstRule
- [ ] `internal/detection/rules/off_hours.go` — OffHoursAccessRule
- [ ] `internal/detection/rules/service_silence.go` — ServiceSilenceRule
- [ ] `internal/detection/engine_test.go` — engine integration tests
- [ ] `internal/detection/rules/rules_test.go` — table-driven rule tests
- [ ] `TestMain` with `goleak.VerifyTestMain(m)` in `internal/detection` package
- [ ] Extend `internal/config/config.go` with `DetectionConfig` struct tree
- [ ] Update `config.yaml` with `detection:` section

---

## Sources

### Primary (HIGH confidence)

- `internal/domain/interfaces.go` — `domain.Detector` interface definition (project source, verified 2026-03-21)
- `internal/domain/types.go` — `LogEntry`, `Anomaly` struct definitions (project source, verified 2026-03-21)
- `internal/metrics/metrics.go` — `AnomaliesDetectedTotal` counter already registered (project source, verified 2026-03-21)
- `internal/config/config.go` — existing Viper config loading pattern with `mapstructure` tags (project source, verified 2026-03-21)
- `go.mod` — confirms all Phase 2 dependencies present: viper v1.21.0, zap v1.27.0, prometheus v1.23.2, testify v1.11.1, goleak v1.3.0 (project source, verified 2026-03-21)
- `.planning/phases/01-foundation-and-ingestion/01-RESEARCH.md` — Phase 1 patterns, goleak VerifyTestMain, single-goroutine ownership rationale (project research, verified 2026-03-21)

### Secondary (MEDIUM confidence)

- Sliding window anomaly detection patterns: standard CS algorithm — per-event timestamp buffer with O(N) scan for count-within-window is the canonical approach for low-to-medium cardinality event streams. No external source needed; algorithm is well-established.
- Message fingerprinting via regex: common in log aggregation systems (ELK, Splunk, Loki). Approach of stripping numerics and IDs to produce stable fingerprints is well-documented in ops engineering literature.

### Tertiary (LOW confidence)

- ServiceSilenceRule heartbeat pattern with adapter approach vs interface extension: based on Go interface idiom analysis. No single authoritative source — recommendation based on Go standard library patterns (`io.Reader` optional interfaces).

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod; no new dependencies
- Architecture: HIGH — domain types and Detector interface already defined; patterns follow Go idiomatic concurrency model
- Sliding window algorithm: HIGH — standard CS algorithm; no library needed
- Pitfalls: HIGH — A1/A2/A3 directly observed pattern from anomaly detection literature; G2 directly addressed in ROADMAP.md plan descriptions; C1/S1/V1 derived from the specific implementation patterns chosen
- Config patterns: HIGH — Viper usage already validated in Phase 1; extending the same struct pattern is straightforward
- Validation architecture: HIGH — test framework identical to Phase 1; test commands follow same go test -race pattern

**Research date:** 2026-03-21
**Valid until:** 2026-04-21 (all libraries stable; no external API dependencies)
