// cmd/generate-testdata/main.go — Generate a deterministic JSONL log file for e2e testing.
//
// Produces a file with realistic log distribution across 6 phases, matching
// the same anomaly patterns that the e2e test expects to trigger:
//
//   - error_rate_spike    (payment-service errors)
//   - auth_failure_burst  (auth failures from single IP)
//   - latency_threshold   (high latency on api-gateway)
//   - repeated_failure    (identical errors on order-service)
//   - off_hours_access    (sensitive path access outside business hours)
//
// Usage:
//
//	go run ./cmd/generate-testdata [flags]
//	go run ./cmd/generate-testdata -count 100000 -o testdata/e2e-logs.jsonl
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"
)

var (
	flagCount  = flag.Int("count", 100_000, "Total number of log entries to generate")
	flagOutput = flag.String("o", "testdata/e2e-logs.jsonl", "Output file path")
	flagSeed   = flag.Int64("seed", 42, "Random seed for deterministic output")
	flagSpan   = flag.Duration("span", 30*time.Minute, "Time span the logs cover")
)

type logMsg struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields"`
}

var (
	services = []string{"api-gateway", "payment-service", "auth-service", "order-service", "inventory-service"}
	paths    = []string{"/api/v1/orders", "/api/v1/products", "/health", "/api/v1/checkout", "/api/v1/login", "/metrics"}
	normalIP = []string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "172.16.0.5", "10.0.2.10", "10.0.2.11"}

	infoMessages = []string{
		"GET %s 200 OK",
		"POST %s 201 Created",
		"GET %s 200 OK (cached)",
		"PUT %s 200 OK",
		"DELETE %s 204 No Content",
	}
	debugMessages = []string{
		"cache hit for %s",
		"connection pool: active=3 idle=7",
		"request tracing: span_id=%s",
		"health check passed",
		"gc cycle completed: freed 2.3MB",
	}
)

// phase defines a segment of the log timeline with its anomaly mix.
type phase struct {
	name       string
	startPct   float64 // fraction of total span where phase starts
	endPct     float64
	weightPct  float64 // fraction of total logs allocated to this phase
	generators []weightedGen
}

type weightedGen struct {
	weight int // relative weight
	gen    func(rng *rand.Rand, ts time.Time) logMsg
}

func pick[T any](rng *rand.Rand, s []T) T { return s[rng.Intn(len(s))] }

func normalLog(rng *rand.Rand, ts time.Time) logMsg {
	level := "info"
	if rng.Intn(5) == 0 {
		level = "debug"
	}
	p := pick(rng, paths)

	var msg string
	if level == "debug" {
		tpl := pick(rng, debugMessages)
		msg = fmt.Sprintf(tpl, p)
	} else {
		tpl := pick(rng, infoMessages)
		msg = fmt.Sprintf(tpl, p)
	}

	return logMsg{
		Timestamp: ts.Format(time.RFC3339),
		Level:     level,
		Service:   pick(rng, services),
		Message:   msg,
		Fields: map[string]any{
			"path":       p,
			"latency_ms": rng.Intn(200) + 5,
			"source_ip":  pick(rng, normalIP),
		},
	}
}

func warnLog(rng *rand.Rand, ts time.Time) logMsg {
	p := pick(rng, paths)
	lat := rng.Intn(300) + 200 // 200-500ms, elevated but not anomalous
	return logMsg{
		Timestamp: ts.Format(time.RFC3339),
		Level:     "warn",
		Service:   pick(rng, services),
		Message:   fmt.Sprintf("elevated latency: %d ms on %s", lat, p),
		Fields: map[string]any{
			"path":       p,
			"latency_ms": lat,
			"source_ip":  pick(rng, normalIP),
		},
	}
}

func errorSpikeLog(rng *rand.Rand, ts time.Time) logMsg {
	errs := []string{
		"internal server error: database connection timeout",
		"internal server error: connection pool exhausted",
		"internal server error: query timeout after 30s",
	}
	return logMsg{
		Timestamp: ts.Format(time.RFC3339),
		Level:     "error",
		Service:   "payment-service",
		Message:   pick(rng, errs),
		Fields: map[string]any{
			"path":      "/api/v1/checkout",
			"source_ip": pick(rng, normalIP),
			"error":     "dial tcp: connection refused",
		},
	}
}

func authFailLog(rng *rand.Rand, ts time.Time) logMsg {
	usernames := []string{"admin", "root", "administrator", "sysadmin"}
	return logMsg{
		Timestamp: ts.Format(time.RFC3339),
		Level:     "error",
		Service:   "auth-service",
		Message:   "authentication failure: invalid credentials",
		Fields: map[string]any{
			"source_ip":   "10.66.6.6", // single attacker IP
			"username":    pick(rng, usernames),
			"auth_result": "failure",
			"path":        "/api/v1/login",
		},
	}
}

func highLatencyLog(rng *rand.Rand, ts time.Time) logMsg {
	lat := rng.Intn(1500) + 600 // 600-2100ms, well above 500ms threshold
	p := pick(rng, paths)
	return logMsg{
		Timestamp: ts.Format(time.RFC3339),
		Level:     "warn",
		Service:   "api-gateway",
		Message:   fmt.Sprintf("slow response: %d ms", lat),
		Fields: map[string]any{
			"path":       p,
			"latency_ms": lat,
			"source_ip":  pick(rng, normalIP),
		},
	}
}

func repeatedFailLog(rng *rand.Rand, ts time.Time) logMsg {
	return logMsg{
		Timestamp: ts.Format(time.RFC3339),
		Level:     "error",
		Service:   "order-service",
		Message:   "handler panic: nil pointer dereference at /api/v1/checkout",
		Fields: map[string]any{
			"path":      "/api/v1/checkout",
			"source_ip": pick(rng, normalIP),
		},
	}
}

