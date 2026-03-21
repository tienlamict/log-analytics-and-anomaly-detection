package detection

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/log-analytics/server/internal/domain"
)

// fingerprintNoise strips hex strings (4+ chars), UUIDs, and decimal numbers
// from log messages to produce a stable fingerprint for the same error class.
var fingerprintNoise = regexp.MustCompile(`\b[\da-fA-F]{4,}\b|\b\d+\b`)

// fingerprint normalizes a log message by lower-casing it and replacing all
// numeric and hex tokens with "X", yielding a stable error class identifier.
func fingerprint(msg string) string {
	return fingerprintNoise.ReplaceAllLiteralString(strings.ToLower(msg), "X")
}

// RepeatedFailureRule fires when the same error class (identified by message
// fingerprint) repeats more than the configured threshold within the window
// for a given service.
type RepeatedFailureRule struct {
	windows  map[string]*slidingWindow // keyed by "service|fingerprint"
	cfg      RepeatedFailureConfig
	capacity int
}

// NewRepeatedFailureRule constructs a RepeatedFailureRule with the given configuration.
func NewRepeatedFailureRule(cfg RepeatedFailureConfig) *RepeatedFailureRule {
	return &RepeatedFailureRule{
		windows:  make(map[string]*slidingWindow),
		cfg:      cfg,
		capacity: 10000,
	}
}

// Name implements domain.Detector.
func (r *RepeatedFailureRule) Name() string {
	return "repeated_failure"
}

// Evaluate implements domain.Detector. It fingerprints the log message and
// tracks per-(service, fingerprint) counts in sliding windows. An Anomaly is
// fired when the repeat count within the window reaches the threshold.
func (r *RepeatedFailureRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
	if entry.Level != "error" {
		return domain.Anomaly{}, false
	}

	fp := fingerprint(entry.Message)
	key := entry.Service + "|" + fp

	w, ok := r.windows[key]
	if !ok {
		w = newSlidingWindow(r.capacity)
		r.windows[key] = w
	}

	w.Add(entry.Timestamp)

	count := w.CountWithin(r.cfg.Window)
	if count >= r.cfg.Threshold {
		return domain.Anomaly{
			ID:          uuid.NewString(),
			RuleID:      r.Name(),
			Severity:    r.cfg.Severity,
			Service:     entry.Service,
			Description: fmt.Sprintf("repeated failure: fingerprint %q seen %d times in %s for service %s", fp, count, r.cfg.Window, entry.Service),
			Evidence:    []domain.LogEntry{entry},
			DetectedAt:  time.Now(),
		}, true
	}

	return domain.Anomaly{}, false
}

// Reset implements domain.Detector. It evicts stale entries from each window
// and removes map entries where the window is now empty to prevent unbounded
// growth for transient error fingerprints.
func (r *RepeatedFailureRule) Reset() {
	for key, w := range r.windows {
		w.Evict(r.cfg.Window)
		if w.count == 0 {
			delete(r.windows, key)
		}
	}
}
