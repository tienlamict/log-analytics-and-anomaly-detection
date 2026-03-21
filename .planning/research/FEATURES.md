# Features Research: Log Analytics & Anomaly Detection

**Domain:** Event-driven log analytics and anomaly detection (Go + Kafka + Elasticsearch)
**Researched:** 2026-03-21
**Confidence:** HIGH (domain is mature; patterns drawn from ELK stack, Splunk, Datadog, Grafana Loki, production Go pipelines)

---

## Table Stakes

Features every system in this space must have or it is not useful.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Structured log ingestion | Raw JSON/logfmt logs arrive with inconsistent field names; consumers need normalized fields | Low | Parse `level`, `timestamp`, `service`, `message`, `trace_id` at minimum |
| Unstructured log fallback | Not all services emit structured logs; regex/pattern extraction keeps pipeline useful | Medium | Treat unparseable lines as plain-text with level=unknown; don't drop |
| Log persistence with time index | Logs are only useful if searchable across time windows; Elasticsearch mapping must include `@timestamp` | Low | ILM (Index Lifecycle Management) policies prevent unbounded storage growth |
| Anomaly record persistence | Detected anomalies must survive process restart; in-memory-only detection has no audit trail | Low | Separate ES index from raw logs; anomaly document links back to log IDs |
| Time-window rule evaluation | All meaningful detection (error rate, auth failure bursts) requires counting over a rolling window, not per-event | Medium | Sliding window (last N seconds) is more accurate than fixed tumbling window |
| Threshold-based alerting | The core value proposition — something crossed a limit, notify now | Low | Alert deduplication (don't fire 500 emails for 500 errors) is critical |
| Email alert delivery | Simplest reliable out-of-band channel for operations teams | Low | Must include: anomaly type, service, time, sample log lines, severity |
| Alert deduplication / cooldown | Without cooldown, a single error storm generates thousands of identical alerts | Low | Per-rule cooldown window (e.g., 5 min per service per rule type) is sufficient for v1 |
| REST API: query logs | Operators need to look up what happened around an incident time | Low | Minimum: filter by service, level, time range, pagination |
| REST API: query anomalies | Anomaly history is needed for post-mortem, trending, and dashboard consumption later | Low | Filter by type, severity, service, time range |
| Health/readiness endpoints | Kafka consumer lag and ES write failures are invisible without a `/health` endpoint | Low | `/health` and `/ready` — standard for container orchestration |
| Kafka consumer offset management | Crash recovery must resume from last committed offset, not reprocess everything from the start | Medium | Use consumer group commits after confirmed ES write, not before |
| Graceful shutdown | Kafka rebalances and in-flight messages must be flushed cleanly on SIGTERM | Low | Critical for zero data loss at <1k/sec; accept and flush, then close |
| Log level classification | Consumers of the API expect `ERROR`, `WARN`, `INFO`, `DEBUG` — normalize vendor-specific variants | Low | Map `err`, `error`, `ERR`, `FATAL` → `ERROR`; `warn`, `WARNING` → `WARN` |
| Service/source identification | Every log must carry service identity; anomalies without service attribution are not actionable | Low | Parse from log field or Kafka topic partition by convention |

---

## Differentiators

Features that make a system stand out beyond the baseline.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Per-service rule configuration | Different services have different normal error rates; one global threshold causes false positives | Medium | Config-driven rules per service label; fallback to global defaults |
| Anomaly severity tiers | Not all anomalies are equally urgent; INFO/WARN/CRITICAL tiering reduces alert fatigue | Low | Rule definitions include severity; affects email priority/subject line |
| Rule hot-reload (no restart) | Tuning thresholds during incidents without redeploying is operationally valuable | Medium | Watch config file via `fsnotify`; atomic rule swap; log when rules change |
| Trace ID correlation | When anomaly is detected, linking back to a specific trace ID enables instant root-cause lookup | Low | Requires trace_id field in log; store in anomaly record; pass in alert email |
| Anomaly suppression window | During planned maintenance, operators want to mute alerts for a service without changing rules | Medium | API endpoint to suppress a service's alerts for N minutes |
| Request ID / correlation ID threading | Group log lines from a single request; useful for API query response (show all logs for request X) | Medium | Requires correlation_id field; ES query by correlation_id |
| Consumer lag monitoring | Kafka lag is a leading indicator of pipeline health degradation | Low | Expose lag metric; alert when lag > threshold (pipeline falling behind) |
| Anomaly rate trending | "Errors are up 3x compared to same time yesterday" is more actionable than raw counts | High | Requires baseline computation; defer to v2 unless simple daily comparison is acceptable |
| Structured alert payload | Email body includes JSON or structured fields, not just plain text — enables future webhook/PagerDuty integration | Low | Low effort, high future value; design email template with structured block |
| Dead letter queue for unparseable logs | Unparseable logs silently dropped are an invisible data quality problem | Low | Route to a `logs-dlq` Kafka topic or ES index for inspection |

---

## Anti-Features (avoid in v1)

Things that look useful but add complexity without v1 value.

| Anti-Feature | Why Avoid in v1 | What to Do Instead |
|--------------|-----------------|-------------------|
| ML / statistical anomaly detection | Requires training data, baseline periods, model lifecycle; unpredictable false positive rate until tuned | Rule-based thresholds are predictable, debuggable, and sufficient for known patterns |
| Dashboard / UI | Requires frontend framework, auth, session management, CORS, static serving — doubles the project surface area | REST API is the v1 interface; any team can build a dashboard on top |
| Multi-tenancy / namespaced rules | Per-tenant isolation requires auth middleware, tenant-scoped ES indices, and rule scoping — large complexity multiplier | Single-tenant design; add when a second team/product needs to share the system |
| Webhook / Slack alerting | Requires configuring delivery targets, retry logic, payload templates per channel | Email SMTP is simpler; webhook is a v2 addition once alert routing patterns are known |
| Log agent / shipper | Writing a log tail agent is a separate product; existing tools (Filebeat, Fluent Bit, Vector) do this well | Services push directly to Kafka; if they can't, use Filebeat as the Kafka producer |
| Log archival / cold storage tiering | S3 archival and ILM tiering are operational concerns, not product features | Configure ES ILM delete policy at 30 days; revisit when storage cost is an actual problem |
| Real-time streaming API (SSE/WebSocket) | Requires managing open connections, backpressure, and reconnect logic | Poll the REST API; streaming is a v2 feature when low-latency UI is needed |
| Complex query DSL / aggregations API | Building a SQL-like or Elasticsearch pass-through query API adds a massive design surface | Expose pre-defined query endpoints (by service, level, time range); avoid generic query builders |
| Alert routing rules engine | Routing "critical alerts to on-call, warnings to Slack" requires a rules engine, teams model, and schedule integration | All alerts go to a configured SMTP address for v1 |
| Distributed tracing integration | Correlating logs with Jaeger/Zipkin spans requires trace context propagation infrastructure | Store trace_id in log records and anomalies as a field; actual span correlation is v2 |
| Schema registry / log format versioning | Managing log format evolution across services is a real problem — but not until you have multiple services with drift | Accept current log shapes; document expected fields; revisit when format drift causes parsing failures |
| High-availability / clustering | Leader election, partition rebalancing, distributed state are premature at <1k/sec | Single consumer process with Kafka offset management provides sufficient resilience |

---

## Rule-Based Detection Patterns

Common detection rules used in production systems. These are the canonical patterns every log anomaly detector implements.

### Error Rate Spike

**What:** Count of `ERROR`-level log lines for a service exceeds N per time window.

**Pattern:**
```
WHEN count(level=ERROR, service=X) > threshold WITHIN window_seconds
AND NOT in_cooldown(service=X, rule=error_rate)
THEN emit anomaly(type=error_rate_spike, service=X, count=N)
```

**Typical thresholds:** >10 errors/min is notable; >50/min is critical for most HTTP services.
**Key detail:** Count is per-service, not global. One noisy service must not mask others.
**Cooldown:** 5 minutes prevents alert storms. Reset cooldown when count drops below threshold.

---

### Latency Threshold Breach

**What:** Log lines reporting response time (or duration) exceed a P95/P99 threshold.

**Pattern:**
```
WHEN log.latency_ms > threshold_ms AND service=X
AND rate_of_breach(service=X) > breach_rate_percent WITHIN window_seconds
THEN emit anomaly(type=latency_breach, service=X, p_value=latency_ms)
```

**Key detail:** Single slow request is not an anomaly. The rule triggers when >N% of requests in the window are slow. Without rate-of-breach check, a single 5s request fires a critical alert.
**Field dependency:** Requires `duration_ms` or `latency_ms` field in log; not all log formats include this.

---

### Repeated Failures (Crash Loop / Retry Storm)

**What:** The same error message or error code appears repeatedly in a short window, indicating a retry loop or crash loop.

**Pattern:**
```
WHEN count(level=ERROR, error_code=X, service=Y) > repeat_threshold WITHIN window_seconds
AND NOT in_cooldown(service=Y, rule=repeated_failure, error_code=X)
THEN emit anomaly(type=repeated_failure, service=Y, error_code=X, count=N)
```

**Key detail:** Group by `error_code` or normalized `message` fingerprint. Without grouping, 1000 unique errors don't trigger this rule, but 10 identical errors do.
**Message fingerprinting:** Strip dynamic values (IDs, timestamps) from message before grouping. "Failed to connect to db-1234" and "Failed to connect to db-5678" are the same class.

---

### Authentication Failure Burst

**What:** Multiple authentication failures from the same IP or for the same user within a time window. Indicates brute force or credential stuffing.

**Pattern:**
```
WHEN count(event=auth_failure, source_ip=X) > auth_threshold WITHIN window_seconds
THEN emit anomaly(type=auth_failure_burst, source_ip=X, count=N, severity=CRITICAL)

OR

WHEN count(event=auth_failure, user_id=X) > auth_threshold WITHIN window_seconds
THEN emit anomaly(type=account_lockout_risk, user_id=X, count=N, severity=HIGH)
```

**Key detail:** Two sub-rules — IP-based (brute force from one source) and account-based (password spray across IPs targeting one account). Both matter.
**Typical threshold:** >5 failures/minute for a single IP is suspicious; >10 for a single user.

---

### Unusual Access Pattern (Off-Hours / Unexpected Endpoint)

**What:** Authenticated access to sensitive endpoints during unusual hours or from unexpected geolocations.

**Pattern:**
```
WHEN event=access AND endpoint IN sensitive_endpoints
AND hour_of_day NOT IN business_hours
THEN emit anomaly(type=off_hours_access, user_id=X, endpoint=Y, severity=WARN)
```

**Key detail:** This rule is configuration-heavy (define sensitive endpoints, define business hours). For v1, a simple allowlist of sensitive path prefixes (e.g., `/admin`, `/internal`) plus a time range check is sufficient.
**Limitation:** No geo-IP in v1 scope; IP-pattern anomalies require MaxMind or similar — defer to v2.

---

### Service Availability / Zero-Traffic Detection

**What:** A service that normally emits logs stops emitting them — could indicate a crash, network partition, or dead consumer.

**Pattern:**
```
WHEN last_seen(service=X) > silence_threshold_seconds
AND service=X IN known_active_services
THEN emit anomaly(type=service_silence, service=X, last_seen=T, severity=HIGH)
```

**Key detail:** This rule runs on a heartbeat timer, not on incoming log events. It requires maintaining a `last_seen` map per service and a background goroutine that checks it periodically.
**Gotcha:** New/ephemeral services trigger false positives. Require N logs before a service enters the "active" tracking set.

---

### Error Rate Ratio (Relative to Request Volume)

**What:** The error rate as a percentage of total requests exceeds a threshold. Prevents false alerts when a low-traffic service has 1 error (100% error rate but not an incident).

**Pattern:**
```
WHEN count(level=ERROR, service=X) / count(level=ANY, service=X) > error_ratio_threshold WITHIN window_seconds
AND count(level=ANY, service=X) > minimum_volume
THEN emit anomaly(type=error_ratio_high, service=X, ratio=R, severity=WARN)
```

**Key detail:** The `minimum_volume` guard is critical. Without it, a service that processes 1 request with 1 error is reported as 100% error rate.
**Typical threshold:** >5% error ratio over 100+ requests is worth flagging; >20% is critical.

---

## Detection Engine Design Notes

These are implementation constraints that emerge from the rules above, relevant for architecture decisions.

**Windowed state is required.** All meaningful rules require counting events over a time window. This means the detector must maintain in-memory state (counters, last-seen timestamps) keyed by `(service, rule_type)` or `(service, error_code)`. This state is ephemeral — a restart resets windows, which is acceptable at <1k/sec with short windows (1-5 min).

**Rule evaluation is per-message, not batch.** Each incoming log event triggers evaluation against all applicable rules. Rules check current window state. This is different from batch processing (Spark, Flink) — the Go process evaluates synchronously in the consumer loop.

**State cleanup is necessary.** Services that stop sending logs leave stale window entries. A background goroutine must expire window entries older than `max_window_duration` to prevent memory growth.

**Rule configuration must be data-driven, not code.** Hardcoding thresholds in Go source requires redeployment to tune. A YAML/JSON config file with rule definitions allows operators to adjust without code changes.

**Alert deduplication state is separate from detection state.** Cooldown tracking (has this rule fired recently for this service?) is a different concern from window counting. Keep them in separate maps.

---

## Sources

- Training knowledge: ELK stack, Splunk enterprise features, Datadog APM and log management, Grafana Loki, OpenTelemetry collector, production Go log pipeline patterns (HIGH confidence — mature, stable domain)
- Pattern derivation: Standard SIEM rule patterns (Sigma rule format), Elasticsearch Watcher rule examples, production incident post-mortem patterns (HIGH confidence)
- WebSearch: Unavailable — findings rely on training knowledge; core log analytics feature set is stable and unlikely to have changed materially
