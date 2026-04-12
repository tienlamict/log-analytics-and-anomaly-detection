// cmd/e2etest/main.go — End-to-end pipeline load test.
//
// Tests the full flow: Kafka produce -> pipeline consume -> ES index -> API query.
//
// Usage:
//
//	go run ./cmd/e2etest [flags]
//
// The test runs in 5 steps:
//
//	[1/5] Health check      — verify the system is ready via GET /ready
//	[2/5] Baseline          — record current log/anomaly counts
//	[3/5] Produce           — send phased logs to Kafka (triggers detection rules)
//	[4/5] Verify drain      — poll API until all logs are indexed in ES
//	[5/5] Report            — print throughput, lag, drain time, detection results
//
// Phases (scaled to --duration, default 3m):
//
//	Phase 1  warm-up         — normal INFO/DEBUG across all services
//	Phase 2  error-spike     — >10 ERRORs on payment-service       -> error_rate_spike
//	Phase 3  auth-burst      — auth failures from single IP        -> auth_failure_burst
//	Phase 4  high-latency    — p95 >500ms on api-gateway           -> latency_threshold
//	Phase 5  repeated-fail   — same error 5x on order-service      -> repeated_failure
//	Phase 6  wind-down       — normal traffic
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ── flags ─────────────────────────────────────────────────────────────────────

var (
	flagBroker        = flag.String("broker", "localhost:9092", "Kafka broker address")
	flagTopic         = flag.String("topic", "application-logs", "Kafka topic")
	flagAPI           = flag.String("api", "http://localhost:8080", "API base URL")
	flagRate          = flag.Int("rate", 100, "Target messages per second (approximate)")
	flagDuration      = flag.Duration("duration", 3*time.Minute, "Produce phase duration (min 1m recommended)")
	flagVerifyTimeout = flag.Duration("verify-timeout", 90*time.Second, "Max time to wait for pipeline drain")
)

// ── log message (matches pipeline.Parse expectations) ─────────────────────────

type logMsg struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields"`
}

// ── API paged response (only need Total for verification) ─────────────────────

type pagedResp struct {
	Total int `json:"total"`
}

// ── phase definition ──────────────────────────────────────────────────────────

type phase struct {
	name           string
	start, end     time.Duration
	generate       func() logMsg
	rateMultiplier float64
	expectAnomaly  string // detection rule_id expected to fire, empty if none
}

// ── latency sample (captured during produce for lag tracking) ─────────────────

type sample struct {
	elapsed time.Duration
	sent    int64
	indexed int
}

// ── globals ───────────────────────────────────────────────────────────────────

var (
	allServices = []string{"api-gateway", "payment-service", "auth-service", "order-service", "inventory-service"}
	allPaths    = []string{"/api/v1/orders", "/api/v1/products", "/health", "/api/v1/checkout"}
	allIPs      = []string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "172.16.0.5"}
)

func ts() string          { return time.Now().UTC().Format(time.RFC3339) }
func pick[T any](s []T) T { return s[rand.Intn(len(s))] }

// ── log generators ────────────────────────────────────────────────────────────

func normalLog() logMsg {
	return logMsg{
		Timestamp: ts(),
		Level:     pick([]string{"info", "info", "info", "debug"}),
		Service:   pick(allServices),
		Message:   fmt.Sprintf("GET %s 200 OK", pick(allPaths)),
		Fields: map[string]any{
			"path":       pick(allPaths),
			"latency_ms": rand.Intn(200) + 10,
			"source_ip":  pick(allIPs),
		},
	}
}

func errorLog() logMsg {
	return logMsg{
		Timestamp: ts(),
		Level:     "error",
		Service:   "payment-service",
		Message:   "internal server error: database connection timeout",
		Fields: map[string]any{
			"path":      "/api/v1/checkout",
			"source_ip": pick(allIPs),
			"error":     "dial tcp: connection refused",
		},
	}
}

func authFailLog() logMsg {
	return logMsg{
		Timestamp: ts(),
		Level:     "error",
		Service:   "auth-service",
		Message:   "authentication failure: invalid credentials",
		Fields: map[string]any{
			"source_ip":   "10.66.6.6",
			"username":    "admin",
			"auth_result": "failure",
			"path":        "/api/v1/login",
		},
	}
}

func highLatencyLog() logMsg {
	lat := rand.Intn(1500) + 600 // 600-2100ms, well above 500ms threshold
	return logMsg{
		Timestamp: ts(),
		Level:     "warn",
		Service:   "api-gateway",
		Message:   fmt.Sprintf("slow response: %d ms", lat),
		Fields: map[string]any{
			"path":       pick(allPaths),
			"latency_ms": lat,
			"source_ip":  pick(allIPs),
		},
	}
}

