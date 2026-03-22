---
phase: 4
slug: rest-api
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-22
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./internal/api/... -count=1` |
| **Full suite command** | `go test -race ./internal/api/... -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/... -count=1`
- **After every plan wave:** Run `go test -race ./internal/api/... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 4-01-01 | 01 | 1 | API-01 | integration | `go test ./internal/api/... -run TestRouter -count=1` | ❌ W0 | ⬜ pending |
| 4-01-02 | 01 | 1 | API-05 | integration | `go test ./internal/api/... -run TestHealth -count=1` | ❌ W0 | ⬜ pending |
| 4-01-03 | 01 | 1 | API-06 | integration | `go test ./internal/api/... -run TestMetrics -count=1` | ❌ W0 | ⬜ pending |
| 4-02-01 | 02 | 2 | API-02 | unit | `go test ./internal/api/... -run TestGetLogs -count=1` | ❌ W0 | ⬜ pending |
| 4-02-02 | 02 | 2 | API-02 | unit | `go test ./internal/api/... -run TestGetLogByID -count=1` | ❌ W0 | ⬜ pending |
| 4-03-01 | 03 | 2 | API-03 | unit | `go test ./internal/api/... -run TestGetAnomalies -count=1` | ❌ W0 | ⬜ pending |
| 4-03-02 | 03 | 2 | API-04 | unit | `go test ./internal/api/... -run TestGetAnomalyByID -count=1` | ❌ W0 | ⬜ pending |
| 4-04-01 | 04 | 3 | API-02,API-03 | unit | `go test -race ./internal/api/... -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/handler_test.go` — test stubs for all handler tests (API-02 through API-06)
- [ ] `internal/api/mock_store_test.go` — mock `LogStore` and `AnomalyStore` implementations for testing
- [ ] `internal/domain/errors.go` — `ErrNotFound` sentinel (required for 404 vs 500 distinction)

*Wave 0 creates test file scaffolding and the missing domain sentinel before implementation begins.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `/health` returns 503 when ES unreachable | API-05 | Requires a running ES instance to be stopped | Start app, stop ES container, `curl -s http://localhost:8080/health` → expect 503 |
| `/metrics` contains custom counters | API-06 | Requires running Prometheus integration | Start app, ingest some logs, `curl -s http://localhost:8080/metrics \| grep logs_consumed_total` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
