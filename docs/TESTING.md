#e2etest:
# Default: 100 msg/s, 3 phút, tự verify
go run ./cmd/e2etest

# Tăng tải: 500 msg/s, 5 phút
go run ./cmd/e2etest --rate 500 --duration 5m

# Custom broker/API
go run ./cmd/e2etest --broker localhost:9092 --api http://localhost:8080 --rate 200


# Manual System Testing Guide
## 1. Start the Stack

```bash
docker-compose up -d
```

Wait ~30s for Elasticsearch and Kafka to be ready, then verify:

```bash
# All 4 containers should be Up
docker-compose ps

# Check app logs
docker-compose logs -f app
```

---

## 2. Verify Health Endpoints

```bash
# Liveness — checks ES connectivity
curl http://localhost:8080/health

# Readiness — checks server is up
curl http://localhost:8080/ready

# Prometheus metrics
curl http://localhost:2112/metrics
```

Expected: `{"status":"ok"}` for both health/ready.

---

## 3. Produce Test Logs to Kafka

You'll publish JSON to the `application-logs` topic. Run a shell inside the Kafka container:

```bash
docker exec -it $(docker-compose ps -q kafka) bash
```

Then use the built-in producer:

```bash
kafka-console-producer \
  --bootstrap-server localhost:9092 \
  --topic application-logs
```

Now paste messages one by one (each line is one message).

### 3a. Basic log types

```json
{"timestamp":"2026-03-23T10:00:00Z","level":"info","service":"auth-service","message":"User login successful","fields":{"username":"alice","source_ip":"10.0.0.1"}}
{"timestamp":"2026-03-23T10:00:01Z","level":"warn","service":"payment-service","message":"Slow DB query","fields":{"latency_ms":350}}
{"timestamp":"2026-03-23T10:00:02Z","level":"error","service":"auth-service","message":"Database connection failed","fields":{}}
{"timestamp":"2026-03-23T10:00:03Z","level":"debug","service":"inventory-service","message":"Cache miss for product 42","fields":{}}
```

### 3b. Trigger **Error Rate Spike** (≥10 errors in 5 min → HIGH)

Paste these 10 error lines rapidly:

```json
{"timestamp":"2026-03-23T10:01:00Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:01Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:02Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:03Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:04Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:05Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:06Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:07Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:08Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
{"timestamp":"2026-03-23T10:01:09Z","level":"error","service":"payment-service","message":"Payment gateway timeout","fields":{}}
```

### 3c. Trigger **Latency Threshold Breach** (≥20% of requests >500ms → MEDIUM)

```json
{"timestamp":"2026-03-23T10:02:00Z","level":"info","service":"api-gateway","message":"GET /products","fields":{"latency_ms":100}}
{"timestamp":"2026-03-23T10:02:01Z","level":"info","service":"api-gateway","message":"GET /products","fields":{"latency_ms":120}}
{"timestamp":"2026-03-23T10:02:02Z","level":"warn","service":"api-gateway","message":"GET /products","fields":{"latency_ms":800}}
{"timestamp":"2026-03-23T10:02:03Z","level":"warn","service":"api-gateway","message":"GET /products","fields":{"latency_ms":950}}
{"timestamp":"2026-03-23T10:02:04Z","level":"warn","service":"api-gateway","message":"GET /products","fields":{"latency_ms":1200}}
```

### 3d. Trigger **Auth Failure Burst** (≥10 auth failures from same IP → HIGH)

```json
{"timestamp":"2026-03-23T10:03:00Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:01Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:02Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:03Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:04Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:05Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:06Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:07Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:08Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
{"timestamp":"2026-03-23T10:03:09Z","level":"error","service":"auth-service","message":"login failed unauthorized","fields":{"source_ip":"192.168.1.99","username":"attacker"}}
```

### 3e. Trigger **Repeated Failure** (same error ≥5 times → MEDIUM)

