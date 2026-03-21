package detection

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/log-analytics/server/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Mock detector
// ---------------------------------------------------------------------------

// mockDetector uses atomic counters for evaluated and resetCount because the
// evictLoop goroutine calls Reset() concurrently with the test goroutine
// reading resetCount. Using atomic ops avoids a data race without a mutex.
type mockDetector struct {
	name       string
	fireNext   bool
	anomaly    domain.Anomaly
	evaluated  atomic.Int64
	resetCount atomic.Int64
}

func (m *mockDetector) Name() string { return m.name }

func (m *mockDetector) Evaluate(_ domain.LogEntry) (domain.Anomaly, bool) {
	m.evaluated.Add(1)
	if m.fireNext {
		return m.anomaly, true
	}
	return domain.Anomaly{}, false
}

func (m *mockDetector) Reset() { m.resetCount.Add(1) }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// drainAnomalies reads from ch until the timeout fires and returns all collected
// anomalies. It is safe to call with a nil or closed channel.
func drainAnomalies(ch <-chan domain.Anomaly, timeout time.Duration) []domain.Anomaly {
	var results []domain.Anomaly
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case a := <-ch:
			results = append(results, a)
		case <-timer.C:
			return results
		}
	}
}

// minimalCfg returns a DetectionConfig suitable for engine tests: no warmup,
// short eviction interval, and short cooldown. WarmupMultiplier<=0 is handled
// by the engine as "use default 2"; we bypass warmup by setting WindowDuration
// to 0 so warmupUntil = Now + 2*0 = Now (already in the past).
func minimalCfg() DetectionConfig {
	return DetectionConfig{
		WindowDuration:   0, // warmup = WarmupMultiplier * 0 = Now (bypass)
		EvictionInterval: 500 * time.Millisecond,
		CooldownDuration: 1 * time.Second,
		WarmupMultiplier: 1,
	}
}

// ---------------------------------------------------------------------------
// TestDetectorEngine_EvaluatesAllDetectors (DETECT-01)
// ---------------------------------------------------------------------------

func TestDetectorEngine_EvaluatesAllDetectors(t *testing.T) {
	d1 := &mockDetector{name: "d1"}
	d2 := &mockDetector{name: "d2"}
	d3 := &mockDetector{name: "d3"}

	detectors := []domain.Detector{d1, d2, d3}
	logger := zap.NewNop()
	cfg := minimalCfg()

	engine := NewDetectorEngine(detectors, cfg, logger)
	t.Cleanup(engine.Stop)

	engine.Evaluate(makeEntry("svc", "info", "ping"))

	assert.Equal(t, int64(1), d1.evaluated.Load(), "d1 should have been called once")
	assert.Equal(t, int64(1), d2.evaluated.Load(), "d2 should have been called once")
	assert.Equal(t, int64(1), d3.evaluated.Load(), "d3 should have been called once")
}

// ---------------------------------------------------------------------------
// TestDetectorEngine_Cooldown (DETECT-08)
// ---------------------------------------------------------------------------

func TestDetectorEngine_Cooldown(t *testing.T) {
	anomaly := domain.Anomaly{
		RuleID:  "test_rule",
		Service: "svc",
	}
	d := &mockDetector{name: "test_rule", fireNext: true, anomaly: anomaly}

	cfg := minimalCfg()
	cfg.CooldownDuration = 500 * time.Millisecond
	engine := NewDetectorEngine([]domain.Detector{d}, cfg, zap.NewNop())
	t.Cleanup(engine.Stop)

	// First evaluate — should emit an anomaly.
	engine.Evaluate(makeEntry("svc", "error", "err"))
	// Second evaluate immediately — should be suppressed by cooldown.
	engine.Evaluate(makeEntry("svc", "error", "err"))

	first := drainAnomalies(engine.Anomalies(), 100*time.Millisecond)
	require.Len(t, first, 1, "exactly one anomaly should be emitted; second suppressed by cooldown")

	// Wait for cooldown to expire, then re-evaluate.
	time.Sleep(600 * time.Millisecond)
	engine.Evaluate(makeEntry("svc", "error", "err"))

	second := drainAnomalies(engine.Anomalies(), 100*time.Millisecond)
	require.Len(t, second, 1, "anomaly should be emitted again after cooldown expires")
}

// ---------------------------------------------------------------------------
// TestDetectorEngine_WarmupSuppression (DETECT-08 / Pitfall A1)
// ---------------------------------------------------------------------------

