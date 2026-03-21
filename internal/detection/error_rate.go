package detection

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/log-analytics/server/internal/domain"
)

// ErrorRateRule fires when the number of error-level log entries for a service
// exceeds the configured threshold within the sliding window duration.
type ErrorRateRule struct {
	windows  map[string]*slidingWindow
	cfg      ErrorRateConfig
	capacity int
}

// NewErrorRateRule constructs an ErrorRateRule with the given configuration.
func NewErrorRateRule(cfg ErrorRateConfig) *ErrorRateRule {
	return &ErrorRateRule{
		windows:  make(map[string]*slidingWindow),
		cfg:      cfg,
		capacity: 10000,
	}
}

// Name implements domain.Detector.
func (r *ErrorRateRule) Name() string {
	return "error_rate_spike"
}

// Evaluate implements domain.Detector. It records error-level entries and fires
// an Anomaly when the per-service count within the window reaches the threshold.
func (r *ErrorRateRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
	if entry.Level != "error" {
		return domain.Anomaly{}, false
	}

	w, ok := r.windows[entry.Service]
	if !ok {
		w = newSlidingWindow(r.capacity)
		r.windows[entry.Service] = w
	}

	w.Add(entry.Timestamp)

	count := w.CountWithin(r.cfg.Window)
	if count >= r.cfg.Threshold {
		return domain.Anomaly{
			ID:          uuid.NewString(),
			RuleID:      r.Name(),
			Severity:    r.cfg.Severity,
			Service:     entry.Service,
			Description: fmt.Sprintf("error rate spike: %d errors in %s for service %s", count, r.cfg.Window, entry.Service),
			Evidence:    []domain.LogEntry{entry},
			DetectedAt:  time.Now(),
		}, true
	}

	return domain.Anomaly{}, false
}

// Reset implements domain.Detector. It evicts stale entries from each service
// window without clearing the entire state (preserves recent events).
func (r *ErrorRateRule) Reset() {
	for _, w := range r.windows {
		w.Evict(r.cfg.Window)
	}
}
