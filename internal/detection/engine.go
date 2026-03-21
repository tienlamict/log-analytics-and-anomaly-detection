package detection

import (
	"time"

	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// cooldownKey identifies the (rule, service) pair used to suppress repeated alerts.
type cooldownKey struct {
	ruleID  string
	service string
}

// DetectorEngine dispatches log entries to a registered set of domain.Detector
// implementations, manages warm-up suppression, per-(rule,service) cooldown, and
// runs a background eviction goroutine to keep detector state bounded.
type DetectorEngine struct {
	detectors    []domain.Detector
	cooldowns    map[cooldownKey]time.Time
	cooldownDur  time.Duration
	warmupUntil  time.Time
	logger       *zap.Logger
	out          chan domain.Anomaly
	evictStop    chan struct{}
}

// maxWindowDuration returns the largest window / silence-after duration across all
// enabled rule configurations. This is used to compute the warmup period.
func maxWindowDuration(cfg DetectionConfig) time.Duration {
	max := cfg.WindowDuration
	update := func(d time.Duration) {
		if d > max {
			max = d
		}
	}
	update(cfg.Rules.ErrorRate.Window)
	update(cfg.Rules.Latency.Window)
	update(cfg.Rules.RepeatedFailure.Window)
	update(cfg.Rules.AuthBurst.Window)
	update(cfg.Rules.ServiceSilence.SilenceAfter)
	return max
}

// NewDetectorEngine creates a DetectorEngine, wires all provided detectors, computes
// the warmup period, and starts the background eviction goroutine.
func NewDetectorEngine(detectors []domain.Detector, cfg DetectionConfig, logger *zap.Logger) *DetectorEngine {
	maxWindow := maxWindowDuration(cfg)
	warmupMultiplier := cfg.WarmupMultiplier
	if warmupMultiplier <= 0 {
		warmupMultiplier = 2 // default per Pitfall A1
	}

	e := &DetectorEngine{
		detectors:   detectors,
		cooldowns:   make(map[cooldownKey]time.Time),
		cooldownDur: cfg.CooldownDuration,
		warmupUntil: time.Now().Add(time.Duration(warmupMultiplier) * maxWindow),
		logger:      logger,
		out:         make(chan domain.Anomaly, 256),
		evictStop:   make(chan struct{}),
	}

	go e.evictLoop(cfg.EvictionInterval)
	return e
}

// Evaluate dispatches entry to all registered detectors and emits any resulting
// anomalies on the output channel.
//
// Single-goroutine-caller contract: Evaluate must be called from a single goroutine
// (the pipeline worker). The engine struct contains no mutex because all writes to
// cooldowns and reads from warmupUntil occur exclusively in this goroutine. The
// evictLoop goroutine calls only d.Reset() on each detector; channel-based
// coordination between Evaluate and evictLoop will be added in Plan 02-04 if a
// race detector finding surfaces.
func (e *DetectorEngine) Evaluate(entry domain.LogEntry) {
	if time.Now().Before(e.warmupUntil) {
		return // suppress all anomaly output during warm-up period
	}

	for _, d := range e.detectors {
		anomaly, fired := d.Evaluate(entry)
		if !fired {
			continue
		}

		key := cooldownKey{ruleID: anomaly.RuleID, service: anomaly.Service}
		if last, ok := e.cooldowns[key]; ok && time.Since(last) < e.cooldownDur {
			continue // suppress: same (rule, service) fired too recently
		}

		e.cooldowns[key] = time.Now()
		metrics.AnomaliesDetectedTotal.WithLabelValues(anomaly.RuleID).Inc()

		select {
		case e.out <- anomaly:
		default:
			e.logger.Warn("anomaly channel full, dropping anomaly",
				zap.String("rule_id", anomaly.RuleID),
				zap.String("service", anomaly.Service),
			)
		}
	}
}

// Anomalies returns a read-only channel that emits detected anomalies.
func (e *DetectorEngine) Anomalies() <-chan domain.Anomaly {
	return e.out
}

// Stop signals the background eviction goroutine to exit cleanly.
func (e *DetectorEngine) Stop() {
	close(e.evictStop)
}

// evictLoop runs on a ticker and periodically calls Reset on all registered
// detectors to evict stale window entries, and removes stale cooldown entries
// to prevent unbounded memory growth.
func (e *DetectorEngine) evictLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, d := range e.detectors {
				d.Reset() // Reset evicts stale entries — does NOT fully clear state
			}
			// Evict cooldown entries that are well past their cooldown window.
			for key, last := range e.cooldowns {
				if time.Since(last) > e.cooldownDur*2 {
					delete(e.cooldowns, key)
				}
			}
		case <-e.evictStop:
			return
		}
	}
}