```json
{"timestamp":"2026-03-23T10:04:00Z","level":"error","service":"order-service","message":"NullPointerException in OrderProcessor.java:42","fields":{}}
{"timestamp":"2026-03-23T10:04:01Z","level":"error","service":"order-service","message":"NullPointerException in OrderProcessor.java:42","fields":{}}
{"timestamp":"2026-03-23T10:04:02Z","level":"error","service":"order-service","message":"NullPointerException in OrderProcessor.java:42","fields":{}}
{"timestamp":"2026-03-23T10:04:03Z","level":"error","service":"order-service","message":"NullPointerException in OrderProcessor.java:42","fields":{}}
{"timestamp":"2026-03-23T10:04:04Z","level":"error","service":"order-service","message":"NullPointerException in OrderProcessor.java:42","fields":{}}
```

### 3f. Trigger **Off-Hours Access** (access to /admin outside 09:00–17:00 UTC)

> Use a timestamp that is currently outside 09-17 UTC (e.g., 02:00Z or 20:00Z):

```json
{"timestamp":"2026-03-23T20:15:00Z","level":"info","service":"admin-portal","message":"GET /admin/settings","fields":{"username":"bob","source_ip":"10.0.0.5"}}
{"timestamp":"2026-03-23T20:15:01Z","level":"info","service":"admin-portal","message":"POST /api/v1/users","fields":{"username":"bob","source_ip":"10.0.0.5"}}
{"timestamp":"2026-03-23T20:15:02Z","level":"info","service":"admin-portal","message":"GET /internal/health","fields":{"username":"bob"}}
```

### 3g. Trigger **Service Silence** (service stops logging → CRITICAL after 5 min)

First warm up the service (send ≥5 logs), then stop sending:

```json
{"timestamp":"2026-03-23T10:05:00Z","level":"info","service":"billing-service","message":"Invoice generated","fields":{}}
{"timestamp":"2026-03-23T10:05:01Z","level":"info","service":"billing-service","message":"Invoice generated","fields":{}}
{"timestamp":"2026-03-23T10:05:02Z","level":"info","service":"billing-service","message":"Invoice generated","fields":{}}
{"timestamp":"2026-03-23T10:05:03Z","level":"info","service":"billing-service","message":"Invoice generated","fields":{}}
{"timestamp":"2026-03-23T10:05:04Z","level":"info","service":"billing-service","message":"Invoice generated","fields":{}}
```

Then wait ~5 minutes without sending any `billing-service` logs. The heartbeat checker fires every 30s.

---

## 4. Query Logs via REST API

```bash
# All logs (first page)
curl "http://localhost:8080/api/v1/logs"

# Filter by service
curl "http://localhost:8080/api/v1/logs?service=auth-service"

# Filter by level
curl "http://localhost:8080/api/v1/logs?level=error"

# Time range (RFC3339)
curl "http://localhost:8080/api/v1/logs?from=2026-03-23T10:00:00Z&to=2026-03-23T10:10:00Z"

# Pagination
curl "http://localhost:8080/api/v1/logs?page=1&size=20"

# Get a single log by ID (use an ID from the list response)
curl "http://localhost:8080/api/v1/logs/<id>"
```

---

## 5. Query Anomalies via REST API

```bash
# All anomalies
curl "http://localhost:8080/api/v1/anomalies"

# Filter by severity
curl "http://localhost:8080/api/v1/anomalies?severity=HIGH"
curl "http://localhost:8080/api/v1/anomalies?severity=CRITICAL"

# Filter by rule type
curl "http://localhost:8080/api/v1/anomalies?type=error_rate_spike"
curl "http://localhost:8080/api/v1/anomalies?type=auth_failure_burst"
curl "http://localhost:8080/api/v1/anomalies?type=latency_threshold"
curl "http://localhost:8080/api/v1/anomalies?type=repeated_failure"
curl "http://localhost:8080/api/v1/anomalies?type=off_hours"
curl "http://localhost:8080/api/v1/anomalies?type=service_silence"

# Filter by service
curl "http://localhost:8080/api/v1/anomalies?service=payment-service"

# Time range
curl "http://localhost:8080/api/v1/anomalies?from=2026-03-23T10:00:00Z&to=2026-03-23T11:00:00Z"

# Single anomaly by ID
curl "http://localhost:8080/api/v1/anomalies/<id>"
```

