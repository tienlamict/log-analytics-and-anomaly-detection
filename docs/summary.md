## Tổng quan hệ thống Log Analytics & Anomaly Detection

---

### Các thành phần trong hệ thống

| # | Thành phần | Công nghệ | Port | Memory |
|---|-----------|-----------|------|--------|
| 1 | Message Queue | Apache Kafka (KRaft) | 9092 | 800MB |
| 2 | Storage & Search | Elasticsearch 9.0.0 | 9200 | 2GB |
| 3 | Core Application | Go (chi + franz-go) | 8080 / 2112 | 512MB |
| 4 | Log Viewer | **Kibana 9.0.0** | **5601** | 1GB |
| 5 | Metrics Dashboard | Grafana 12.0.1 | 3000 | 256MB |
| 6 | Metrics Collector | Prometheus 3.4.0 | 9090 | 256MB |
| 7 | Init Containers | kafka-init, kibana-init | — | — |

---

### Vai trò của từng thành phần

#### 1. Kafka — Hàng đợi log đầu vào
- Nhận log JSON từ ứng dụng qua topic `application-logs` (3 partitions)
- Đóng vai trò **buffer** giữa producers và pipeline xử lý
- Đảm bảo **at-least-once delivery** (offset commit sau khi xử lý thành công)
- `kafka-init` tự động tạo topic khi khởi động lần đầu

#### 2. Elasticsearch — Lưu trữ & truy vấn
- **Index `logs-YYYY.MM.DD`**: lưu log theo ngày, mapping cố định (`dynamic: false`)
  - Fields: `@timestamp` (date), `level` (keyword), `service` (keyword), `message` (text), `fields` (flattened)
- **Index `anomalies`**: lưu anomaly phát hiện được
  - Fields: `rule_id`, `severity`, `service`, `description`, `detected_at`, `evidence`
- Document ID log = SHA256(partition:offset) → **idempotent** khi reprocessing
- Bulk indexer: 4 worker, flush mỗi 10MB hoặc 5 giây
- Refresh interval 5s (tăng write throughput 5x so với default 1s)

#### 3. Core Application — Bộ não xử lý

Gồm các module con chạy song song qua `errgroup`:

| Module | Goroutines | Chức năng |
|--------|-----------|----------|
| **Kafka Consumer** | 1 | Poll message từ Kafka → đẩy vào Go channel (buffer 10K) |
| **Pipeline Workers** | 4 | Parse JSON → normalize level → index log → chạy detection |
| **Detection Engine** | 1 (eviction) + 1 (heartbeat) | Chạy 6 rules, quản lý warmup/cooldown |
| **Anomaly Dispatcher** | 1 | Nhận anomaly → ghi ES + gửi alert channel |
| **REST API Server** | 1 | Expose /api/v1/logs và /api/v1/anomalies |
| **Metrics Server** | 1 | Expose Prometheus metrics trên port 2112 |

**6 Detection Rules:**

| Rule | Điều kiện | Severity | Cơ chế |
|------|----------|----------|--------|
| **Error Rate Spike** | ≥10 error/service trong 5 phút | HIGH | Sliding window per-service |
| **Latency Threshold** | ≥20% request vượt 500ms trong 5 phút | MEDIUM | Circular buffer per-service |
| **Repeated Failure** | Cùng fingerprint lỗi ≥5 lần trong 5 phút | MEDIUM | Message fingerprinting (strip numbers/hex) |
| **Auth Failure Burst** | ≥10 auth fail từ cùng IP trong 5 phút | HIGH | Track by source_ip, fallback username |
| **Off-Hours Access** | Truy cập `/admin`, `/internal` ngoài 9h-17h UTC | MEDIUM | Stateless — fire ngay lập tức |
| **Service Silence** | Service ngừng gửi log >5 phút (sau ≥5 log ban đầu) | CRITICAL | Heartbeat check mỗi 30s |

**Cơ chế bảo vệ:**
- **Warmup**: 10 phút sau khi app start, không fire anomaly (tránh false positive)
- **Cooldown**: 15 phút per (rule + service), ngăn spam alert trùng lặp
- **Eviction**: Mỗi 30s dọn dẹp state cũ ngoài window

#### 4. Kibana — Xem log trực quan (ELK)
- Giao diện web tại `http://localhost:5601` — không cần login
- 2 data views tự động tạo bởi `kibana-init`:
  - **Application Logs** (`logs-*`) — browse log entries, filter bằng KQL
  - **Anomalies** (`anomalies`) — browse anomaly đã phát hiện
- Hỗ trợ search bằng KQL: `level : "error"`, `service : "payment-service" and fields.source_ip : "192.168.1.99"`

