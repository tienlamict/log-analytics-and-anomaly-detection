//go:build integration

package integration

// High-load system test: 10,000 messages exercising all 5 targeted anomaly rules
// under concurrent producer load. Verifies throughput, detection coverage, and
// pipeline stability under stress.
//
// Run with:
//
//	go test -v -tags integration ./internal/integration/ -run TestHighLoad -timeout 5m

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/log-analytics/server/internal/alert"
	"github.com/log-analytics/server/internal/config"
	"github.com/log-analytics/server/internal/detection"
	"github.com/log-analytics/server/internal/domain"
	es "github.com/log-analytics/server/internal/elasticsearch"
	kafkaconsumer "github.com/log-analytics/server/internal/kafka"
	"github.com/log-analytics/server/internal/pipeline"
)

// ── Test constants ────────────────────────────────────────────────────────────

const (
	// isolatedTopic is used when running against testcontainers (isolated mode).
	isolatedTopic = "load-test-logs"
	// liveTopic is the topic consumed by the running Docker stack app.
	liveTopic   = "application-logs"
	loadGroupID = "load-test-consumer-group"

	totalMessages = 10_000

	// Detection windows are short to minimise total test time (isolated mode only).
	// windowDur must be large enough to cover the full produce→consume latency.
	windowDur = 10 * time.Second

	// warmupWait is slightly longer than the warmup period (warmup_multiplier=1 × window=10s).
	warmupWait = 11 * time.Second

	// How long to wait for all anomaly rules to fire after producing (isolated mode).
	anomalyTimeout = 90 * time.Second

	// Minimum acceptable ingestion throughput (msgs/s) for the Kafka producer.
	minProduceThroughput = 500.0

	// Expected rules to fire. service_silence is excluded (requires waiting 5+ min).
	expectedRuleCount = 5
)

// Message-mix breakdown (total = 10,000):
//   - background:       9,490 info/warn logs across dedicated services
//   - errorSpike:         200 errors from payment-service       → error_rate_spike
//   - latencyBreach:      200 requests to api-gateway (50% slow) → latency_threshold
//   - authBurst:           50 auth failures from 192.168.1.99   → auth_failure_burst
//   - repeatedFailure:     50 identical errors from order-service → repeated_failure
//   - offHours:            10 /admin accesses at 02:00 UTC       → off_hours_access
const (
	nBackground       = 9_490
	nErrorSpike       = 200
	nLatencyBreach    = 200 // 100 fast + 100 slow
	nAuthBurst        = 50
	nRepeatedFailure  = 50
	nOffHours         = 10
)

// ── Test entry point ──────────────────────────────────────────────────────────

func TestHighLoad(t *testing.T) {
	if liveMode {
		testHighLoadLive(t)
	} else {
		testHighLoadIsolated(t)
	}
}

// testHighLoadLive produces the full message mix to the running Docker stack's
// application-logs topic. Detection and indexing are handled by the running app;
// this function only measures producer throughput and reports when to expect results.
//
// Run with:
//
//	LIVE_KAFKA_BROKERS=localhost:9092 LIVE_ES_ADDRESS=http://localhost:9200 \
//	  go test -v -tags integration ./internal/integration/ -run TestHighLoad -timeout 5m
func testHighLoadLive(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	messages := generateLoadMessages()
	require.Equal(t, totalMessages, len(messages), "message count mismatch")

	t.Logf("live mode: producing %d messages to topic %q on brokers %v", totalMessages, liveTopic, kafkaBrokers)
	produceStart := time.Now()
	require.NoError(t, produceMessages(ctx, kafkaBrokers, liveTopic, messages))
	produceDuration := time.Since(produceStart)
	produceRate := float64(totalMessages) / produceDuration.Seconds()

	assert.GreaterOrEqual(t, produceRate, minProduceThroughput,
		"producer throughput below %.0f msg/s (got %.0f)", minProduceThroughput, produceRate)

	t.Logf("─── live load test summary ──────────────────────────")
	t.Logf("  topic            : %s", liveTopic)
	t.Logf("  total messages   : %d", totalMessages)
	t.Logf("  produce duration : %v", produceDuration.Round(time.Millisecond))
	t.Logf("  produce rate     : %.0f msg/s", produceRate)
	t.Logf("  message mix:")
	t.Logf("    background       : %d  (info/warn across 5 services)", nBackground)
	t.Logf("    error_rate_spike : %d  errors from payment-service", nErrorSpike)
	t.Logf("    latency_threshold: %d  requests to api-gateway (50%% slow)", nLatencyBreach)
	t.Logf("    auth_failure_burst: %d  failures from 192.168.1.99", nAuthBurst)
	t.Logf("    repeated_failure : %d  identical errors from order-service", nRepeatedFailure)
	t.Logf("    off_hours_access : %d  /admin accesses at 02:00 UTC", nOffHours)
	t.Logf("────────────────────────────────────────────────────")
	t.Log("NOTE: the running app uses a 5m detection window with 2x warmup multiplier.")
	t.Log("      Anomalies will appear after ~10 minutes if the app just started,")
	t.Log("      or sooner if it has already warmed up.")
	t.Log("      Query: curl http://localhost:8080/api/v1/anomalies")
}