---

## 6. Query Elasticsearch Directly

### 6a. Check indices

```bash
# List all indices
curl "http://localhost:9200/_cat/indices?v"

# Should show: logs-YYYY.MM.DD and anomalies
```

### 6b. Query log documents

```bash
# Count all logs today
curl "http://localhost:9200/logs-$(date +%Y.%m.%d)/_count"

# Get 10 most recent logs
curl -X GET "http://localhost:9200/logs-$(date +%Y.%m.%d)/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "size": 10,
  "sort": [{"@timestamp": {"order": "desc"}}],
  "query": {"match_all": {}}
}'

# Filter logs by service
curl -X GET "http://localhost:9200/logs-$(date +%Y.%m.%d)/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "term": {"service": "auth-service"}
  }
}'

# Filter by level = error
curl -X GET "http://localhost:9200/logs-$(date +%Y.%m.%d)/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "term": {"level": "error"}
  }
}'

# Time range query
curl -X GET "http://localhost:9200/logs-$(date +%Y.%m.%d)/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "range": {
      "@timestamp": {
        "gte": "2026-03-23T10:00:00Z",
        "lte": "2026-03-23T11:00:00Z"
      }
    }
  }
}'
```

### 6c. Query anomaly documents

```bash
# Count all anomalies
curl "http://localhost:9200/anomalies/_count"

# All anomalies, most recent first
curl -X GET "http://localhost:9200/anomalies/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "size": 20,
  "sort": [{"detected_at": {"order": "desc"}}],
  "query": {"match_all": {}}
}'

# Filter by severity = HIGH
curl -X GET "http://localhost:9200/anomalies/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "term": {"severity": "HIGH"}
  }
}'

# Filter by rule
curl -X GET "http://localhost:9200/anomalies/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "term": {"rule_id": "error_rate_spike"}
  }
}'

# Filter by service
curl -X GET "http://localhost:9200/anomalies/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "term": {"service": "payment-service"}
  }
}'

# Combined filter: HIGH severity auth anomalies
curl -X GET "http://localhost:9200/anomalies/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "bool": {
      "must": [
        {"term": {"severity": "HIGH"}},
        {"term": {"rule_id": "auth_failure_burst"}}
      ]
    }
  }
}'
```

### 6d. Index mapping (see what fields exist)

```bash
curl "http://localhost:9200/anomalies/_mapping?pretty"
curl "http://localhost:9200/logs-$(date +%Y.%m.%d)/_mapping?pretty"
```

---

## 7. View Logs in Kibana (ELK)

Open **http://localhost:5601** in your browser — no login required.

Two data views are auto-provisioned on first startup:

| Data View | Index | Time field | Use for |
|---|---|---|---|
| **Application Logs** | `logs-*` | `@timestamp` | Browsing raw log entries |
| **Anomalies** | `anomalies` | `detected_at` | Browsing detected anomalies |

### Quick steps to explore logs:

1. Go to **Discover** (left sidebar)
2. Select data view **"Application Logs"** (top-left dropdown)
3. Set time range to **Last 1 hour** (top-right)
4. Use the search bar with KQL:
   ```
   level : "error"
   service : "auth-service"
   service : "payment-service" and level : "error"
   fields.source_ip : "192.168.1.99"
   ```
5. Click any log row to expand and see all fields

### Available fields per log entry:

| Field | Type | Example |
|---|---|---|
| `@timestamp` | date | `2026-03-23T10:03:00Z` |
| `level` | keyword | `error`, `warn`, `info`, `debug` |
| `service` | keyword | `auth-service`, `payment-service` |
| `message` | text | `login failed unauthorized` |
| `fields.source_ip` | keyword | `192.168.1.99` |
| `fields.username` | keyword | `attacker` |
| `fields.latency_ms` | number | `800` |
| `fields.path` | keyword | `/api/v1/login` |

---

## 7a. Check Grafana Dashboards (Metrics only)

Open **http://localhost:3000** (admin / admin123) in your browser.

The **Log Analytics & Anomaly Detection** dashboard is pre-provisioned with 7 panels:

