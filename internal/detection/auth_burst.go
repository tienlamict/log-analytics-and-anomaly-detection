package detection

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/log-analytics/server/internal/domain"
)

// AuthFailureBurstRule fires when the number of authentication failure events
// for a single IP address or username exceeds a configurable threshold within a
// sliding time window. It is keyed per-IP (preferred) or per-username fallback.
type AuthFailureBurstRule struct {
	windows  map[string]*slidingWindow
	cfg      AuthBurstConfig
	capacity int
}

// NewAuthFailureBurstRule creates an AuthFailureBurstRule with the given config.
// If IPField or UserField are empty, they default to "source_ip" and "username".
func NewAuthFailureBurstRule(cfg AuthBurstConfig) *AuthFailureBurstRule {
	if cfg.IPField == "" {
		cfg.IPField = "source_ip"
	}
	if cfg.UserField == "" {
		cfg.UserField = "username"
	}
	return &AuthFailureBurstRule{
		windows:  make(map[string]*slidingWindow),
		cfg:      cfg,
		capacity: 10000,
	}
}

// Name returns the rule identifier.
func (r *AuthFailureBurstRule) Name() string {
	return "auth_failure_burst"
}

// isAuthFailure reports whether entry represents an authentication failure event.
// It matches: level=="error" AND (message contains "auth"/"login"/"unauthorized"
// case-insensitively, OR Fields["auth_result"]=="failure").
func (r *AuthFailureBurstRule) isAuthFailure(entry domain.LogEntry) bool {
	if entry.Level != "error" {
		return false
	}
	msgLower := strings.ToLower(entry.Message)
	if strings.Contains(msgLower, "auth") ||
		strings.Contains(msgLower, "login") ||
		strings.Contains(msgLower, "unauthorized") {
		return true
	}
	if v, ok := entry.Fields["auth_result"]; ok {
		if s, ok := v.(string); ok && s == "failure" {
			return true
		}
	}
	return false
}

// Evaluate checks whether the current entry pushes the per-key auth failure count
// past the configured threshold within the configured window duration.
func (r *AuthFailureBurstRule) Evaluate(entry domain.LogEntry) (domain.Anomaly, bool) {
	if !r.isAuthFailure(entry) {
		return domain.Anomaly{}, false
	}

	// Determine the key: prefer IP, fall back to username.
	var key string
	if v, ok := entry.Fields[r.cfg.IPField]; ok {
		if s, ok := v.(string); ok && s != "" {
			key = s
		}
	}
	if key == "" {
		if v, ok := entry.Fields[r.cfg.UserField]; ok {
			if s, ok := v.(string); ok && s != "" {
				key = s
			}
		}
	}
	if key == "" {
		return domain.Anomaly{}, false
	}

	w, ok := r.windows[key]
	if !ok {
		w = newSlidingWindow(r.capacity)
		r.windows[key] = w
	}

	w.Add(entry.Timestamp)
	count := w.CountWithin(r.cfg.Window)
	if count < r.cfg.Threshold {
		return domain.Anomaly{}, false
	}

	return domain.Anomaly{
		ID:          uuid.New().String(),
		RuleID:      "auth_failure_burst",
		Severity:    r.cfg.Severity,
		Service:     entry.Service,
		Description: fmt.Sprintf("auth failure burst: %d failures from %s in %s", count, key, r.cfg.Window),
		Evidence:    []domain.LogEntry{entry},
		DetectedAt:  time.Now(),
	}, true
}

// Reset evicts stale window entries for all tracked keys and removes keys that
// have no remaining entries. This prevents unbounded memory growth.
func (r *AuthFailureBurstRule) Reset() {
	for key, w := range r.windows {
		w.Evict(r.cfg.Window)
		if w.CountWithin(r.cfg.Window) == 0 {
			delete(r.windows, key)
		}
	}
}
