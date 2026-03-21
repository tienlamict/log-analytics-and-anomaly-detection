# Log Analytics and Anomaly Detection

## What This Is

An event-driven log analysis and anomaly detection system built in Go. It consumes application logs from a message queue (Kafka), applies rule-based detection logic to identify anomalies (error spikes, behavioral drift, security events), persists logs and anomalies to Elasticsearch, and exposes a REST API to query them. Anomaly alerts are sent via email and stored for historical review.

## Core Value

Detect anomalies in application logs in real-time and surface them via alerts and a queryable API — so problems are caught before users report them.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Consume application logs from a message queue (Kafka)
- [ ] Parse and normalize structured and unstructured log entries
- [ ] Apply rule-based anomaly detection: error rate spikes, latency thresholds, repeated failures within a time window
- [ ] Detect security-relevant patterns: auth failures, unusual access patterns
- [ ] Persist raw logs and detected anomalies to Elasticsearch
- [ ] Send email alerts when anomalies are detected
- [ ] Expose REST API for querying logs and anomalies
- [ ] End-to-end pipeline: ingest → process → detect → alert → store

### Out of Scope

- Slack / generic webhook alerting — deferred to v2
- Statistical / ML-based anomaly detection — rule-based is sufficient for v1
- Dashboard UI — REST API is the interface for v1
- Log shipper / agent — services push to Kafka directly
- Multi-tenancy — single-system scope for v1

## Context

- **Language**: Go — idiomatic Go with interfaces for extensibility
- **Message queue**: Kafka (recommended for event-driven architecture; consumer group model fits well)
- **Storage**: Elasticsearch — used for both raw log storage and anomaly records; supports full-text search and time-range queries
- **Scale**: Low volume (<1k logs/sec) — design for correctness and clarity over raw throughput
- **Log types**: Application logs from HTTP services, gRPC services, background workers
- **Anomaly categories**: Error spikes, latency/behavioral drift, security events (auth failures, IP anomalies)
- **Alerting**: Email via SMTP for v1; anomalies also persisted to Elasticsearch for historical queries

## Constraints

- **Tech stack**: Go — no Python, Node.js, or other runtimes in the core pipeline
- **Storage**: Elasticsearch only — no relational DB in v1
- **Scale**: Designed for <1k logs/sec; horizontal scaling is a future concern
- **Alerting v1**: Email + Elasticsearch persistence only; webhook/Slack are v2

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Kafka as message queue | Natural fit for event-driven architecture; consumer groups allow multiple processors; supports replay | — Pending |
| Rule-based detection only | Predictable, debuggable, sufficient for known anomaly patterns in v1 | — Pending |
| Elasticsearch for storage | Supports full-text log search + time-range anomaly queries in one store | — Pending |
| Email alerts for v1 | Simplest reliable channel; webhook/Slack added when routing logic is clearer | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-03-21 after initialization*
