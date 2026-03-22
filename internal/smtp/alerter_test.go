package smtp

import (
	"context"
	"strings"
	"testing"
	"time"

	mail "github.com/wneessen/go-mail"
	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// fixedTime is a stable reference time used in tests.
var fixedTime = time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)

func testAnomaly() domain.Anomaly {
	return domain.Anomaly{
		ID:          "test-id-1",
		RuleID:      "error_rate",
		Severity:    "high",
		Service:     "payment-api",
		Description: "Error count exceeded threshold",
		DetectedAt:  fixedTime,
		Evidence: []domain.LogEntry{
			{
				Timestamp: fixedTime.Add(-2 * time.Minute),
				Level:     "error",
				Message:   "connection refused",
			},
			{
				Timestamp: fixedTime.Add(-1 * time.Minute),
				Level:     "error",
				Message:   "timeout exceeded",
			},
		},
	}
}

func TestFormatAlertBody_ContainsAllFields(t *testing.T) {
	a := testAnomaly()
	body := formatAlertBody(a)

	assert.Contains(t, body, "error_rate", "body must include RuleID")
	assert.Contains(t, body, "payment-api", "body must include Service")
	assert.Contains(t, body, "high", "body must include Severity")
	assert.Contains(t, body, fixedTime.UTC().Format(time.RFC3339), "body must include DetectedAt in RFC3339")
	assert.Contains(t, body, "Error count exceeded threshold", "body must include Description")
	assert.Contains(t, body, "connection refused", "body must include first evidence message")
	assert.Contains(t, body, "timeout exceeded", "body must include second evidence message")
}

func TestFormatAlertBody_CapsAt5Evidence(t *testing.T) {
	a := domain.Anomaly{
		ID:          "test-id-cap",
		RuleID:      "rate_limit",
		Severity:    "medium",
		Service:     "auth-service",
		Description: "Too many requests",
		DetectedAt:  fixedTime,
	}

	// Add 8 evidence entries, each with a distinct 2026- timestamp prefix.
	for i := 0; i < 8; i++ {
		a.Evidence = append(a.Evidence, domain.LogEntry{
			Timestamp: fixedTime.Add(time.Duration(i) * time.Second),
			Level:     "warn",
			Message:   "request rejected",
		})
	}

	body := formatAlertBody(a)

	// Each evidence line starts with "  [20" (two spaces then the RFC3339 year).
	count := strings.Count(body, "  [20")
	assert.Equal(t, 5, count, "evidence must be capped at 5 lines")
}

func TestParseTLSPolicy_ValidValues(t *testing.T) {
	cases := []struct {
		input    string
		expected mail.TLSPolicy
	}{
		{"mandatory", mail.TLSMandatory},
		{"opportunistic", mail.TLSOpportunistic},
		{"none", mail.NoTLS},
		{"", mail.NoTLS},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			policy, err := parseTLSPolicy(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, policy)
		})
	}
}

func TestParseTLSPolicy_InvalidValue(t *testing.T) {
	_, err := parseTLSPolicy("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown TLS policy")
}

func TestSMTPAlerter_Name(t *testing.T) {
	a := &SMTPAlerter{}
	assert.Equal(t, "smtp", a.Name())
}

func TestSMTPAlerter_Send_Enqueues(t *testing.T) {
	// Send only enqueues — no real SMTP dial needed.
	a := &SMTPAlerter{
		queue: make(chan domain.Anomaly, 10),
	}

	anomaly := testAnomaly()
	err := a.Send(context.Background(), anomaly)
	require.NoError(t, err)

	select {
	case received := <-a.queue:
		assert.Equal(t, anomaly.ID, received.ID)
	default:
		t.Fatal("expected anomaly in queue but queue was empty")
	}
}

func TestSMTPAlerter_Send_QueueFull(t *testing.T) {
	// Create alerter with a queue of size 1.
	a := &SMTPAlerter{
		queue:  make(chan domain.Anomaly, 1),
		logger: noopLogger(t),
	}

	// Fill the queue.
	first := testAnomaly()
	first.ID = "first"
	err := a.Send(context.Background(), first)
	require.NoError(t, err)

	// Record baseline before the drop.
	before := testutil.ToFloat64(metrics.EmailAlertsFailedTotal)

	// Second Send must not block and must not return an error.
	second := testAnomaly()
	second.ID = "second"
	err = a.Send(context.Background(), second)
	assert.NoError(t, err, "Send must not return error on queue full")

	// Failure counter must have been incremented.
	after := testutil.ToFloat64(metrics.EmailAlertsFailedTotal)
	assert.Equal(t, before+1, after, "EmailAlertsFailedTotal must increment on queue full")
}

// noopLogger returns a no-op zap.Logger suitable for tests that trigger log
// output but do not assert on it.
func noopLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("create zap logger: %v", err)
	}
	return logger
}
