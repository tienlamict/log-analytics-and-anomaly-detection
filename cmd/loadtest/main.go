// cmd/loadtest/main.go — 5-minute Kafka load generator for manual testing.
//
// Usage:
//
//	go run ./cmd/loadtest [--broker localhost:9092] [--topic application-logs] [--rate 500]
//
// The generator runs for exactly 5 minutes, cycling through scripted phases
// that exercise every detection rule in the pipeline:
//
//	Phase 1  0:00–1:00  Warm-up     — normal INFO/DEBUG baseline across all services
//	Phase 2  1:00–1:30  Error spike  — >10 ERRORs/5m on payment-service  → error_rate_spike
//	Phase 3  1:30–2:00  Auth burst   — 15 rapid auth failures from one IP → auth_failure_burst
//	Phase 4  2:00–3:00  High latency — sustained p95 >500 ms on api-gateway → latency_threshold
//	Phase 5  3:00–3:30  Repeat fail  — same endpoint 5×/5m errors on order-service → repeated_failure
//	Phase 6  3:30–4:30  Off-hours    — requests to /admin, /internal outside business hours
//	Phase 7  4:30–5:00  Wind-down    — normal traffic; inventory-service goes silent → service_silence
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ── flags ─────────────────────────────────────────────────────────────────────

var (
	flagBroker   = flag.String("broker", "localhost:9092", "Kafka broker address")
	flagTopic    = flag.String("topic", "application-logs", "Kafka topic")
	flagRate     = flag.Int("rate", 500, "Target messages per second (approximate)")
	flagDuration = flag.Duration("duration", 5*time.Minute, "Total run duration")
)

// ── log message shape (must match pipeline.Parse expectations) ─────────────────

type logMsg struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields"`
}

func encode(m logMsg) []byte {
	b, _ := json.Marshal(m)
	return b
}

// ── helpers ───────────────────────────────────────────────────────────────────

var services = []string{
	"api-gateway", "payment-service", "auth-service",
	"order-service", "inventory-service", "notification-service",
}

var paths = []string{"/api/v1/orders", "/api/v1/products", "/api/v1/users", "/health", "/metrics"}
var sensititivePaths = []string{"/admin", "/api/v1/users", "/internal"}
var sourceIPs = []string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "10.0.2.100", "172.16.0.5"}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func randItem[T any](s []T) T { return s[rand.Intn(len(s))] }

// normalLog produces a healthy INFO/DEBUG entry for a random service.
func normalLog() logMsg {
	svc := randItem(services)
	path := randItem(paths)
	latency := rand.Intn(200) + 10 // 10–210 ms — well within threshold
	return logMsg{
		Timestamp: now(),
		Level:     randItem([]string{"info", "info", "info", "debug"}),
		Service:   svc,
		Message:   fmt.Sprintf("GET %s 200 OK", path),
		Fields: map[string]any{
			"path":       path,
			"latency_ms": latency,
			"source_ip":  randItem(sourceIPs),
		},
	}
}

// errorLog produces an ERROR for a specific service.
func errorLog(svc string) logMsg {
	return logMsg{
		Timestamp: now(),
		Level:     "error",
		Service:   svc,
		Message:   "internal server error: database connection timeout",
		Fields: map[string]any{
			"path":      "/api/v1/checkout",
			"source_ip": randItem(sourceIPs),
			"error":     "dial tcp: connection refused",
		},
	}
}

// authFailureLog produces a rapid auth failure from a single attacker IP.
func authFailureLog(svc, attackerIP, username string) logMsg {
	return logMsg{
		Timestamp: now(),
		Level:     "error",
		Service:   svc,
		Message:   "authentication failure: invalid credentials",
		Fields: map[string]any{
			"source_ip":   attackerIP,
			"username":    username,
			"auth_result": "failure",
			"path":        "/api/v1/login",
		},
	}
}

// highLatencyLog produces a log with latency above the 500 ms threshold.
func highLatencyLog(svc string) logMsg {
	latency := rand.Intn(1500) + 600 // 600–2100 ms
	return logMsg{
		Timestamp: now(),
		Level:     "warn",
		Service:   svc,
		Message:   fmt.Sprintf("slow response: %d ms", latency),
		Fields: map[string]any{
			"path":       randItem(paths),
			"latency_ms": latency,
			"source_ip":  randItem(sourceIPs),
		},
	}
}

// repeatedFailureLog produces the same endpoint failing repeatedly.
func repeatedFailureLog(svc, endpoint string) logMsg {
	return logMsg{
		Timestamp: now(),
		Level:     "error",
		Service:   svc,
		Message:   fmt.Sprintf("handler panic: nil pointer dereference at %s", endpoint),
		Fields: map[string]any{
			"path":      endpoint,
			"source_ip": randItem(sourceIPs),
		},
	}
}

// offHoursLog produces a sensitive-path access that would fire outside 09:00–17:00 UTC.
func offHoursLog(svc string) logMsg {
	return logMsg{
		Timestamp: now(),
		Level:     "info",
		Service:   svc,
		Message:   "GET " + randItem(sensititivePaths) + " 200 OK",
		Fields: map[string]any{
			"path":      randItem(sensititivePaths),
			"source_ip": randItem(sourceIPs),
		},
	}
}

