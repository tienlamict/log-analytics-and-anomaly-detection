package detection

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/log-analytics/server/internal/domain"
)

// OffHoursAccessRule fires when an HTTP request to a sensitive path is processed
// outside of configured business hours. It is stateless: every qualifying event
// fires an anomaly immediately with no window or threshold.
type OffHoursAccessRule struct {
	cfg OffHoursConfig
	loc *time.Location
}

// NewOffHoursAccessRule creates an OffHoursAccessRule, loading the timezone from
// cfg.Timezone. An empty or "UTC" timezone uses time.UTC. Returns an error if
// the timezone string is invalid.
func NewOffHoursAccessRule(cfg OffHoursConfig) (*OffHoursAccessRule, error) {
	var loc *time.Location
	if cfg.Timezone == "" || cfg.Timezone == "UTC" {
		loc = time.UTC
	} else {
		var err error
		loc, err = time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("off_hours_access: invalid timezone %q: %w", cfg.Timezone, err)
		}
	}
	return &OffHoursAccessRule{cfg: cfg, loc: loc}, nil
}

// Name returns the rule identifier.
func (r *OffHoursAccessRule) Name() string {
	return "off_hours_access"
}

// isSensitivePath reports whether path starts with any of the configured sensitive
// path prefixes.
func (r *OffHoursAccessRule) isSensitivePath(path string) bool {
	for _, prefix := range r.cfg.SensitivePaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// Evaluate fires when entry.Fields["path"] matches a sensitive path prefix AND the
// event timestamp falls outside of configured business hours in the configured
// timezone.
func (r *OffHoursAccessRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
	path, ok := entry.Fields["path"].(string)
	if !ok || path == "" {
		return domain.Anomaly{}, false
	}

	if !r.isSensitivePath(path) {
		return domain.Anomaly{}, false
	}

	hour := entry.Timestamp.In(r.loc).Hour()
	if hour >= r.cfg.BusinessHoursStart && hour < r.cfg.BusinessHoursEnd {
		return domain.Anomaly{}, false // within business hours
	}

	return domain.Anomaly{
		ID:          uuid.New().String(),
		RuleID:      "off_hours_access",
		Severity:    r.cfg.Severity,
		Service:     entry.Service,
		Description: fmt.Sprintf("off-hours access to %s at %02d:00 by service %s", path, hour, entry.Service),
		Evidence:    []domain.LogEntry{entry},
		DetectedAt:  time.Now(),
	}, true
}

// Reset is a no-op because OffHoursAccessRule is stateless.
func (r *OffHoursAccessRule) Reset() {}
