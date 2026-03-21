---
phase: 1
slug: foundation-and-ingestion
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-21
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./internal/pipeline/... -race` |
| **Full suite command** | `go test -race ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pipeline/... -race`
- **After every plan wave:** Run `go test -race ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01-01 | 1 | INGEST-01 | build | `go build ./...` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01-01 | 1 | INGEST-01 | compile | `go vet ./internal/domain/...` | ❌ W0 | ⬜ pending |
| 1-02-01 | 01-02 | 2 | INGEST-02 | build | `go build ./internal/kafka/...` | ❌ W0 | ⬜ pending |
| 1-02-02 | 01-02 | 2 | INGEST-03 | build | `go build ./internal/kafka/...` | ❌ W0 | ⬜ pending |
| 1-02-03 | 01-02 | 2 | INGEST-04 | build | `go build ./internal/kafka/...` | ❌ W0 | ⬜ pending |
| 1-03-01 | 01-03 | 2 | PARSE-01 | unit | `go test -race ./internal/pipeline/...` | ❌ W0 | ⬜ pending |
| 1-03-02 | 01-03 | 2 | PARSE-02 | unit | `go test -race ./internal/pipeline/...` | ❌ W0 | ⬜ pending |
| 1-03-03 | 01-03 | 2 | PARSE-03 | unit | `go test -race ./internal/pipeline/...` | ❌ W0 | ⬜ pending |
| 1-03-04 | 01-03 | 2 | PARSE-04 | unit | `go test -race ./internal/pipeline/...` | ❌ W0 | ⬜ pending |
| 1-04-01 | 01-04 | 2 | OBS-01 | build | `go build ./cmd/...` | ❌ W0 | ⬜ pending |
| 1-04-02 | 01-04 | 2 | OBS-02 | manual | see manual verifications | N/A | ⬜ pending |
| 1-05-01 | 01-05 | 3 | TEST-01 | unit | `go test -race ./internal/pipeline/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/domain/` — package scaffold with all interfaces and types
- [ ] `internal/pipeline/parser_test.go` — test file stubs for PARSE-01 through PARSE-04, TEST-01
- [ ] `go.mod` + `go.sum` — module initialisation required before any build commands

*All test files depend on the module scaffold being created in Wave 1 (plan 01-01). Wave 0 for this phase is therefore the scaffold plan itself — test stubs are created alongside the domain types.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Consumer commits offsets only after downstream confirmation | INGEST-03 | Requires live Kafka broker and process kill/restart | Start consumer against Kafka, send N messages, kill mid-stream, restart, verify no messages lost or double-processed by checking consumer group offset |
| `kafka_consumer_lag` metric reported correctly | OBS-02 | Requires live Kafka broker with consumer group | Produce messages faster than consumer processes them; curl `/metrics`, confirm `kafka_consumer_lag` gauge is non-zero and decreases as consumer catches up |
| `/metrics` endpoint returns all registered metrics | OBS-01 | Requires running HTTP server | `curl http://localhost:2112/metrics` must include `logs_consumed_total`, `logs_processed_duration_seconds`, `anomalies_detected_total`, `elasticsearch_write_errors_total`, `email_alerts_sent_total`, `email_alerts_failed_total`, `kafka_consumer_lag` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
