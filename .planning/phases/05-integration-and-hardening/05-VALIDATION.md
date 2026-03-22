---
phase: 5
slug: integration-and-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-22
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — build tags gate integration tests |
| **Quick run command** | `go test -race ./internal/integration/... -tags integration -count=1` |
| **Full suite command** | `go test -race -tags integration ./...` |
| **Estimated runtime** | ~90 seconds (container startup ~30s + tests) |

---

## Sampling Rate

- **After every task commit:** Run `go build ./...` (compilation gate)
- **After every plan wave:** Run `go test -race ./internal/integration/... -tags integration -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green: `go test -race -tags integration ./...`
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | TEST-02 | build | `go build ./...` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | TEST-02 | integration | `go test -tags integration -run TestLogIngestionEndToEnd ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | TEST-02 | integration | `go test -tags integration -run TestAnomalyDetectionEndToEnd ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 05-01-04 | 01 | 1 | TEST-02 | integration | `go test -tags integration -run TestOffsetCommitOnRestart ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 05-02-01 | 02 | 1 | TEST-02 | build | `go build ./cmd/server/...` | ❌ W0 | ⬜ pending |
| 05-02-02 | 02 | 1 | TEST-02 | integration | `go test -tags integration -run TestShutdownTiming ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 05-02-03 | 02 | 1 | TEST-02 | integration | `go test -tags integration -run TestNoGoroutineLeaks ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 05-03-01 | 03 | 2 | TEST-02 | build | `go build ./cmd/server/...` | ❌ W0 | ⬜ pending |
| 05-03-02 | 03 | 2 | TEST-02 | integration | `go test -race -tags integration ./...` | ❌ W0 | ⬜ pending |
| 05-03-03 | 03 | 2 | — | manual | verify README instructions work end-to-end | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/integration/` directory and `integration_test.go` stub with `//go:build integration` tag
- [ ] `TestMain` scaffold with container startup/teardown
- [ ] `go get github.com/testcontainers/testcontainers-go/modules/kafka@v0.41.0`
- [ ] `go get github.com/testcontainers/testcontainers-go/modules/elasticsearch@v0.41.0`

*Existing go.mod includes testify, goleak, franz-go, go-elasticsearch — no additional framework installs beyond testcontainers modules.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| README end-to-end developer onboarding | TEST-02 | Requires human to follow instructions on a clean machine | Clone repo, follow README from scratch: install deps, run `docker compose up`, run unit + integration tests |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
