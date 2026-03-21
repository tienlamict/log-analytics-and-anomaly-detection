# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-21)

**Core value:** Detect anomalies in application logs in real-time and surface them via alerts and a queryable API — so problems are caught before users report them.
**Current focus:** Ready for Phase 1 planning

---

## Current Status

**Phase:** Pre-execution — project initialized, ready to plan Phase 1
**Milestone:** v1.0.0
**Overall progress:** 0 / 5 phases complete

| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation and Ingestion | Pending |
| 2 | Detection Engine | Pending |
| 3 | Storage and Alerting | Pending |
| 4 | REST API | Pending |
| 5 | Integration and Hardening | Pending |

---

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Kafka via franz-go v1.20.7 | Pure Go, no CGo, actively maintained, first-class Prometheus/Zap plugins |
| Elasticsearch v9 (go-elasticsearch/v9) | TypedClient for compile-time checked schemas, native OTel, current major |
| chi v5 for HTTP | 100% net/http compatible, lightweight, no framework lock-in |
| go-mail v0.7.2 for SMTP | Security-patched (GO-2025-3988 fixed in v0.7.1), context-aware |
| Rule-based detection only in v1 | Predictable, debuggable, sufficient for known anomaly patterns |
| Config-driven thresholds (YAML) | Operators tune without redeployment |
| 7 detection rules in v1 | Error spike, latency breach, repeated failure, auth burst, off-hours access, service silence, dedup/cooldown |

---

## Open Questions

- What log format do target application services emit? (JSON structured vs logfmt vs unstructured) — affects parser complexity
- Is `duration_ms` / `latency_ms` reliably present in logs? Required for latency breach rule (DETECT-03)
- What is the expected cardinality of services? (handful vs hundreds) — affects per-service rule config structure

---

## Session Continuity

**Last session:** 2026-03-21 — project initialized
**Next action:** `/gsd:plan-phase 1`

---

## Todos

(None yet)

---
*Last updated: 2026-03-21 after project initialization*