// testHighLoadIsolated runs the full pipeline in-process against isolated
// testcontainers. Uses short detection windows (10s) to complete quickly.
func testHighLoadIsolated(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := zap.NewNop()

	// 1. Wire Elasticsearch — reuse the TLS-capable client from TestMain.
	esClient, err := newESClient()
	require.NoError(t, err, "create ES client")

	require.NoError(t, es.ApplyIndexTemplates(ctx, esClient), "apply ES index templates")

	logIndexer, err := es.NewLogIndexer(esClient, logger)
	require.NoError(t, err)
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = logIndexer.Close(shutCtx)
	}()

	anomalyIndexer, err := es.NewAnomalyIndexer(esClient, logger)
	require.NoError(t, err)
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = anomalyIndexer.Close(shutCtx)
	}()

	// 2. Wire detection engine with short windows so the test completes quickly.
	detectionCfg := loadDetectionConfig()
	offHoursRule, err := detection.NewOffHoursAccessRule(detectionCfg.Rules.OffHours)
	require.NoError(t, err)

	detectors := []domain.Detector{
		detection.NewErrorRateRule(detectionCfg.Rules.ErrorRate),
		detection.NewLatencyThresholdRule(detectionCfg.Rules.Latency),
		detection.NewRepeatedFailureRule(detectionCfg.Rules.RepeatedFailure),
		detection.NewAuthFailureBurstRule(detectionCfg.Rules.AuthBurst),
		offHoursRule,
		// ServiceSilenceRule omitted: requires minutes of silence to fire.
	}
	engine := detection.NewDetectorEngine(detectors, detectionCfg, logger)
	defer engine.Stop()

	// 3. Wire Kafka consumer with dedicated topic + consumer group for isolation.
	consumer, err := kafkaconsumer.New(config.KafkaConfig{
		Brokers:       kafkaBrokers,
		Topic:         isolatedTopic,
		GroupID:       loadGroupID,
		InitialOffset: "oldest",
	}, logger)
	require.NoError(t, err)

	// 4. Wire dispatcher (no push channel; monitoring via Prometheus → Grafana).
	dispatcher := alert.NewDispatcher(engine.Anomalies(), anomalyIndexer, nil, logger)

	// 5. Start pipeline goroutines.
	pipelineCtx, pipelineCancel := context.WithCancel(ctx)
	defer pipelineCancel()

	g, gCtx := errgroup.WithContext(pipelineCtx)

	g.Go(func() error { return consumer.Run(gCtx) })

	var processed atomic.Int64
	for range 4 {
		g.Go(func() error {
			for msg := range consumer.Messages() {
				entry := pipeline.ProcessMessage(msg, logger)
				if err := logIndexer.IndexLog(gCtx, entry); err != nil {
					logger.Warn("load test: index log error", zap.Error(err))
				}
				engine.Evaluate(entry)
				processed.Add(1)
			}
			return nil
		})
	}

	g.Go(func() error { return dispatcher.Run(gCtx) })

	// 6. Snapshot Prometheus metric baselines before producing load.
	baselineByRule := snapshotAnomalyCountsByRule()
	baselineParseErrors := gatherCounterSum("parse_errors_total")

	t.Logf("waiting %s for detection warmup to expire…", warmupWait)
	select {
	case <-time.After(warmupWait):
	case <-ctx.Done():
		t.Fatal("context cancelled during warmup wait")
	}

	// 7. Generate messages NOW (post-warmup timestamps land within the detection window).
	messages := generateLoadMessages()
	require.Equal(t, totalMessages, len(messages), "message count mismatch")

	// 8. Produce all messages concurrently; measure throughput.
	t.Logf("producing %d messages with %d workers…", totalMessages, 8)
	produceStart := time.Now()
	require.NoError(t, produceMessages(ctx, kafkaBrokers, isolatedTopic, messages))
	produceDuration := time.Since(produceStart)
	produceRate := float64(totalMessages) / produceDuration.Seconds()
	t.Logf("produced %d messages in %v → %.0f msg/s", totalMessages, produceDuration.Round(time.Millisecond), produceRate)

	// 9. Wait until all 5 targeted anomaly rules have fired (or timeout).
	t.Log("waiting for anomaly rules to fire…")
	firedRules := waitForAnomalyRules(ctx, baselineByRule, anomalyTimeout)

	// 10. Cancel pipeline; wait for clean shutdown.
	pipelineCancel()
	_ = g.Wait()

	// ── Assertions ────────────────────────────────────────────────────────────

	t.Logf("pipeline processed %d messages", processed.Load())

	assert.GreaterOrEqual(t, produceRate, minProduceThroughput,
		"producer throughput below %.0f msg/s (got %.0f)", minProduceThroughput, produceRate)

	for _, rule := range []string{
		"error_rate_spike",
		"latency_threshold",
		"auth_failure_burst",
		"repeated_failure",
		"off_hours_access",
	} {
		delta := firedRules[rule]
		assert.GreaterOrEqualf(t, delta, 1.0,
			"expected rule %q to fire at least once (delta=%.0f)", rule, delta)
	}

	parseErrorDelta := gatherCounterSum("parse_errors_total") - baselineParseErrors
	parseErrorRate := parseErrorDelta / float64(totalMessages) * 100
	assert.Less(t, parseErrorRate, 1.0,
		"parse error rate %.2f%% exceeds 1%% threshold", parseErrorRate)

	t.Logf("─── load test summary ───────────────────────────────")
	t.Logf("  total messages    : %d", totalMessages)
	t.Logf("  produce duration  : %v", produceDuration.Round(time.Millisecond))
	t.Logf("  produce rate      : %.0f msg/s", produceRate)
	t.Logf("  pipeline processed: %d", processed.Load())
	t.Logf("  parse errors      : %.0f (%.2f%%)", parseErrorDelta, parseErrorRate)
	t.Logf("  anomaly rule deltas:")
	for rule, delta := range firedRules {
		t.Logf("    %-25s +%.0f", rule, delta)
	}
	t.Logf("────────────────────────────────────────────────────")
}