#### 5. Grafana — Dashboard metrics
- Login: `admin / admin123` tại `http://localhost:3000`
- Datasources: **Prometheus** (metrics) + **Elasticsearch** (logs/anomalies)
- Dashboard **"Log Analytics & Anomaly Detection"**: ingestion rate, processing latency (p50/p95/p99), anomaly rate, consumer lag, ES write errors
- Dashboard **"Log Explorer"**: log volume, log stream, anomaly timeline & table

#### 6. Prometheus — Thu thập metrics
- Scrape app mỗi 15 giây tại `app:2112/metrics`
- Metrics: `logs_consumed_total`, `logs_processed_duration_seconds`, `anomalies_detected_total`, `kafka_consumer_lag`, `elasticsearch_write_errors_total`, `parse_errors_total`

---

### Luồng hoạt động End-to-End

Ví dụ: Auth service ghi log đăng nhập thất bại liên tục từ IP `10.66.6.6`

```
 [App gửi log JSON vào Kafka]
 {"timestamp":"2026-04-12T14:00:00Z","level":"error","service":"auth-service",
  "message":"authentication failure: invalid credentials",
  "fields":{"source_ip":"10.66.6.6","username":"admin","auth_result":"failure"}}
                    |
                    v
 +-----------------+------------------+
 |           KAFKA                     |
 |  topic: application-logs            |
 |  3 partitions, round-robin          |
 +----------------+-------------------+
                  |
                  v
 +----------------+-------------------+
 |        KAFKA CONSUMER               |
 |  franz-go poll -> Go channel        |
 |  buffer: 10,000 messages            |
 |  commit offset after channel send   |
 +-------+--------+--------+----------+
         |        |        |       (4 pipeline workers lấy từ channel)
         v        v        v
 +-------+--------+--------+----------+
 |        PIPELINE WORKER              |
 |                                     |
 |  1. Parse JSON -> LogEntry          |
 |     level normalized: "error"       |
 |     fields extracted                |
 |                                     |
 |  2. Index Log -> ES                 |
 |     index: logs-2026.04.12          |
 |     docID: sha256(partition:offset) |
 |     -> bulk queue (flush 10MB/5s)   |
 |                                     |
 |  3. Detection Engine.Evaluate()     |
 |     [x] error_rate_spike: count++   |
 |     [x] auth_failure_burst:         |
 |         IP=10.66.6.6, count=11      |
 |         -> THRESHOLD MET (>=10)     |
 |         -> cooldown check: PASS     |
 |         -> warmup check: PASS       |
 |         -> FIRE ANOMALY!            |
 |     [x] repeated_failure: check...  |
 |     [x] latency: no latency_ms     |
 |     [x] off_hours: within hours    |
 |     [x] service_silence: update ts  |
 +----------------+-------------------+
                  |
                  v  Anomaly{
                  |    rule_id: "auth_failure_burst",
                  |    severity: "high",
                  |    service: "auth-service",
                  |    description: "auth failures from 10.66.6.6"
                  |  }
                  v
 +----------------+-------------------+
 |        ANOMALY DISPATCHER           |
 |                                     |
 |  1. AnomalyIndexer.IndexAnomaly()  |
 |     -> ES index "anomalies"         |
 |     -> docID: UUID                  |
 |                                     |
 |  2. AlertChannel.Send()             |
 |     -> (nil — extensible for       |
 |        Slack/email/PagerDuty)       |
 +----------------+-------------------+
                  |
                  v
 +----------------+-------------------+
 |        ELASTICSEARCH                |
 |                                     |
 |  logs-2026.04.12:  11,000+ docs    |
 |  anomalies:        auth_failure_burst|
 +---+----------+----------+----------+
     |          |          |
     v          v          v
 +---+---+ +---+---+ +----+----+
 |  API  | |Kibana | |Grafana  |
 | :8080 | | :5601 | |  :3000  |
 +---+---+ +---+---+ +----+----+
     |          |          |
     v          v          v
  curl/code  Discover   Dashboard
  GET /api/  KQL search  metrics +
  v1/logs    log stream  log explorer
  v1/anomalies
```

**Kết quả quan sát được:**

| Nơi xem | Cách xem |
|---------|---------|
| **API** | `curl localhost:8080/api/v1/anomalies?type=auth_failure_burst&service=auth-service` |
| **Kibana** | Discover → data view "Anomalies" → filter `rule_id : "auth_failure_burst"` |
| **Grafana** | Dashboard "Log Explorer" → Anomalies table hiện severity=HIGH |
| **Prometheus** | `anomalies_detected_total{rule="auth_failure_burst"}` tăng lên 1 |

**Testing tools:**

| Tool | Lệnh | Vai trò |
|------|------|---------|
| `cmd/loadtest` | `go run ./cmd/loadtest --rate 500` | Đẩy log vào Kafka (5 phút, 7 phase) |
| `cmd/e2etest` | `go run ./cmd/e2etest --rate 100` | Full E2E: đẩy log + verify ES + verify anomaly + report PASS/FAIL |