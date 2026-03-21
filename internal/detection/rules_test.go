package detection

import (
	"testing"
	"time"

	"github.com/log-analytics/server/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeEntry builds a domain.LogEntry with the given service, level, and message.
// All other fields are set to zero values unless overridden by calling code.
func makeEntry(service, level, message string) domain.LogEntry {
	return domain.LogEntry{
		Timestamp: time.Now(),
		Service:   service,
		Level:     level,
		Message:   message,
		Fields:    map[string]any{},
	}
}

// makeEntryAt is like makeEntry but lets the caller specify the timestamp.
func makeEntryAt(service, level, message string, ts time.Time) domain.LogEntry {
	return domain.LogEntry{
		Timestamp: ts,
		Service:   service,
		Level:     level,
		Message:   message,
		Fields:    map[string]any{},
	}
}

// ---------------------------------------------------------------------------
// ErrorRateRule tests (DETECT-02)
// ---------------------------------------------------------------------------

func TestErrorRateRule_FiresAtThreshold(t *testing.T) {
	cfg := ErrorRateConfig{
		Threshold: 5,
		Window:    1 * time.Minute,
		Severity:  "high",
	}
	rule := NewErrorRateRule(cfg)
	now := time.Now()

	var lastAnomaly domain.Anomaly
	var fired bool
	for i := 0; i < 5; i++ {
		e := makeEntryAt("svc-a", "error", "something failed", now.Add(-time.Duration(i)*time.Second))
		lastAnomaly, fired = rule.Evaluate(e)
	}

	require.True(t, fired, "expected rule to fire at threshold")
	assert.Equal(t, "error_rate_spike", lastAnomaly.RuleID)
	assert.Equal(t, "svc-a", lastAnomaly.Service)
	assert.Equal(t, "high", lastAnomaly.Severity)
}

func TestErrorRateRule_DoesNotFireBelowThreshold(t *testing.T) {
	cfg := ErrorRateConfig{
		Threshold: 5,
		Window:    1 * time.Minute,
		Severity:  "high",
	}
	rule := NewErrorRateRule(cfg)

	for i := 0; i < 4; i++ {
		_, fired := rule.Evaluate(makeEntry("svc-a", "error", "something failed"))
		assert.False(t, fired, "should not fire below threshold (iteration %d)", i)
	}
}

func TestErrorRateRule_IgnoresNonErrorLevels(t *testing.T) {
	cfg := ErrorRateConfig{Threshold: 1, Window: 1 * time.Minute, Severity: "high"}
	rule := NewErrorRateRule(cfg)

	for _, level := range []string{"info", "warn", "debug"} {
		_, fired := rule.Evaluate(makeEntry("svc-a", level, "something happened"))
		assert.False(t, fired, "level %q should not trigger error rate rule", level)
	}
}

func TestErrorRateRule_PerServiceIsolation(t *testing.T) {
	cfg := ErrorRateConfig{Threshold: 3, Window: 1 * time.Minute, Severity: "high"}
	rule := NewErrorRateRule(cfg)

	// svc-a gets 2 errors (below threshold)
	for i := 0; i < 2; i++ {
		_, fired := rule.Evaluate(makeEntry("svc-a", "error", "err"))
		assert.False(t, fired)
	}

	// svc-b gets 3 errors (at threshold)
	var fired bool
	var anomaly domain.Anomaly
	for i := 0; i < 3; i++ {
		anomaly, fired = rule.Evaluate(makeEntry("svc-b", "error", "err"))
	}
	require.True(t, fired)
	assert.Equal(t, "svc-b", anomaly.Service)
}

func TestErrorRateRule_ResetEvictsStaleEntries(t *testing.T) {
	cfg := ErrorRateConfig{Threshold: 5, Window: 1 * time.Second, Severity: "high"}
	rule := NewErrorRateRule(cfg)
	old := time.Now().Add(-10 * time.Second)

	// Add 4 old entries
	for i := 0; i < 4; i++ {
		rule.Evaluate(makeEntryAt("svc-a", "error", "err", old))
	}
	// Reset evicts them
	rule.Reset()

	// After reset the window should be empty so adding 1 new entry does not fire
	_, fired := rule.Evaluate(makeEntry("svc-a", "error", "err"))
	assert.False(t, fired, "stale entries should have been evicted by Reset")
}

// ---------------------------------------------------------------------------
// LatencyThresholdRule tests (DETECT-03)
// ---------------------------------------------------------------------------

func TestLatencyThresholdRule_FiresAtBreachRate(t *testing.T) {
	cfg := LatencyConfig{
		ThresholdMs:       500,
		BreachRatePercent: 50,
		Window:            1 * time.Minute,
		Severity:          "medium",
	}
	rule := NewLatencyThresholdRule(cfg)

	// 5 entries: 3 breach (600ms), 2 don't (200ms) — 60% breach rate
	for i := 0; i < 3; i++ {
		e := makeEntry("svc", "info", "req")
		e.Fields["duration_ms"] = float64(600)
		rule.Evaluate(e)
	}
	var anomaly domain.Anomaly
	var fired bool
	for i := 0; i < 2; i++ {
		e := makeEntry("svc", "info", "req")
		e.Fields["duration_ms"] = float64(200)
		anomaly, fired = rule.Evaluate(e)
	}
	// After the last entry: 3/5 = 60% >= 50% threshold
	// The rule fires on each evaluate once breach rate is reached; check last call
	_ = anomaly
	// Re-evaluate a breaching entry to confirm it fires
	e := makeEntry("svc", "info", "req")
	e.Fields["duration_ms"] = float64(600)
	anomaly, fired = rule.Evaluate(e)
	require.True(t, fired, "should fire when breach rate >= configured percent")
	assert.Equal(t, "latency_threshold_breach", anomaly.RuleID)
}

func TestLatencyThresholdRule_DoesNotFireBelowRate(t *testing.T) {
	cfg := LatencyConfig{
		ThresholdMs:       500,
		BreachRatePercent: 80, // require 80% breach rate
		Window:            1 * time.Minute,
		Severity:          "medium",
	}
	rule := NewLatencyThresholdRule(cfg)

	// Add 7 non-breaching entries first to establish a baseline, then add 3
	// breaching entries.  Final ratio = 3/10 = 30% — below the 80% threshold.
	for i := 0; i < 7; i++ {
		e := makeEntry("svc", "info", "req")
		e.Fields["duration_ms"] = float64(100)
		rule.Evaluate(e)
	}
	for i := 0; i < 3; i++ {
		e := makeEntry("svc", "info", "req")
		e.Fields["duration_ms"] = float64(600)
		_, fired := rule.Evaluate(e)
		assert.False(t, fired, "30%% breach rate should not reach 80%% threshold (iteration %d)", i)
	}
}

func TestLatencyThresholdRule_SkipsEntriesWithoutDuration(t *testing.T) {
	cfg := LatencyConfig{
		ThresholdMs:       100,
		BreachRatePercent: 50,
		Window:            1 * time.Minute,
		Severity:          "medium",
	}
	rule := NewLatencyThresholdRule(cfg)

	// Entries with no duration field should be silently skipped
	for i := 0; i < 10; i++ {
		e := makeEntry("svc", "info", "req")
		// No duration_ms or latency_ms field
		_, fired := rule.Evaluate(e)
		assert.False(t, fired, "entry without duration field must not fire (iteration %d)", i)
	}
}

func TestLatencyThresholdRule_AcceptsBothFieldNames(t *testing.T) {
	cfg := LatencyConfig{
		ThresholdMs:       100,
		BreachRatePercent: 100, // fire when all entries breach
		Window:            1 * time.Minute,
		Severity:          "low",
	}

	tests := []struct {
		fieldName string
		value     any
	}{
		{"duration_ms", float64(200)},
		{"latency_ms", float64(200)},
		{"duration_ms", "200"},
		{"latency_ms", "200"},
	}

	for _, tc := range tests {
		rule := NewLatencyThresholdRule(cfg)
		e := makeEntry("svc", "info", "req")
		e.Fields[tc.fieldName] = tc.value
		anomaly, fired := rule.Evaluate(e)
		assert.True(t, fired, "field %q with value %v should trigger latency rule", tc.fieldName, tc.value)
		assert.Equal(t, "latency_threshold_breach", anomaly.RuleID)
	}
}

// ---------------------------------------------------------------------------
// RepeatedFailureRule tests (DETECT-04)
// ---------------------------------------------------------------------------

func TestRepeatedFailureRule_FiresOnRepeats(t *testing.T) {
	cfg := RepeatedFailureConfig{Threshold: 3, Window: 1 * time.Minute, Severity: "high"}
	rule := NewRepeatedFailureRule(cfg)

	var anomaly domain.Anomaly
	var fired bool
	for i := 0; i < 3; i++ {
		anomaly, fired = rule.Evaluate(makeEntry("svc", "error", "database connection failed"))
	}
	require.True(t, fired, "expected rule to fire at threshold")
	assert.Equal(t, "repeated_failure", anomaly.RuleID)
}

func TestRepeatedFailureRule_FingerprintNormalization(t *testing.T) {
	cfg := RepeatedFailureConfig{Threshold: 2, Window: 1 * time.Minute, Severity: "high"}
	rule := NewRepeatedFailureRule(cfg)

	// Two messages that differ only in IP address — same fingerprint
	e1 := makeEntry("svc", "error", "connection to 10.0.0.1:5432 failed")
	e2 := makeEntry("svc", "error", "connection to 10.0.0.2:5432 failed")

	_, fired1 := rule.Evaluate(e1)
	assert.False(t, fired1, "first entry should not fire alone")

	_, fired2 := rule.Evaluate(e2)
	assert.True(t, fired2, "second entry with same fingerprint should reach threshold and fire")
}

func TestRepeatedFailureRule_DifferentMessagesDoNotCombine(t *testing.T) {
	cfg := RepeatedFailureConfig{Threshold: 2, Window: 1 * time.Minute, Severity: "high"}
	rule := NewRepeatedFailureRule(cfg)

	messages := []string{
		"disk write error",
		"network timeout",
		"authentication failed",
	}

	// Each message is unique — none should reach threshold of 2
	for _, msg := range messages {
		_, fired := rule.Evaluate(makeEntry("svc", "error", msg))
		assert.False(t, fired, "unique message %q should not fire", msg)
	}
}

func TestRepeatedFailureRule_IgnoresNonErrors(t *testing.T) {
	cfg := RepeatedFailureConfig{Threshold: 1, Window: 1 * time.Minute, Severity: "high"}
	rule := NewRepeatedFailureRule(cfg)

	for _, level := range []string{"info", "warn", "debug"} {
		_, fired := rule.Evaluate(makeEntry("svc", level, "repeated message"))
		assert.False(t, fired, "level %q should not be tracked by repeated failure rule", level)
	}
}

// ---------------------------------------------------------------------------
// AuthFailureBurstRule tests (DETECT-05)
// ---------------------------------------------------------------------------

func TestAuthFailureBurstRule_FiresAtThreshold(t *testing.T) {
	cfg := AuthBurstConfig{
		Threshold: 3,
		Window:    1 * time.Minute,
		IPField:   "source_ip",
		Severity:  "critical",
	}
	rule := NewAuthFailureBurstRule(cfg)

	var anomaly domain.Anomaly
	var fired bool
	for i := 0; i < 3; i++ {
		e := makeEntry("auth-svc", "error", "authentication failed")
		e.Fields["source_ip"] = "192.168.1.100"
		anomaly, fired = rule.Evaluate(e)
	}
	require.True(t, fired)
	assert.Equal(t, "auth_failure_burst", anomaly.RuleID)
	assert.Equal(t, "critical", anomaly.Severity)
}

func TestAuthFailureBurstRule_PerKeyIsolation(t *testing.T) {
	cfg := AuthBurstConfig{
		Threshold: 3,
		Window:    1 * time.Minute,
		IPField:   "source_ip",
		Severity:  "critical",
	}
	rule := NewAuthFailureBurstRule(cfg)

	// IP-A gets 2 failures (below threshold)
	for i := 0; i < 2; i++ {
		e := makeEntry("auth-svc", "error", "login failed")
		e.Fields["source_ip"] = "10.0.0.1"
		_, fired := rule.Evaluate(e)
		assert.False(t, fired)
	}

	// IP-B gets 3 failures (at threshold) — only IP-B fires
	var fired bool
	for i := 0; i < 3; i++ {
		e := makeEntry("auth-svc", "error", "login failed")
		e.Fields["source_ip"] = "10.0.0.2"
		_, fired = rule.Evaluate(e)
	}
	assert.True(t, fired)
}

func TestAuthFailureBurstRule_SkipsNonAuthEntries(t *testing.T) {
	cfg := AuthBurstConfig{
		Threshold: 1,
		Window:    1 * time.Minute,
		IPField:   "source_ip",
		Severity:  "critical",
	}
	rule := NewAuthFailureBurstRule(cfg)

	// Non-auth error entries — message contains no auth keywords
	for i := 0; i < 5; i++ {
		e := makeEntry("svc", "error", "database query failed")
		e.Fields["source_ip"] = "10.0.0.1"
		_, fired := rule.Evaluate(e)
		assert.False(t, fired, "non-auth error entry should not trigger auth burst rule")
	}
}

// ---------------------------------------------------------------------------
// OffHoursAccessRule tests (DETECT-06)
// ---------------------------------------------------------------------------

func TestOffHoursAccessRule_FiresOutsideHours(t *testing.T) {
	cfg := OffHoursConfig{
		BusinessHoursStart: 9,
		BusinessHoursEnd:   17,
		Timezone:           "UTC",
		SensitivePaths:     []string{"/admin", "/internal"},
		Severity:           "high",
	}
	rule, err := NewOffHoursAccessRule(cfg)
	require.NoError(t, err)

	// 3 AM UTC — outside business hours
	ts := time.Date(2026, 3, 21, 3, 0, 0, 0, time.UTC)
	e := makeEntryAt("api-svc", "info", "GET /admin", ts)
	e.Fields["path"] = "/admin"

	anomaly, fired := rule.Evaluate(e)
	require.True(t, fired, "should fire for sensitive path at 3 AM")
	assert.Equal(t, "off_hours_access", anomaly.RuleID)
}

func TestOffHoursAccessRule_DoesNotFireDuringBusinessHours(t *testing.T) {
	cfg := OffHoursConfig{
		BusinessHoursStart: 9,
		BusinessHoursEnd:   17,
		Timezone:           "UTC",
		SensitivePaths:     []string{"/admin"},
		Severity:           "high",
	}
	rule, err := NewOffHoursAccessRule(cfg)
	require.NoError(t, err)

	// 10 AM UTC — within business hours
	ts := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	e := makeEntryAt("api-svc", "info", "GET /admin", ts)
	e.Fields["path"] = "/admin"

	_, fired := rule.Evaluate(e)
	assert.False(t, fired, "should not fire during business hours")
}

func TestOffHoursAccessRule_IgnoresNonSensitivePaths(t *testing.T) {
	cfg := OffHoursConfig{
		BusinessHoursStart: 9,
		BusinessHoursEnd:   17,
		Timezone:           "UTC",
		SensitivePaths:     []string{"/admin"},
		Severity:           "high",
	}
	rule, err := NewOffHoursAccessRule(cfg)
	require.NoError(t, err)

	// 3 AM — but path is /public (not sensitive)
	ts := time.Date(2026, 3, 21, 3, 0, 0, 0, time.UTC)
	e := makeEntryAt("api-svc", "info", "GET /public", ts)
	e.Fields["path"] = "/public"

	_, fired := rule.Evaluate(e)
	assert.False(t, fired, "non-sensitive path should not fire off-hours rule")
}

func TestOffHoursAccessRule_IgnoresEntriesWithoutPath(t *testing.T) {
	cfg := OffHoursConfig{
		BusinessHoursStart: 9,
		BusinessHoursEnd:   17,
		Timezone:           "UTC",
		SensitivePaths:     []string{"/admin"},
		Severity:           "high",
	}
	rule, err := NewOffHoursAccessRule(cfg)
	require.NoError(t, err)

	// 3 AM — but no path field
	ts := time.Date(2026, 3, 21, 3, 0, 0, 0, time.UTC)
	e := makeEntryAt("api-svc", "info", "background task", ts)
	// No path field in Fields

	_, fired := rule.Evaluate(e)
	assert.False(t, fired, "entry without path field should not fire off-hours rule")
}

// ---------------------------------------------------------------------------
// ServiceSilenceRule tests (DETECT-07)
// ---------------------------------------------------------------------------

func TestServiceSilenceRule_FiresAfterSilence(t *testing.T) {
	cfg := ServiceSilenceConfig{
		SilenceAfter:  150 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
		MinLogCount:   3,
		Severity:      "critical",
	}
	rule := NewServiceSilenceRule(cfg)

	// Emit MinLogCount entries to pass the cold-start guard.
	for i := 0; i < 3; i++ {
		rule.Evaluate(makeEntry("svc-silent", "info", "heartbeat"))
	}

	// Wait beyond SilenceAfter.
	time.Sleep(200 * time.Millisecond)

	anomalies := rule.CheckSilence()
	found := false
	for _, a := range anomalies {
		if a.Service == "svc-silent" {
			found = true
			assert.Equal(t, "service_silence", a.RuleID)
		}
	}
	assert.True(t, found, "expected service_silence anomaly after silence duration")
}

func TestServiceSilenceRule_ColdStartGuard(t *testing.T) {
	cfg := ServiceSilenceConfig{
		SilenceAfter:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
		MinLogCount:   5,
		Severity:      "critical",
	}
	rule := NewServiceSilenceRule(cfg)

	// Only 2 entries — below MinLogCount of 5 (cold-start guard active)
	for i := 0; i < 2; i++ {
		rule.Evaluate(makeEntry("new-svc", "info", "startup"))
	}

	// Wait beyond SilenceAfter.
	time.Sleep(150 * time.Millisecond)

	anomalies := rule.CheckSilence()
	for _, a := range anomalies {
		assert.NotEqual(t, "new-svc", a.Service, "cold-start service must not fire silence rule")
	}
}

func TestServiceSilenceRule_DoesNotFireWhileActive(t *testing.T) {
	cfg := ServiceSilenceConfig{
		SilenceAfter:  200 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
		MinLogCount:   3,
		Severity:      "critical",
	}
	rule := NewServiceSilenceRule(cfg)

	// Continuously emit entries over 150ms — service is active throughout.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rule.Evaluate(makeEntry("active-svc", "info", "heartbeat"))
			case <-done:
				return
			}
		}
	}()

	time.Sleep(160 * time.Millisecond)
	close(done)

	anomalies := rule.CheckSilence()
	for _, a := range anomalies {
		assert.NotEqual(t, "active-svc", a.Service, "actively-logging service must not fire silence rule")
	}
}
