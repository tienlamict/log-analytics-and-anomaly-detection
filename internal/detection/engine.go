package detection

import (
	"sync"
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
// If a *ServiceSilenceRule is registered, a separate heartbeat goroutine runs
// CheckSilence on the configured interval.
type DetectorEngine struct {
	detectors   []domain.Detector
	cooldowns   map[cooldownKey]time.Time
	cooldownMu  sync.Mutex // guards cooldowns map (written by Evaluate/silenceWatcher, iterated by evictLoop)
	cooldownDur time.Duration
	warmupUntil time.Time
	logger      *zap.Logger
	out         chan domain.Anomaly
	evictStop   chan struct{}
	silenceRule *ServiceSilenceRule // non-nil when a ServiceSilenceRule is registered
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

	// Wire silence rule heartbeat: type-assert each detector for *ServiceSilenceRule.
	for _, d := range detectors {
		if sr, ok := d.(*ServiceSilenceRule); ok {
			e.silenceRule = sr
			go e.silenceWatcher(sr)
			break // only one silence rule is expected
		}
	}

	go e.evictLoop(cfg.EvictionInterval)
	return e
}

// Evaluate dispatches entry to all registered detectors and emits any resulting
// anomalies on the output channel.
//
// Evaluate is safe to call from multiple goroutines: the cooldowns map is
// protected by cooldownMu. Calls to d.Evaluate() on individual detectors are
// not mutex-protected — detectors are expected to manage their own concurrency
// if needed (e.g. ServiceSilenceRule uses its own mutex).
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

		e.cooldownMu.Lock()
		last, ok := e.cooldowns[key]
		suppress := ok && time.Since(last) < e.cooldownDur
		if !suppress {
			e.cooldowns[key] = time.Now()
		}
		e.cooldownMu.Unlock()

		if suppress {
			continue // suppress: same (rule, service) fired too recently
		}

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
			e.cooldownMu.Lock()
			for key, last := range e.cooldowns {
				if time.Since(last) > e.cooldownDur*2 {
					delete(e.cooldowns, key)
				}
			}
			e.cooldownMu.Unlock()
		case <-e.evictStop:
			return
		}
	}
}

// silenceWatcher is a heartbeat goroutine that periodically calls CheckSilence on
// the given ServiceSilenceRule and emits resulting anomalies to the output channel.
// It respects the engine's cooldown mechanism and stops when evictStop is closed.
func (e *DetectorEngine) silenceWatcher(sr *ServiceSilenceRule) {
	ticker := time.NewTicker(sr.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().Before(e.warmupUntil) {
				continue // suppress during warmup
			}
			anomalies := sr.CheckSilence()
			for _, anomaly := range anomalies {
				key := cooldownKey{ruleID: anomaly.RuleID, service: anomaly.Service}

				e.cooldownMu.Lock()
				last, ok := e.cooldowns[key]
				suppress := ok && time.Since(last) < e.cooldownDur
				if !suppress {
					e.cooldowns[key] = time.Now()
				}
				e.cooldownMu.Unlock()

				if suppress {
					continue // suppress: cooldown still active
				}

				metrics.AnomaliesDetectedTotal.WithLabelValues(anomaly.RuleID).Inc()

				select {
				case e.out <- anomaly:
				default:
					e.logger.Warn("anomaly channel full, dropping silence anomaly",
						zap.String("rule_id", anomaly.RuleID),
						zap.String("service", anomaly.Service),
					)
				}
			}
		case <-e.evictStop:
			return
		}
	}
}
