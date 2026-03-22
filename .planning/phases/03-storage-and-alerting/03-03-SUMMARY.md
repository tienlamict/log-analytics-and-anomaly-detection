---
phase: 03-storage-and-alerting
plan: "03"
subsystem: alerting
tags: [go-mail, smtp, email, async, prometheus, zap]

# Dependency graph
requires:
  - phase: 03-storage-and-alerting
    provides: domain.AlertChannel interface, metrics.EmailAlertsSentTotal and EmailAlertsFailedTotal counters, SMTPConfig struct in internal/config

provides:
  - SMTPAlerter implementing domain.AlertChannel via async buffered dispatch (internal/smtp/alerter.go)
  - formatAlertBody producing plain-text ALERT-02 compliant email body
  - parseTLSPolicy mapping config strings to go-mail TLSPolicy constants
  - go-mail v0.7.2 dependency added to go.mod

affects:
  - 03-04 (alert dispatcher wires SMTPAlerter.Send into Dispatcher)
  - 04 (REST API phase — no dependency on smtp)
  - 05 (integration phase wires SMTPAlerter.Run in main.go errgroup)

# Tech tracking
tech-stack:
  added:
    - github.com/wneessen/go-mail v0.7.2 (SMTP email dispatch, context-aware DialAndSendWithContext)
  patterns:
    - Buffered chan domain.Anomaly (capacity 100) decouples detection hot path from SMTP latency
    - Send enqueues non-blocking; Run goroutine drains and dispatches
    - Failures logged and metered via EmailAlertsFailedTotal; never propagated
    - Conditional auth options (skip WithSMTPAuth/Username/Password when Username is empty, for MailHog)
    - WithPort + WithTLSPolicy pattern (not WithTLSPortPolicy) when port is explicit in config

key-files:
  created:
    - internal/smtp/alerter.go
    - internal/smtp/alerter_test.go
  modified:
    - go.mod (added go-mail v0.7.2)
    - go.sum

key-decisions:
  - "WithPort + WithTLSPolicy (not WithTLSPortPolicy) used because port is always explicit in SMTPConfig — avoids auto-port-selection side effect"
  - "Conditional SMTP auth options allow MailHog-style local testing without credentials"
  - "Queue drop is non-fatal (nil error) with metric increment — prevents back-pressure into dispatcher"

patterns-established:
  - "Pattern: Buffered async queue for external I/O bound alerters — Send enqueues, Run drains"
  - "Pattern: TLS policy parsed from config string to typed constant at construction time, not per-send"

requirements-completed: [ALERT-01, ALERT-02, ALERT-03]

# Metrics
duration: 10min
completed: 2026-03-22
---

# Phase 3 Plan 03: SMTP Alerter Summary

**SMTPAlerter using go-mail v0.7.2 with buffered async dispatch, ALERT-02 rich email body (type, service, severity, detection time, description, evidence capped at 5 lines), and TLS policy parsed from config string**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-22T01:48:00Z
- **Completed:** 2026-03-22T01:58:00Z
- **Tasks:** 1
- **Files modified:** 4 (alerter.go, alerter_test.go, go.mod, go.sum)

## Accomplishments

- SMTPAlerter.Send enqueues to buffered channel and returns immediately (non-blocking)
- Background Run goroutine drains queue and calls DialAndSendWithContext per anomaly
- Email body includes all ALERT-02 required fields with evidence capped at 5 lines
- TLS configurable via "mandatory", "opportunistic", or "none" string in config
- 7 unit tests covering formatAlertBody, parseTLSPolicy, Send enqueue, queue-full drop — all pass under -race

## Task Commits

Each task was committed atomically:

1. **Task 1: SMTPAlerter with async dispatch, email formatting, and tests** - `d70cae5` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `internal/smtp/alerter.go` - SMTPAlerter struct, NewSMTPAlerter, Send, Run, dispatch, formatAlertBody, parseTLSPolicy
- `internal/smtp/alerter_test.go` - Unit tests for all exported and internal-package functions
- `go.mod` - Added github.com/wneessen/go-mail v0.7.2
- `go.sum` - Updated checksums

## Decisions Made

- Used `WithPort` + `WithTLSPolicy` (not `WithTLSPortPolicy`) because the port is always explicit in SMTPConfig — avoids unwanted port auto-selection side effects per Research Pitfall 6.
- Made SMTP auth options conditional: when `cfg.Username` is empty, auth options are omitted, allowing MailHog/no-auth local SMTP to work without config changes.
- Queue full drop returns `nil` error (non-fatal) to prevent the dispatcher from failing on a transient SMTP buffer pressure event.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added missing `go.uber.org/zap` import in test file**
- **Found during:** Task 1 (test compilation)
- **Issue:** `noopLogger` helper used `zap.NewDevelopment()` but `zap` was not imported in the test file
- **Fix:** Added `"go.uber.org/zap"` to test file imports
- **Files modified:** internal/smtp/alerter_test.go
- **Verification:** `go test ./internal/smtp/...` compiles and all tests pass
- **Committed in:** d70cae5 (part of task commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor import omission; corrected inline during compilation. No scope changes.

## Issues Encountered

A pre-existing build error exists in `internal/elasticsearch/setup.go` (from parallel plan 03-01/03-02 execution): `cannot call non-function client.Indices`. This is out-of-scope for this plan. The smtp package itself builds, vets, and tests cleanly in isolation (`go build ./internal/smtp/... && go vet ./internal/smtp/...`).

## User Setup Required

None - no external service configuration required for the smtp package itself. SMTP server (host, port, credentials) is provided via `SMTPConfig` at runtime wiring in plan 03-04/main.go.

## Next Phase Readiness

- SMTPAlerter is ready to be injected into the Dispatcher in plan 03-04
- `SMTPAlerter.Run(ctx)` must be started in an errgroup goroutine in main.go (plan 03-04 or Phase 5 wiring)
- No blockers for the rest of Phase 3

## Self-Check: PASSED

- internal/smtp/alerter.go: FOUND
- internal/smtp/alerter_test.go: FOUND
- .planning/phases/03-storage-and-alerting/03-03-SUMMARY.md: FOUND
- Commit d70cae5: FOUND

---
*Phase: 03-storage-and-alerting*
*Completed: 2026-03-22*