func TestDetectorEngine_WarmupSuppression(t *testing.T) {
	anomaly := domain.Anomaly{
		RuleID:  "warmup_rule",
		Service: "svc",
	}
	d := &mockDetector{name: "warmup_rule", fireNext: true, anomaly: anomaly}

	// WarmupMultiplier=2, WindowDuration=100ms → warmup = 200ms
	cfg := DetectionConfig{
		WindowDuration:   100 * time.Millisecond,
		EvictionInterval: 500 * time.Millisecond,
		CooldownDuration: 50 * time.Millisecond,
		WarmupMultiplier: 2,
	}

	engine := NewDetectorEngine([]domain.Detector{d}, cfg, zap.NewNop())
	t.Cleanup(engine.Stop)

	// Call immediately — still in warmup window.
	engine.Evaluate(makeEntry("svc", "error", "err"))

	// Channel should be empty: Evaluate returns early without calling detectors.
	immediate := drainAnomalies(engine.Anomalies(), 50*time.Millisecond)
	assert.Len(t, immediate, 0, "no anomalies expected during warmup period")
	assert.Equal(t, int64(0), d.evaluated.Load(), "detectors should not be called during warmup")

	// Wait for warmup to expire.
	time.Sleep(250 * time.Millisecond)

	engine.Evaluate(makeEntry("svc", "error", "err"))
	postWarmup := drainAnomalies(engine.Anomalies(), 100*time.Millisecond)
	assert.Len(t, postWarmup, 1, "anomaly should be emitted after warmup period expires")
}

// ---------------------------------------------------------------------------
// TestDetectorEngine_StaleWindowEviction (DETECT-10)
// ---------------------------------------------------------------------------

func TestDetectorEngine_StaleWindowEviction(t *testing.T) {
	d1 := &mockDetector{name: "d1"}
	d2 := &mockDetector{name: "d2"}

	cfg := DetectionConfig{
		WindowDuration:   0,
		EvictionInterval: 100 * time.Millisecond, // short interval for test
		CooldownDuration: 50 * time.Millisecond,
		WarmupMultiplier: 1,
	}

	engine := NewDetectorEngine([]domain.Detector{d1, d2}, cfg, zap.NewNop())
	defer engine.Stop()

	// Wait for at least one eviction tick.
	time.Sleep(200 * time.Millisecond)

	assert.GreaterOrEqual(t, d1.resetCount.Load(), int64(1), "eviction loop should have called d1.Reset at least once")
	assert.GreaterOrEqual(t, d2.resetCount.Load(), int64(1), "eviction loop should have called d2.Reset at least once")
}

// ---------------------------------------------------------------------------
// TestDetectorEngine_StopCleansUp
// ---------------------------------------------------------------------------

func TestDetectorEngine_StopCleansUp(t *testing.T) {
	d := &mockDetector{name: "d"}
	engine := NewDetectorEngine([]domain.Detector{d}, minimalCfg(), zap.NewNop())

	// Stop must not panic; goroutine leak is validated by goleak.VerifyTestMain.
	assert.NotPanics(t, engine.Stop)
}

// ---------------------------------------------------------------------------
// TestDetectorEngine_ConfigDriven (DETECT-09)
// ---------------------------------------------------------------------------

func TestDetectorEngine_ConfigDriven(t *testing.T) {
	makeAlwaysFire := func(ruleName string) *mockDetector {
		return &mockDetector{
			name:     ruleName,
			fireNext: true,
			anomaly:  domain.Anomaly{RuleID: ruleName, Service: "svc"},
		}
	}

	t.Run("short cooldown allows re-fire quickly", func(t *testing.T) {
		d := makeAlwaysFire("rule_short")
		cfg := minimalCfg()
		cfg.CooldownDuration = 50 * time.Millisecond

		engine := NewDetectorEngine([]domain.Detector{d}, cfg, zap.NewNop())
		t.Cleanup(engine.Stop)

		engine.Evaluate(makeEntry("svc", "error", "err"))
		drainAnomalies(engine.Anomalies(), 30*time.Millisecond) // drain first

		time.Sleep(80 * time.Millisecond) // cooldown expired

		engine.Evaluate(makeEntry("svc", "error", "err"))
		after := drainAnomalies(engine.Anomalies(), 50*time.Millisecond)
		assert.Len(t, after, 1, "short cooldown: re-fire should succeed after 80ms")
	})

	t.Run("long cooldown suppresses re-fire", func(t *testing.T) {
		d := makeAlwaysFire("rule_long")
		cfg := minimalCfg()
		cfg.CooldownDuration = 2 * time.Second

		engine := NewDetectorEngine([]domain.Detector{d}, cfg, zap.NewNop())
		t.Cleanup(engine.Stop)

		engine.Evaluate(makeEntry("svc", "error", "err"))
		drainAnomalies(engine.Anomalies(), 50*time.Millisecond) // drain first

		// Immediately re-evaluate — cooldown (2s) not expired yet.
		engine.Evaluate(makeEntry("svc", "error", "err"))
		second := drainAnomalies(engine.Anomalies(), 50*time.Millisecond)
		assert.Len(t, second, 0, "long cooldown: re-fire must be suppressed within 2s window")
	})
}