// ── Detection config ──────────────────────────────────────────────────────────

// loadDetectionConfig returns a DetectionConfig with short windows suitable
// for the load test. warmup_multiplier=1 gives a 10s warmup (1 × 10s window).
func loadDetectionConfig() detection.DetectionConfig {
	return detection.DetectionConfig{
		WindowDuration:   windowDur,
		EvictionInterval: 5 * time.Second,
		CooldownDuration: 5 * time.Second,
		WarmupMultiplier: 1,
		Rules: detection.DetectionRulesConfig{
			ErrorRate: detection.ErrorRateConfig{
				Enabled:   true,
				Threshold: 10,
				Window:    windowDur,
				Severity:  "high",
			},
			Latency: detection.LatencyConfig{
				Enabled:           true,
				ThresholdMs:       500,
				BreachRatePercent: 20,
				Window:            windowDur,
				Severity:          "medium",
			},
			RepeatedFailure: detection.RepeatedFailureConfig{
				Enabled:   true,
				Threshold: 5,
				Window:    windowDur,
				Severity:  "medium",
			},
			AuthBurst: detection.AuthBurstConfig{
				Enabled:   true,
				Threshold: 10,
				Window:    windowDur,
				IPField:   "source_ip",
				UserField: "username",
				Severity:  "high",
			},
			OffHours: detection.OffHoursConfig{
				Enabled:            true,
				BusinessHoursStart: 9,
				BusinessHoursEnd:   17,
				Timezone:           "UTC",
				SensitivePaths:     []string{"/admin", "/api/v1/users", "/internal"},
				Severity:           "medium",
			},
			// ServiceSilenceRule excluded from load test (requires minutes of silence).
			ServiceSilence: detection.ServiceSilenceConfig{
				Enabled: false,
			},
		},
	}
}