| Panel | Shows |
|---|---|
| Log Ingestion Rate | Kafka messages/s arriving |
| Processing Latency (p50/p95/p99) | End-to-end pipeline latency |
| Anomaly Detection Rate | Anomalies/s by rule type |
| Anomalies by Rule (Total) | Pie chart of all-time anomaly counts |
| Kafka Consumer Lag | Backlog depth per partition |
| Elasticsearch Write Errors | ES failures in last 1h (red ≥ 10) |
| Parse Errors | Parse failures in last 1h (red ≥ 50) |

**Grafana alert rules** can be added directly in the UI against any panel expression —
for example, alert when `rate(anomalies_detected_total[5m]) > 0`.

---

## 8. Check Prometheus Metrics

```bash
curl -s http://localhost:2112/metrics | grep -E "logs_|anomalies_|es_write_|parse_|kafka_"
```

Key metrics to verify:

| Metric | Expect |
|---|---|
| `logs_consumed_total` | increments as Kafka messages arrive |
| `anomalies_detected_total{rule=…}` | increments per anomaly rule fired |
| `kafka_consumer_lag` | should trend toward 0 |
| `elasticsearch_write_errors_total` | should stay at 0 |
| `parse_errors_total` | should stay near 0 for well-formed JSON |

---

## 9. High-Load System Test

The automated high-load test spins up real Kafka and Elasticsearch containers via testcontainers,
produces 10,000 messages, and verifies all 5 anomaly rules fire under concurrent load.

```bash
go test -v -tags integration ./internal/integration/ -run TestHighLoad -timeout 5m
```

**Message mix (10,000 total):**

| Scenario | Count | Expected Rule |
|---|---|---|
| Normal info/warn logs | 9,490 | (baseline traffic) |
| Errors from `payment-service` | 200 | `error_rate_spike` |
| Mixed-latency to `api-gateway` (50% slow) | 200 | `latency_threshold` |
| Auth failures from same IP | 50 | `auth_failure_burst` |
| Identical errors from `order-service` | 50 | `repeated_failure` |
| `/admin` access at 02:00 UTC | 10 | `off_hours_access` |

**Assertions:**
- Producer throughput ≥ 500 msg/s
- All 5 rules fire at least once
- Parse error rate < 1%

**Sample output:**
```
load test summary
  total messages    : 10000
  produce duration  : 1.23s
  produce rate      : 8130 msg/s
  pipeline processed: 10000
  parse errors      : 0 (0.00%)
  anomaly rule deltas:
    error_rate_spike          +1
    latency_threshold         +1
    auth_failure_burst        +1
    repeated_failure          +1
    off_hours_access          +10
```

---

## 10. Check Application Logs

```bash
# Live stream
docker-compose logs -f app

# Look for specific events
docker-compose logs app | grep -i anomaly
docker-compose logs app | grep -i "error"
docker-compose logs app | grep -i "warmup"
```

---

## 11. Warmup Period Verification

On first startup the detection engine suppresses anomalies for `2 × window_duration = 10 minutes`. To verify:

1. Start the stack fresh (`docker-compose down -v && docker-compose up -d`)
2. Immediately produce 10+ error logs
3. For the first ~10 min: no anomalies should appear in ES or Grafana
4. After 10 min: produce more errors — anomalies should now appear

---

## 12. Cooldown Verification

After an anomaly fires for `(rule, service)`:

1. Note the anomaly in `/api/v1/anomalies`
2. Send another batch of the same trigger logs within 15 minutes
3. No second anomaly should be created for that rule+service pair
4. Wait 15 min, trigger again — new anomaly fires

---

## Quick Reference

| Component | URL |
|---|---|
| REST API | http://localhost:8080 |
| Prometheus metrics | http://localhost:2112/metrics |
| Kibana (log viewer) | http://localhost:5601 |
| Grafana dashboards | http://localhost:3000 (admin/admin123) |
| Prometheus UI | http://localhost:9090 |
| Elasticsearch | http://localhost:9200 |
| Kafka (external) | localhost:9092 |
