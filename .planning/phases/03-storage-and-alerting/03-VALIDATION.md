---
phase: 3
slug: storage-and-alerting
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-21
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test toolchain |
| **Quick run command** | `go test ./internal/elasticsearch/... ./internal/smtp/... ./internal/alert/...` |
| **Full suite command** | `go test -race ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/elasticsearch/... ./internal/smtp/... ./internal/alert/...`
- **After every plan wave:** Run `go test -race ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 03-01 | 1 | STORE-03 | integration | `go test ./internal/elasticsearch/... -run TestIndexTemplates` | ❌ W0 | ⬜ pending |
| 03-02-01 | 03-02 | 1 | STORE-01 | unit | `go test ./internal/elasticsearch/... -run TestLogIndexer` | ❌ W0 | ⬜ pending |
| 03-02-02 | 03-02 | 1 | STORE-02 | unit | `go test ./internal/elasticsearch/... -run TestAnomalyIndexer` | ❌ W0 | ⬜ pending |
| 03-02-03 | 03-02 | 1 | STORE-04 | unit | `go test ./internal/elasticsearch/... -run TestBulkIndexerFailure` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03-03 | 2 | ALERT-01 | unit | `go test ./internal/smtp/... -run TestSMTPAlerter` | ❌ W0 | ⬜ pending |
| 03-03-02 | 03-03 | 2 | ALERT-02 | unit | `go test ./internal/smtp/... -run TestEmailBody` | ❌ W0 | ⬜ pending |
| 03-03-03 | 03-03 | 2 | ALERT-03 | unit | `go test ./internal/smtp/... -run TestSMTPConfig` | ❌ W0 | ⬜ pending |
| 03-04-01 | 03-04 | 3 | ALERT-01 | integration | `go test ./internal/alert/... -run TestDispatcher` | ❌ W0 | ⬜ pending |
| 03-04-02 | 03-04 | 3 | STORE-01,STORE-02 | integration | `go test ./internal/alert/... -run TestFanOut` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/elasticsearch/indexer_test.go` — stubs for STORE-01, STORE-02, STORE-03, STORE-04
- [ ] `internal/smtp/alerter_test.go` — stubs for ALERT-01, ALERT-02, ALERT-03
- [ ] `internal/alert/dispatcher_test.go` — stubs for fan-out integration

*Existing go test infrastructure covers framework setup; only test files need to be created.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ES index template applied on fresh cluster startup | STORE-03 | Requires live ES container | Start docker-compose, run server, verify `GET /_index_template/logs-template` returns 200 |
| SMTP email received by recipient | ALERT-01 | Requires live SMTP server (MailHog) | Configure MailHog in config.yaml, trigger anomaly, check MailHog UI at localhost:8025 |
| SMTP failure does not block log ingestion | STORE-04/ALERT-01 | Requires SMTP server shutdown mid-run | Kill SMTP server during run, verify Kafka consumer continues processing |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