// ── Message generation ────────────────────────────────────────────────────────

type logPayload struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields"`
}

// generateLoadMessages builds 10,000 JSON payloads that exercise every targeted
// anomaly rule. Timestamps use time.Now() so all entries land within the 10s
// detection window when consumed moments later.
func generateLoadMessages() [][]byte {
	now := time.Now().UTC()
	offHoursTS := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)

	msgs := make([][]byte, 0, totalMessages)

	marshal := func(p logPayload) []byte {
		b, _ := json.Marshal(p)
		return b
	}
	ts := func() string { return time.Now().UTC().Format(time.RFC3339) }

	// Background: diverse info/warn logs — use services that never overlap with
	// the targeted rule services so their counts don't interfere.
	bgServices := []string{
		"inventory-svc", "billing-svc", "notification-svc",
		"search-svc", "recommendation-svc",
	}
	bgMessages := []struct{ level, msg string }{
		{"info", "request handled successfully"},
		{"info", "cache hit for key product:42"},
		{"warn", "slow database query detected"},
		{"info", "user session refreshed"},
		{"debug", "background job completed"},
	}
	for i := 0; i < nBackground; i++ {
		svc := bgServices[i%len(bgServices)]
		bm := bgMessages[i%len(bgMessages)]
		msgs = append(msgs, marshal(logPayload{
			Timestamp: ts(),
			Level:     bm.level,
			Service:   svc,
			Message:   bm.msg,
			Fields:    map[string]any{"latency_ms": 10 + (i % 200)},
		}))
	}

	// Error rate spike: 200 errors from payment-service → fires error_rate_spike
	// (threshold=10 in 10s; 200 >> 10).
	for i := 0; i < nErrorSpike; i++ {
		msgs = append(msgs, marshal(logPayload{
			Timestamp: ts(),
			Level:     "error",
			Service:   "payment-service",
			Message:   fmt.Sprintf("payment gateway timeout: attempt %d", i),
			Fields:    map[string]any{},
		}))
	}

	// Latency breach: 100 fast + 100 slow requests to api-gateway.
	// Breach rate = 100/200 = 50% >> 20% threshold → fires latency_threshold.
	for i := 0; i < nLatencyBreach; i++ {
		var latencyMs int
		var level string
		if i%2 == 0 {
			latencyMs = 50 // fast
			level = "info"
		} else {
			latencyMs = 850 // slow — well above 500ms threshold
			level = "warn"
		}
		msgs = append(msgs, marshal(logPayload{
			Timestamp: ts(),
			Level:     level,
			Service:   "api-gateway",
			Message:   "GET /products",
			Fields:    map[string]any{"latency_ms": latencyMs},
		}))
	}

	// Auth failure burst: 50 failures from the same source IP.
	// (threshold=10; 50 >> 10) → fires auth_failure_burst.
	for i := 0; i < nAuthBurst; i++ {
		msgs = append(msgs, marshal(logPayload{
			Timestamp: ts(),
			Level:     "error",
			Service:   "auth-service",
			Message:   "login failed: unauthorized",
			Fields:    map[string]any{"source_ip": "192.168.1.99", "username": "attacker"},
		}))
	}

	// Repeated failure: 50 identical errors from order-service.
	// The fingerprinter strips numbers, making all 50 the same fingerprint.
	// (threshold=5; 50 >> 5) → fires repeated_failure.
	for i := 0; i < nRepeatedFailure; i++ {
		msgs = append(msgs, marshal(logPayload{
			Timestamp: ts(),
			Level:     "error",
			Service:   "order-service",
			Message:   "NullPointerException in OrderProcessor.java:42",
			Fields:    map[string]any{},
		}))
	}

	// Off-hours access: 10 accesses to /admin at 02:00 UTC from distinct services
	// so each (rule_id, service) pair is unique and cooldown doesn't suppress them.
	// The OffHoursRule is stateless — it fires on the timestamp hour, not a window.
	for i := 0; i < nOffHours; i++ {
		msgs = append(msgs, marshal(logPayload{
			Timestamp: offHoursTS.Format(time.RFC3339),
			Level:     "info",
			Service:   fmt.Sprintf("admin-portal-%d", i+1),
			Message:   "GET /admin/settings",
			Fields:    map[string]any{"path": "/admin/settings", "username": "ops-user"},
		}))
	}

	return msgs
}