func offHoursLog(rng *rand.Rand, ts time.Time) logMsg {
	// Force timestamp to off-hours (02:00-04:00 UTC)
	offHour := ts.Truncate(24 * time.Hour).Add(time.Duration(2+rng.Intn(2)) * time.Hour).
		Add(time.Duration(rng.Intn(60)) * time.Minute)

	sensitivePaths := []string{"/admin", "/api/v1/users", "/internal"}
	p := pick(rng, sensitivePaths)
	return logMsg{
		Timestamp: offHour.Format(time.RFC3339),
		Level:     "info",
		Service:   pick(rng, services),
		Message:   fmt.Sprintf("GET %s 200 OK", p),
		Fields: map[string]any{
			"path":       p,
			"latency_ms": rng.Intn(100) + 10,
			"source_ip":  "192.168.1.99",
		},
	}
}

func buildPhases() []phase {
	return []phase{
		{
			name: "warm-up", startPct: 0.0, endPct: 0.15, weightPct: 0.15,
			generators: []weightedGen{
				{weight: 90, gen: normalLog},
				{weight: 10, gen: warnLog},
			},
		},
		{
			name: "error-spike", startPct: 0.15, endPct: 0.30, weightPct: 0.18,
			generators: []weightedGen{
				{weight: 40, gen: normalLog},
				{weight: 55, gen: errorSpikeLog},
				{weight: 5, gen: warnLog},
			},
		},
		{
			name: "auth-burst", startPct: 0.30, endPct: 0.45, weightPct: 0.15,
			generators: []weightedGen{
				{weight: 30, gen: normalLog},
				{weight: 65, gen: authFailLog},
				{weight: 5, gen: warnLog},
			},
		},
		{
			name: "high-latency", startPct: 0.45, endPct: 0.60, weightPct: 0.15,
			generators: []weightedGen{
				{weight: 30, gen: normalLog},
				{weight: 60, gen: highLatencyLog},
				{weight: 10, gen: warnLog},
			},
		},
		{
			name: "repeated-fail", startPct: 0.60, endPct: 0.75, weightPct: 0.15,
			generators: []weightedGen{
				{weight: 35, gen: normalLog},
				{weight: 55, gen: repeatedFailLog},
				{weight: 10, gen: warnLog},
			},
		},
		{
			name: "off-hours-access", startPct: 0.75, endPct: 0.82, weightPct: 0.07,
			generators: []weightedGen{
				{weight: 60, gen: normalLog},
				{weight: 30, gen: offHoursLog},
				{weight: 10, gen: warnLog},
			},
		},
		{
			name: "wind-down", startPct: 0.82, endPct: 1.0, weightPct: 0.15,
			generators: []weightedGen{
				{weight: 90, gen: normalLog},
				{weight: 10, gen: warnLog},
			},
		},
	}
}

func pickGenerator(rng *rand.Rand, gens []weightedGen) func(rng *rand.Rand, ts time.Time) logMsg {
	total := 0
	for _, g := range gens {
		total += g.weight
	}
	r := rng.Intn(total)
	cum := 0
	for _, g := range gens {
		cum += g.weight
		if r < cum {
			return g.gen
		}
	}
	return gens[len(gens)-1].gen
}

func main() {
	flag.Parse()

	rng := rand.New(rand.NewSource(*flagSeed))
	phases := buildPhases()
	count := *flagCount
	span := *flagSpan
	baseTime := time.Date(2026, 1, 15, 2, 0, 0, 0, time.UTC) // start at 02:00 UTC to enable off-hours detection

	f, err := os.Create(*flagOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	// Pre-calculate how many logs each phase gets.
	phaseCounts := make([]int, len(phases))
	remaining := count
	for i, p := range phases {
		if i == len(phases)-1 {
			phaseCounts[i] = remaining // last phase gets the remainder
		} else {
			phaseCounts[i] = int(float64(count) * p.weightPct)
			remaining -= phaseCounts[i]
		}
	}

	generated := 0
	stats := make(map[string]int) // level -> count

	for i, p := range phases {
		phaseStart := baseTime.Add(time.Duration(float64(span) * p.startPct))
		phaseEnd := baseTime.Add(time.Duration(float64(span) * p.endPct))
		phaseDur := phaseEnd.Sub(phaseStart)
		n := phaseCounts[i]

		for j := 0; j < n; j++ {
			// Spread timestamps evenly within the phase with small jitter.
			progress := float64(j) / float64(n)
			ts := phaseStart.Add(time.Duration(float64(phaseDur) * progress))
			jitter := time.Duration(rng.Int63n(int64(phaseDur / time.Duration(n+1))))
			ts = ts.Add(jitter)

			gen := pickGenerator(rng, p.generators)
			msg := gen(rng, ts)

			if err := enc.Encode(msg); err != nil {
				fmt.Fprintf(os.Stderr, "error writing log %d: %v\n", generated, err)
				os.Exit(1)
			}

			stats[msg.Level]++
			generated++
		}

		fmt.Printf("  phase %-18s  %6d logs  [%s - %s]\n",
			p.name, n, phaseStart.Format("15:04:05"), phaseEnd.Format("15:04:05"))
	}

	fmt.Printf("\nGenerated %d logs -> %s\n", generated, *flagOutput)
	fmt.Printf("Distribution: ")
	for _, lvl := range []string{"info", "debug", "warn", "error"} {
		if c, ok := stats[lvl]; ok {
			fmt.Printf("%s=%d (%.1f%%)  ", lvl, c, float64(c)/float64(generated)*100)
		}
	}
	fmt.Println()
}
