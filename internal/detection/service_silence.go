package detection

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/log-analytics/server/internal/domain"
)

// ServiceSilenceRule detects when a previously-active service stops emitting logs.
// It uses the adapter pattern: Evaluate only updates tracking state and always
// returns false; CheckSilence is called by the engine's heartbeat goroutine and
// returns anomalies for services that have gone silent past the configured threshold.
//
// A sync.Mutex is required because Evaluate is called from the pipeline goroutine
// while CheckSilence is called from the heartbeat goroutine.
type ServiceSilenceRule struct {
	lastSeen  map[string]time.Time // service -> ingest time of most recent observed log
	seenCount map[string]int       // service -> total logs seen (cold-start guard)
	cfg       ServiceSilenceConfig
	mu        sync.Mutex
}

// NewServiceSilenceRule creates a ServiceSilenceRule with the given config.
func NewServiceSilenceRule(cfg ServiceSilenceConfig) *ServiceSilenceRule {
	return &ServiceSilenceRule{
		lastSeen:  make(map[string]time.Time),
		seenCount: make(map[string]int),
		cfg:       cfg,
	}
}

// Name returns the rule identifier.
func (r *ServiceSilenceRule) Name() string {
	return "service_silence"
}

// Evaluate records that a log has been seen for the service. Once the service
// accumulates at least MinLogCount observations, it begins tracking ingest time
// for silence detection. Evaluate always returns false — anomalies are produced
// only by CheckSilence.
//
// NOTE: lastSeen is updated with time.Now() (not entry.Timestamp) because we track
// INGEST time for silence detection, not event time.
func (r *ServiceSilenceRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seenCount[entry.Service]++
	if r.seenCount[entry.Service] >= r.cfg.MinLogCount {
		r.lastSeen[entry.Service] = time.Now()
	}

	return domain.Anomaly{}, false
}

// CheckSilence scans tracked services and returns anomalies for those that have
// not emitted a log within the configured SilenceAfter duration. A service is
// removed from tracking after it fires; it re-enters tracking once new logs arrive
// and exceed MinLogCount again.
func (r *ServiceSilenceRule) CheckSilence() []domain.Anomaly {
	r.mu.Lock()
	defer r.mu.Unlock()

	var anomalies []domain.Anomaly
	for service, lastTime := range r.lastSeen {
		elapsed := time.Since(lastTime)
		if elapsed > r.cfg.SilenceAfter {
			anomalies = append(anomalies, domain.Anomaly{
				ID:      uuid.New().String(),
				RuleID:  "service_silence",
				Severity: r.cfg.Severity,
				Service:  service,
				Description: fmt.Sprintf(
					"service silence: %s has not emitted logs for %s (threshold: %s)",
					service, elapsed.Round(time.Second), r.cfg.SilenceAfter,
				),
				Evidence:   nil,
				DetectedAt: time.Now(),
			})
			// Remove so the service must re-accumulate MinLogCount before re-tracking.
			delete(r.lastSeen, service)
		}
	}
	return anomalies
}

// Reset evicts services from both maps whose last-seen time is more than 3×
// SilenceAfter in the past. This prevents unbounded memory growth for services
// that were tracked but have gone permanently quiet. It does NOT fully clear state.
func (r *ServiceSilenceRule) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	staleThreshold := r.cfg.SilenceAfter * 3
	for service, lastTime := range r.lastSeen {
		if time.Since(lastTime) > staleThreshold {
			delete(r.lastSeen, service)
			delete(r.seenCount, service)
		}
	}
}