func repeatedFailLog() logMsg {
	return logMsg{
		Timestamp: ts(),
		Level:     "error",
		Service:   "order-service",
		Message:   "handler panic: nil pointer dereference at /api/v1/checkout",
		Fields: map[string]any{
			"path":      "/api/v1/checkout",
			"source_ip": pick(allIPs),
		},
	}
}

// ── phase builder (scales 6 equal segments to --duration) ─────────────────────

func buildPhases(d time.Duration) []phase {
	seg := d / 6
	at := func(n int) time.Duration { return time.Duration(n) * seg }

	return []phase{
		{
			name: "warm-up", start: at(0), end: at(1),
			generate:       normalLog,
			rateMultiplier: 1.0,
		},
		{
			name: "error-spike", start: at(1), end: at(2),
			generate: func() logMsg {
				if rand.Intn(3) == 0 {
					return normalLog()
				}
				return errorLog()
			},
			rateMultiplier: 1.5,
			expectAnomaly:  "error_rate_spike",
		},
		{
			name: "auth-burst", start: at(2), end: at(3),
			generate: func() logMsg {
				if rand.Intn(4) == 0 {
					return normalLog()
				}
				return authFailLog()
			},
			rateMultiplier: 2.0,
			expectAnomaly:  "auth_failure_burst",
		},
		{
			name: "high-latency", start: at(3), end: at(4),
			generate: func() logMsg {
				if rand.Intn(4) == 0 {
					return normalLog()
				}
				return highLatencyLog()
			},
			rateMultiplier: 1.0,
			expectAnomaly:  "latency_threshold",
		},
		{
			name: "repeated-fail", start: at(4), end: at(5),
			generate: func() logMsg {
				if rand.Intn(3) == 0 {
					return normalLog()
				}
				return repeatedFailLog()
			},
			rateMultiplier: 1.2,
			expectAnomaly:  "repeated_failure",
		},
		{
			name: "wind-down", start: at(5), end: at(6),
			generate:       normalLog,
			rateMultiplier: 0.7,
		},
	}
}

func activePhase(phases []phase, elapsed time.Duration) phase {
	for _, p := range phases {
		if elapsed >= p.start && elapsed < p.end {
			return p
		}
	}
	return phases[len(phases)-1]
}

// ── API helpers ───────────────────────────────────────────────────────────────

var httpCl = &http.Client{Timeout: 10 * time.Second}

func healthCheck(base string) error {
	resp, err := httpCl.Get(base + "/ready")
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("/ready returned %d", resp.StatusCode)
	}
	return nil
}

func apiTotal(base, endpoint string, params url.Values) (int, error) {
	u := base + endpoint + "?" + params.Encode()
	resp, err := httpCl.Get(u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var pr pagedResp
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, err
	}
	return pr.Total, nil
}

func logCount(base string, from time.Time) (int, error) {
	return apiTotal(base, "/api/v1/logs", url.Values{
		"from": {from.Format(time.RFC3339)},
		"size": {"1"},
	})
}

