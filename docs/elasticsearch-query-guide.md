# Elasticsearch Query Guide

## Indices

| Index | Description |
|---|---|
| `logs-YYYY.MM.DD` | Log entries (daily rolling) |
| `anomalies` | Detected anomalies |

## Field Reference

### `logs-*`

| Field | Type | Values |
|---|---|---|
| `@timestamp` | date | RFC3339 |
| `level` | keyword | `error`, `warn`, `info`, `debug`, `unknown` |
| `service` | keyword | `api-gateway`, `payment-service`, `auth-service`, `order-service`, `inventory-service`, `notification-service` |
| `message` | text | full-text searchable |
| `raw_source` | keyword | original raw payload |
| `fields` | flattened | `fields.source_ip`, `fields.latency_ms`, `fields.path`, `fields.username`, `fields.auth_result` |

### `anomalies`

| Field | Type | Values |
|---|---|---|
| `id` | keyword | UUID |
| `rule_id` | keyword | `error_rate_spike`, `auth_failure_burst`, `latency_threshold`, `repeated_failure`, `off_hours_access`, `service_silence` |
| `severity` | keyword | `low`, `medium`, `high`, `critical` |
| `service` | keyword | same as logs |
| `description` | text | human-readable summary |
| `detected_at` | date | RFC3339 |
| `evidence` | flattened | log entries that triggered the rule |

---

## Queries

### 1. Peek at recent logs

```bash
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "size": 5,
  "sort": [{ "@timestamp": "desc" }],
  "_source": ["@timestamp", "level", "service", "message"]
}'
```

---

### 2. Filter by level and service

```bash
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "bool": {
      "must": [
        { "term": { "level": "error" } },
        { "term": { "service": "payment-service" } }
      ]
    }
  },
  "sort": [{ "@timestamp": "desc" }],
  "size": 20
}'
```

---

### 3. Full-text search on message

```bash
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "match": { "message": "authentication failure" }
  },
  "size": 10
}'
```

---

### 4. Time range query

```bash
# Last 5 minutes
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "range": {
      "@timestamp": { "gte": "now-5m", "lte": "now" }
    }
  },
  "sort": [{ "@timestamp": "desc" }],
  "size": 10
}'

# Absolute range
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "range": {
      "@timestamp": {
        "gte": "2026-04-08T00:00:00Z",
        "lte": "2026-04-08T23:59:59Z"
      }
    }
  },
  "size": 10
}'
```

---

### 5. Query nested `fields` (flattened — use dot notation with `term`)

```bash
# By source IP
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "term": { "fields.source_ip": "10.66.6.6" }
  },
  "size": 10
}'

# By auth result
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "term": { "fields.auth_result": "failure" }
  },
  "size": 10
}'
```

> **Note:** `fields` uses the `flattened` type. Nested keys are queryable with dot notation but only via `term`/`terms` — range queries (e.g. `fields.latency_ms > 500`) do not work. To enable range queries on nested fields, promote them to top-level mapped fields in the index template.

---

### 6. Aggregation — error count per service (last hour)

```bash
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "size": 0,
  "query": {
    "bool": {
      "must": [
        { "term": { "level": "error" } },
        { "range": { "@timestamp": { "gte": "now-1h" } } }
      ]
    }
  },
  "aggs": {
    "errors_per_service": {
      "terms": { "field": "service", "size": 20 }
    }
  }
}'
```

---

### 7. Aggregation — log volume over time (1-minute buckets)

```bash
curl -s "http://localhost:9200/logs-*/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "size": 0,
  "aggs": {
    "logs_over_time": {
      "date_histogram": {
        "field": "@timestamp",
        "fixed_interval": "1m"
      },
      "aggs": {
        "by_level": {
          "terms": { "field": "level" }
        }
      }
    }
  }
}'
```

---

### 8. Query anomalies

```bash
# All anomalies sorted by time
curl -s "http://localhost:9200/anomalies/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "sort": [{ "detected_at": "desc" }],
  "size": 10
}'

# Filter by rule
curl -s "http://localhost:9200/anomalies/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "term": { "rule_id": "auth_failure_burst" }
  }
}'

# Filter by severity
curl -s "http://localhost:9200/anomalies/_search?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "bool": {
      "must": [
        { "term": { "rule_id": "auth_failure_burst" } },
        { "term": { "severity": "high" } }
      ]
    }
  }
}'
```

---

### 9. Count documents

```bash
curl -s "http://localhost:9200/logs-*/_count" | python -m json.tool
curl -s "http://localhost:9200/anomalies/_count" | python -m json.tool
```

---

### 10. Delete logs older than 7 days

```bash
curl -s -X POST "http://localhost:9200/logs-*/_delete_by_query?pretty" \
  -H "Content-Type: application/json" -d '{
  "query": {
    "range": {
      "@timestamp": { "lt": "now-7d" }
    }
  }
}'
```