// ── phase scheduler ───────────────────────────────────────────────────────────

type phase struct {
	name     string
	start    time.Duration // offset from run start
	end      time.Duration
	generate func() logMsg
	// rateMultiplier: 1.0 = use global --rate, >1 = burst
	rateMultiplier float64
}

func buildPhases() []phase {
	return []phase{
		{
			name:           "warm-up (normal traffic)",
			start:          0,
			end:            60 * time.Second,
			generate:       normalLog,
			rateMultiplier: 1.0,
		},
		{
			name:  "error rate spike — payment-service",
			start: 60 * time.Second,
			end:   90 * time.Second,
			generate: func() logMsg {
				if rand.Intn(3) == 0 {
					return normalLog()
				}
				return errorLog("payment-service")
			},
			rateMultiplier: 1.5,
		},
		{
			name:  "auth failure burst — attacker 10.66.6.6",
			start: 90 * time.Second,
			end:   120 * time.Second,
			generate: func() logMsg {
				if rand.Intn(4) == 0 {
					return normalLog()
				}
				return authFailureLog("auth-service", "10.66.6.6", "admin")
			},
			rateMultiplier: 2.0,
		},
		{
			name:  "high latency — api-gateway",
			start: 120 * time.Second,
			end:   180 * time.Second,
			generate: func() logMsg {
				if rand.Intn(4) == 0 {
					return normalLog()
				}
				return highLatencyLog("api-gateway")
			},
			rateMultiplier: 1.0,
		},
		{
			name:  "repeated failure — order-service /api/v1/checkout",
			start: 180 * time.Second,
			end:   210 * time.Second,
			generate: func() logMsg {
				if rand.Intn(3) == 0 {
					return normalLog()
				}
				return repeatedFailureLog("order-service", "/api/v1/checkout")
			},
			rateMultiplier: 1.2,
		},
		{
			name:  "off-hours sensitive path access",
			start: 210 * time.Second,
			end:   270 * time.Second,
			generate: func() logMsg {
				if rand.Intn(3) == 0 {
					return normalLog()
				}
				return offHoursLog("api-gateway")
			},
			rateMultiplier: 1.0,
		},
		{
			name:  "wind-down (inventory-service goes silent)",
			start: 270 * time.Second,
			end:   300 * time.Second,
			generate: func() logMsg {
				// Inventory-service deliberately excluded — triggers service_silence
				nonInventory := []string{"api-gateway", "payment-service", "auth-service", "order-service", "notification-service"}
				m := normalLog()
				m.Service = randItem(nonInventory)
				return m
			},
			rateMultiplier: 0.7,
		},
	}
}

// currentPhase returns the active phase for the given elapsed time.
// Falls back to the last phase if nothing matches.
func currentPhase(phases []phase, elapsed time.Duration) phase {
	for _, p := range phases {
		if elapsed >= p.start && elapsed < p.end {
			return p
		}
	}
	return phases[len(phases)-1]
}

// ── producer ──────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := kgo.NewClient(
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
	defer client.Close()

	phases := buildPhases()
	startTime := time.Now()
	deadline := startTime.Add(*flagDuration)

	var (
		sent    atomic.Int64
		dropped atomic.Int64
	)

	// Progress ticker — print stats every 10 seconds.
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				elapsed := time.Since(startTime).Truncate(time.Second)
				p := currentPhase(phases, elapsed)
				fmt.Printf("[%s] phase=%q  sent=%d  dropped=%d\n",
					elapsed, p.name, sent.Load(), dropped.Load())
			}
		}
	}()

	fmt.Printf("load test start  broker=%s  topic=%s  rate=%d msg/s  duration=%s\n",
		*flagBroker, *flagTopic, *flagRate, *flagDuration)
	printPhaseSchedule(phases)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			break
		}

		elapsed := time.Since(startTime)
		p := currentPhase(phases, elapsed)

		effectiveRate := float64(*flagRate) * p.rateMultiplier
		interval := time.Duration(float64(time.Second) / effectiveRate)

		msg := p.generate()
		payload := encode(msg)

		client.Produce(ctx, &kgo.Record{Value: payload}, func(r *kgo.Record, err error) {
			if err != nil {
				dropped.Add(1)
			} else {
				sent.Add(1)
			}
		})

		select {
		case <-ctx.Done():
		case <-time.After(interval):
		}
	}

	// Flush remaining buffered records.
	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Flush(flushCtx); err != nil {
		log.Printf("flush warning: %v", err)
	}

	elapsed := time.Since(startTime).Truncate(time.Second)
	fmt.Printf("\nload test complete  elapsed=%s  sent=%d  dropped=%d\n",
		elapsed, sent.Load(), dropped.Load())
}

func printPhaseSchedule(phases []phase) {
	fmt.Println("\nPhase schedule:")
	for _, p := range phases {
		fmt.Printf("  %5s – %5s  %s\n",
			p.start.String(), p.end.String(), p.name)
	}
	fmt.Println()
}