func anomalyCount(base string, from time.Time, ruleType string) (int, error) {
	p := url.Values{
		"from": {from.Format(time.RFC3339)},
		"size": {"1"},
	}
	if ruleType != "" {
		p.Set("type", ruleType)
	}
	return apiTotal(base, "/api/v1/anomalies", p)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	testStart := time.Now().UTC()

	// ── Banner ──
	fmt.Println(strings.Repeat("=", 62))
	fmt.Println("  E2E Pipeline Load Test")
	fmt.Println(strings.Repeat("=", 62))
	fmt.Printf("  broker: %s   topic: %s\n", *flagBroker, *flagTopic)
	fmt.Printf("  api:    %s\n", *flagAPI)
	fmt.Printf("  rate:   %d msg/s   duration: %s   verify-timeout: %s\n\n",
		*flagRate, *flagDuration, *flagVerifyTimeout)

	// ── Step 1: Health check ──────────────────────────────────────────────
	fmt.Print("[1/5] Health check .......... ")
	if err := healthCheck(*flagAPI); err != nil {
		fmt.Printf("FAIL (%v)\n", err)
		fmt.Println("      Ensure the system is running: docker compose up -d")
		os.Exit(1)
	}
	fmt.Println("OK")

	// ── Step 2: Baseline ──────────────────────────────────────────────────
	fmt.Print("[2/5] Baseline .............. ")
	baseLogs, err := logCount(*flagAPI, testStart)
	if err != nil {
		fmt.Printf("FAIL (%v)\n", err)
		os.Exit(1)
	}
	baseAnoms, _ := anomalyCount(*flagAPI, testStart, "")
	fmt.Printf("logs=%d  anomalies=%d\n", baseLogs, baseAnoms)

	// ── Step 3: Produce ───────────────────────────────────────────────────
	phases := buildPhases(*flagDuration)

	fmt.Println("[3/5] Producing logs to Kafka")
	fmt.Println()
	for _, p := range phases {
		extra := ""
		if p.expectAnomaly != "" {
			extra = fmt.Sprintf("  -> %s", p.expectAnomaly)
		}
		fmt.Printf("       %5s - %-5s  %-18s  x%.1f%s\n",
			p.start.Truncate(time.Second), p.end.Truncate(time.Second),
			p.name, p.rateMultiplier, extra)
	}
	fmt.Println()

	// Kafka producer client.
	kClient, err := kgo.NewClient(
		kgo.SeedBrokers(*flagBroker),
		kgo.DefaultProduceTopic(*flagTopic),
		kgo.RecordPartitioner(kgo.RoundRobinPartitioner()),
		kgo.ProducerBatchMaxBytes(1<<20),
		kgo.MaxBufferedRecords(50_000),
		kgo.ProduceRequestTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatalf("kafka client: %v", err)
	}
	defer kClient.Close()

	var sent, dropped atomic.Int64
	var samples []sample
	var sampleMu sync.Mutex

	produceStart := time.Now()
	deadline := produceStart.Add(*flagDuration)

	// Sampling goroutine: every 10s query API to track pipeline lag in real time.
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-tick.C:
				if t.After(deadline) {
					return
				}
				elapsed := time.Since(produceStart).Truncate(time.Second)
				p := activePhase(phases, elapsed)
				s := sent.Load()
				indexed, _ := logCount(*flagAPI, testStart)

				sampleMu.Lock()
				samples = append(samples, sample{elapsed: elapsed, sent: s, indexed: indexed})
				sampleMu.Unlock()

				fmt.Printf("       [%5s] %-18s  sent=%-8d  indexed=%-8d  lag=%-6d\n",
					elapsed, p.name, s, indexed, s-int64(indexed))
			}
		}
	}()

	// Producer workers (same pattern as cmd/loadtest).
	const (
		tickInterval = 100 * time.Millisecond
		numWorkers   = 4
	)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(tickInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-ticker.C:
					if t.After(deadline) {
						return
					}
					elapsed := time.Since(produceStart)
					p := activePhase(phases, elapsed)
					eff := float64(*flagRate) * p.rateMultiplier
					batch := int(eff * tickInterval.Seconds() / float64(numWorkers))
					if batch < 1 {
						batch = 1
					}
					for i := 0; i < batch; i++ {
						payload, _ := json.Marshal(p.generate())
						kClient.Produce(ctx, &kgo.Record{Value: payload},
							func(_ *kgo.Record, err error) {
								if err != nil {
									dropped.Add(1)
								} else {
									sent.Add(1)
								}
							})
					}
				}
			}
		}()
	}

	wg.Wait()
	<-samplerDone

	// Flush remaining buffered records before measuring.
	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := kClient.Flush(flushCtx); err != nil {
		log.Printf("flush warning: %v", err)
	}

	produceDur := time.Since(produceStart)
	totalSent := sent.Load()
	totalDropped := dropped.Load()
	produceRate := float64(totalSent) / produceDur.Seconds()

	fmt.Printf("\n       Produce done: sent=%d  dropped=%d  rate=%.0f msg/s  elapsed=%s\n\n",
		totalSent, totalDropped, produceRate, produceDur.Truncate(time.Second))

	if totalSent == 0 {
		fmt.Println("  No messages sent. Exiting.")
		os.Exit(1)
	}

	// ── Step 4: Verify drain ──────────────────────────────────────────────
	fmt.Println("[4/5] Verifying pipeline drain")

	drainStart := time.Now()
	drainDeadline := drainStart.Add(*flagVerifyTimeout)
	var finalLogs int
	stableRuns := 0
	prevCount := -1

	for time.Now().Before(drainDeadline) {
		select {
		case <-ctx.Done():
			fmt.Println("       interrupted")
			goto report
		case <-time.After(3 * time.Second):
		}

		cur, err := logCount(*flagAPI, testStart)
		if err != nil {
			fmt.Printf("       poll error: %v\n", err)
			continue
		}
		anoms, _ := anomalyCount(*flagAPI, testStart, "")
		pct := float64(cur) / float64(totalSent) * 100
		fmt.Printf("       logs: %d/%d (%.1f%%)  anomalies: %d  elapsed: %s\n",
			cur, totalSent, pct, anoms, time.Since(drainStart).Truncate(time.Second))

		if cur == prevCount {
			stableRuns++
		} else {
			stableRuns = 0
		}
		prevCount = cur

		// Done: all logs indexed.
		if int64(cur) >= totalSent {
			break
		}
		// Stable at >=90%: pipeline has settled (some logs may have been deduplicated).
		if stableRuns >= 3 && float64(cur) >= float64(totalSent)*0.9 {
			break
		}
	}
	finalLogs = prevCount

