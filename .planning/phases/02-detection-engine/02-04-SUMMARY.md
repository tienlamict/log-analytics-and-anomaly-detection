---
phase: 02-detection-engine
plan: "04"
subsystem: infra
tags: [viper, config, yaml, detection, thresholds]

# Dependency graph
requires:
  - phase: 02-detection-engine/02-01
    provides: DetectionConfig type tree (DetectionConfig, DetectionRulesConfig, all per-rule structs in internal/detection/config.go)
provides:
  - Config struct extended with Detection field (detection.DetectionConfig)
  - Viper defaults for all detection parameters in internal/config/config.go
  - Full detection section in config.yaml with all rule parameters operator-tunable
affects: [03-storage-alerting, 04-rest-api, 05-integration-hardening, cmd/server]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config-driven thresholds: all detection parameters loaded from YAML via Viper mapstructure"
    - "Import direction: config imports detection (not vice versa) — no cycle"
    - "Viper defaults before ReadInConfig: zero-config startup with sensible values"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - config.yaml

key-decisions:
  - "Import detection from config (not vice versa) — detection package has no config dependency so no cycle"
  - "No custom DecodeHook needed — Viper's built-in mapstructure.StringToTimeDurationHookFunc handles string->time.Duration"
  - "All six rule configs plus engine-level params (window_duration, eviction_interval, cooldown_duration, warmup_multiplier) surfaced in YAML"

patterns-established:
  - "Detection threshold extension pattern: add field to DetectionConfig, add viper.SetDefault in Load(), add key to config.yaml"

requirements-completed: [DETECT-08, DETECT-09]

# Metrics
duration: 3min
completed: 2026-03-21
---

# Phase 2 Plan 04: Config Integration Summary

**All detection rule thresholds and operational parameters (cooldown, warmup) wired to YAML via Viper defaults and config.yaml — operators tune detection without code changes**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-21T15:40:07Z
- **Completed:** 2026-03-21T15:43:53Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Extended `Config` struct with `Detection detection.DetectionConfig` field enabling full Viper unmarshal of the detection section
- Added 28 `viper.SetDefault` calls covering all detection parameters — application starts without any detection config in YAML
- Appended complete `detection:` section to `config.yaml` with all six rules (error_rate, latency, repeated_failure, auth_burst, off_hours, service_silence) and engine-level settings

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend Config struct and add Viper defaults for detection** - `9726192` (feat)
2. **Task 2: Update config.yaml with full detection section** - `51b4c63` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `internal/config/config.go` - Added `detection` import and `Detection` field to Config struct; added all Viper defaults for detection parameters
- `config.yaml` - Appended full `detection:` YAML section with all rule configurations

## Decisions Made

- Import direction is `config -> detection` (not vice versa). The detection package has no config dependency, so there is no import cycle. This avoids the alternative of moving DetectionConfig types into the config package.
- Viper's built-in `StringToTimeDurationHookFunc` handles `"5m"` -> `time.Duration` automatically during `Unmarshal`. No custom DecodeHook is needed (and adding one incorrectly would override the default).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Detection engine is fully config-driven: thresholds, windows, cooldown, warmup multiplier all loaded from `config.yaml`
- Phase 03 (Storage and Alerting) can safely call `config.Load()` and access `cfg.Detection` for any detection-related parameters
- `go build ./...` confirms zero import cycles across the full module

---
*Phase: 02-detection-engine*
*Completed: 2026-03-21*