// ── Kafka producer ────────────────────────────────────────────────────────────

// produceMessages sends all payloads to the given Kafka topic using 8 concurrent
// producer goroutines, then flushes before returning. The topic is auto-created
// on first produce by the confluent-local broker.
func produceMessages(ctx context.Context, brokers []string, topic string, payloads [][]byte) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(topic),
		kgo.ProducerBatchMaxBytes(4*1024*1024),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		// Retry indefinitely on UNKNOWN_TOPIC_OR_PARTITION: the topic is
		// auto-created on first produce; leader election can take a few seconds
		// in a fresh container and the default 4 retries are not enough.
		kgo.UnknownTopicRetries(-1),
	)
	if err != nil {
		return fmt.Errorf("create kafka producer: %w", err)
	}
	defer client.Close()

	const workers = 8
	chunk := len(payloads) / workers

	var (
		mu      sync.Mutex
		firstErr error
	)
	setErr := func(e error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * chunk
		end := start + chunk
		if w == workers-1 {
			end = len(payloads) // last worker picks up remainder
		}
		wg.Add(1)
		go func(slice [][]byte) {
			defer wg.Done()
			for _, payload := range slice {
				client.Produce(ctx, &kgo.Record{Value: payload}, func(_ *kgo.Record, err error) {
					if err != nil {
						setErr(fmt.Errorf("produce record: %w", err))
					}
				})
			}
		}(payloads[start:end])
	}
	wg.Wait()

	if err := client.Flush(ctx); err != nil {
		return fmt.Errorf("flush producer: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	return firstErr
}

// ── Prometheus helpers ────────────────────────────────────────────────────────

// snapshotAnomalyCountsByRule returns the current anomalies_detected_total value
// keyed by the "rule" label. Used to compute deltas after the load run.
func snapshotAnomalyCountsByRule() map[string]float64 {
	snapshot := make(map[string]float64)
	mfs, _ := prometheus.DefaultGatherer.Gather()
	for _, mf := range mfs {
		if mf.GetName() != "anomalies_detected_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "rule" {
					snapshot[lp.GetValue()] = m.GetCounter().GetValue()
				}
			}
		}
	}
	return snapshot
}

// gatherCounterSum sums all label combinations of a named counter metric.
func gatherCounterSum(metricName string) float64 {
	mfs, _ := prometheus.DefaultGatherer.Gather()
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		var total float64
		for _, m := range mf.GetMetric() {
			total += m.GetCounter().GetValue()
		}
		return total
	}
	return 0
}

// waitForAnomalyRules polls until all 5 expected rules show a positive delta
// vs the provided baseline snapshot, or until timeout. Returns deltas per rule.
func waitForAnomalyRules(ctx context.Context, baseline map[string]float64, timeout time.Duration) map[string]float64 {
	expected := map[string]bool{
		"error_rate_spike":  true,
		"latency_threshold": true,
		"auth_failure_burst": true,
		"repeated_failure":  true,
		"off_hours_access":  true,
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			current := snapshotAnomalyCountsByRule()
			allFired := true
			for rule := range expected {
				if current[rule]-baseline[rule] < 1 {
					allFired = false
					break
				}
			}
			if allFired || time.Now().After(deadline) {
				deltas := make(map[string]float64, len(current))
				for rule, val := range current {
					deltas[rule] = val - baseline[rule]
				}
				return deltas
			}
		case <-ctx.Done():
			current := snapshotAnomalyCountsByRule()
			deltas := make(map[string]float64, len(current))
			for rule, val := range current {
				deltas[rule] = val - baseline[rule]
			}
			return deltas
		}
	}
}
