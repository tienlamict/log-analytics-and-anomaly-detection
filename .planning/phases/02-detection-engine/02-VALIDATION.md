---
phase: 2
slug: detection-engine
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-21
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing `go.mod` covers all dependencies |
| **Quick run command** | `go test ./internal/detection/... -count=1` |
| **Full suite command** | `go test -race ./internal/detection/... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/detection/... -count=1`
- **After every plan wave:** Run `go test -race ./internal/detection/... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 02-01 | 1 | DETECT-01 | unit | `go test ./internal/detection/... -run TestDetectorEngine` | ❌ W0 | ⬜ pending |
| 2-01-02 | 02-01 | 1 | DETECT-10 | unit | `go test ./internal/detection/... -run TestWindowEviction` | ❌ W0 | ⬜ pending |
| 2-02-01 | 02-02 | 2 | DETECT-02 | unit | `go test ./internal/detection/... -run TestErrorRateRule` | ❌ W0 | ⬜ pending |
| 2-02-02 | 02-02 | 2 | DETECT-03 | unit | `go test ./internal/detection/... -run TestLatencyThresholdRule` | ❌ W0 | ⬜ pending |
| 2-02-03 | 02-02 | 2 | DETECT-04 | unit | `go test ./internal/detection/... -run TestRepeatedFailureRule` | ❌ W0 | ⬜ pending |
| 2-03-01 | 02-03 | 2 | DETECT-05 | unit | `go test ./internal/detection/... -run TestAuthFailureBurstRule` | ❌ W0 | ⬜ pending |
| 2-03-02 | 02-03 | 2 | DETECT-06 | unit | `go test ./internal/detection/... -run TestOffHoursAccessRule` | ❌ W0 | ⬜ pending |
| 2-03-03 | 02-03 | 2 | DETECT-07 | unit | `go test ./internal/detection/... -run TestServiceSilenceRule` | ❌ W0 | ⬜ pending |
| 2-04-01 | 02-04 | 3 | DETECT-08 | unit | `go test ./internal/detection/... -run TestCooldown` | ❌ W0 | ⬜ pending |
| 2-04-02 | 02-04 | 3 | DETECT-09 | unit | `go test ./internal/... -run TestConfig` | ❌ W0 | ⬜ pending |
| 2-05-01 | 02-05 | 4 | DETECT-01..10 | unit | `go test -race ./internal/detection/... -count=1 -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/detection/engine_test.go` — stubs for DETECT-01, DETECT-10
- [ ] `internal/detection/rules_test.go` — stubs for DETECT-02..DETECT-07
- [ ] `internal/detection/cooldown_test.go` — stubs for DETECT-08
- [ ] `internal/detection/config_test.go` — stubs for DETECT-09

*Existing `go test` infrastructure in `internal/` covers all mechanics; only stub files need creating.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| YAML config change at restart changes rule behavior | DETECT-09 | Integration — requires process restart | 1. Set `error_rate.threshold: 5` in config.yaml. 2. Start server. 3. Inject 6 error logs for same service in window. 4. Observe alert fires. 5. Change threshold to 10. 6. Restart. 7. Same 6 errors should NOT fire. |
| Warm-up period suppresses all alerts | DETECT-08/A1 | Timing-dependent | Start process, inject logs triggering all 7 rules within first `2×maxWindow` seconds — zero alerts should appear in output. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
