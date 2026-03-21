package detection

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/log-analytics/server/internal/domain"
)

// latencyEntry records a single request with its timestamp and whether it
// breached the configured latency threshold.
type latencyEntry struct {
	timestamp time.Time
	breached  bool
}

// latencyWindow is a fixed-capacity circular buffer of latencyEntry values.
// It is analogous to slidingWindow but tracks breach information per entry.
type latencyWindow struct {
	entries  []latencyEntry
	head     int
	count    int
}

// newLatencyWindow allocates a latencyWindow with the given capacity.
func newLatencyWindow(capacity int) *latencyWindow {
	return &latencyWindow{entries: make([]latencyEntry, capacity)}
}

// Add records a new entry, overwriting the oldest entry when the buffer is full.
func (w *latencyWindow) Add(t time.Time, breached bool) {
	w.entries[w.head%len(w.entries)] = latencyEntry{timestamp: t, breached: breached}
	w.head++
	if w.count < len(w.entries) {
		w.count++
	}
}

// StatsWithin returns the total number of requests and the number of breaching
// requests whose timestamp falls within the last d duration. All entries are
// scanned because Kafka delivery order is not guaranteed monotonic.
func (w *latencyWindow) StatsWithin(d time.Duration) (total int, breaches int) {
	cutoff := time.Now().Add(-d)
	for i := 0; i < w.count; i++ {
		idx := (w.head - 1 - i + len(w.entries)) % len(w.entries)
		e := w.entries[idx]
		if e.timestamp.After(cutoff) {
			total++
			if e.breached {
				breaches++
			}
		}
	}
	return total, breaches
}

// Evict removes entries older than olderThan, compacting in place.
func (w *latencyWindow) Evict(olderThan time.Duration) {
	cutoff := time.Now().Add(-olderThan)
	newEntries := w.entries[:0]
	for i := 0; i < w.count; i++ {
		idx := (w.head - w.count + i + len(w.entries)) % len(w.entries)
		if w.entries[idx].timestamp.After(cutoff) {
			newEntries = append(newEntries, w.entries[idx])
		}
	}
	copy(w.entries, newEntries)
	w.count = len(newEntries)
	w.head = w.count
}

// LatencyThresholdRule fires when the percentage of requests exceeding the
// latency threshold breaches the configured breach rate within the window.
type LatencyThresholdRule struct {
	windows  map[string]*latencyWindow
	cfg      LatencyConfig
	capacity int
}

// NewLatencyThresholdRule constructs a LatencyThresholdRule with the given configuration.
func NewLatencyThresholdRule(cfg LatencyConfig) *LatencyThresholdRule {
	return &LatencyThresholdRule{
		windows:  make(map[string]*latencyWindow),
		cfg:      cfg,
		capacity: 10000,
	}
}

// Name implements domain.Detector.
func (r *LatencyThresholdRule) Name() string {
	return "latency_threshold_breach"
}

// Evaluate implements domain.Detector. It extracts duration_ms (or latency_ms)
// from the log entry fields and fires when the breach rate percentage within
// the window exceeds the configured threshold.
func (r *LatencyThresholdRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
	// Extract duration from either "duration_ms" or "latency_ms" field.
	var durationMs float64
	found := false
	for _, key := range []string{"duration_ms", "latency_ms"} {
		if v, ok := entry.Fields[key]; ok {
			switch val := v.(type) {
			case float64:
				durationMs = val
				found = true
			case string:
				if parsed, err := strconv.ParseFloat(val, 64); err == nil {
					durationMs = parsed
					found = true
				}
			case int:
				durationMs = float64(val)
				found = true
			case int64:
				durationMs = float64(val)
				found = true
			}
			if found {
				break
			}
		}
	}
	if !found {
		return domain.Anomaly{}, false
	}

	breached := durationMs >= float64(r.cfg.ThresholdMs)

	w, ok := r.windows[entry.Service]
	if !ok {
		w = newLatencyWindow(r.capacity)
		r.windows[entry.Service] = w
	}

	w.Add(entry.Timestamp, breached)

	total, breaches := w.StatsWithin(r.cfg.Window)
	if total == 0 {
		return domain.Anomaly{}, false
	}

	breachRate := (breaches * 100) / total
	if breachRate >= r.cfg.BreachRatePercent {
		return domain.Anomaly{
			ID:          uuid.NewString(),
			RuleID:      r.Name(),
			Severity:    r.cfg.Severity,
			Service:     entry.Service,
			Description: fmt.Sprintf("latency breach: %d%% of %d requests exceeded %dms in %s", breachRate, total, r.cfg.ThresholdMs, r.cfg.Window),
			Evidence:    []domain.LogEntry{entry},
			DetectedAt:  time.Now(),
		}, true
	}

	return domain.Anomaly{}, false
}

// Reset implements domain.Detector. It evicts stale entries from each service
// window without clearing the entire state.
func (r *LatencyThresholdRule) Reset() {
	for _, w := range r.windows {
		w.Evict(r.cfg.Window)
	}
}