report:
	drainDur := time.Since(drainStart)

	// Anomaly breakdown by rule type.
	ruleTypes := []string{
		"error_rate_spike", "auth_failure_burst", "latency_threshold",
		"repeated_failure", "off_hours_access", "service_silence",
	}
	anomByType := make(map[string]int)
	totalAnoms := 0
	for _, rt := range ruleTypes {
		c, _ := anomalyCount(*flagAPI, testStart, rt)
		anomByType[rt] = c
		totalAnoms += c
	}

	// ── Step 5: Report ────────────────────────────────────────────────────
	fmt.Printf("\n[5/5] Report\n")
	fmt.Println(strings.Repeat("=", 62))

	// Producer section.
	fmt.Println("\n  PRODUCER")
	fmt.Printf("    Sent:            %d messages\n", totalSent)
	fmt.Printf("    Dropped:         %d messages\n", totalDropped)
	fmt.Printf("    Duration:        %s\n", produceDur.Truncate(time.Second))
	fmt.Printf("    Throughput:      %.0f msg/s\n", produceRate)

	// Pipeline section.
	indexPct := float64(finalLogs) / float64(totalSent) * 100
	fmt.Println("\n  PIPELINE")
	fmt.Printf("    Indexed:         %d / %d (%.1f%%)\n", finalLogs, totalSent, indexPct)
	fmt.Printf("    Drain time:      %s (after produce ended)\n", drainDur.Truncate(time.Second))
	e2eThroughput := float64(finalLogs) / (produceDur + drainDur).Seconds()
	fmt.Printf("    E2E throughput:  %.0f msg/s\n", e2eThroughput)

	// Anomaly detection section.
	fmt.Println("\n  ANOMALY DETECTION")
	fmt.Printf("    Total: %d anomalies detected\n", totalAnoms)
	for _, rt := range ruleTypes {
		c := anomByType[rt]
		mark := "-"
		if c > 0 {
			mark = "+"
		}
		expected := ""
		for _, p := range phases {
			if p.expectAnomaly == rt {
				expected = " (expected)"
				break
			}
		}
		fmt.Printf("    [%s] %-25s count=%-3d%s\n", mark, rt, c, expected)
	}

	if totalAnoms == 0 {
		fmt.Println()
		fmt.Println("    Note: No anomalies detected. The detection engine has a")
		fmt.Println("    warmup period (default 10min) after app start. Ensure the")
		fmt.Println("    app has been running for >10 minutes before testing.")
	}

	// Lag samples table.
	if len(samples) > 0 {
		fmt.Println("\n  PIPELINE LAG (sampled every 10s during produce)")
		fmt.Printf("    %-8s  %-10s  %-10s  %-8s\n", "Elapsed", "Sent", "Indexed", "Lag")
		for _, s := range samples {
			fmt.Printf("    %-8s  %-10d  %-10d  %-8d\n",
				s.elapsed, s.sent, s.indexed, s.sent-int64(s.indexed))
		}
	}

	// Final verdict.
	fmt.Println()
	fmt.Println(strings.Repeat("=", 62))
	switch {
	case indexPct >= 99.0:
		fmt.Printf("  RESULT: PASS  (%.1f%% indexed, %d anomalies detected)\n", indexPct, totalAnoms)
	case indexPct >= 95.0:
		fmt.Printf("  RESULT: WARN  (%.1f%% indexed, some logs may still be draining)\n", indexPct)
	default:
		fmt.Printf("  RESULT: FAIL  (%.1f%% indexed, pipeline bottleneck or error)\n", indexPct)
	}
	fmt.Println(strings.Repeat("=", 62))
}
