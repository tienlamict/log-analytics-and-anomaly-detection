package pipeline

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNormaliseLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		// error variants
		{"error", "error"},
		{"ERROR", "error"},
		{"ERR", "error"},
		{"err", "error"},
		{"FATAL", "error"},
		{"fatal", "error"},
		{"CRITICAL", "error"},
		{"CRIT", "error"},
		// warn variants
		{"warn", "warn"},
		{"WARN", "warn"},
		{"WARNING", "warn"},
		{"warning", "warn"},
		// info variants
		{"info", "info"},
		{"INFO", "info"},
		{"INFORMATION", "info"},
		// debug variants
		{"debug", "debug"},
		{"DEBUG", "debug"},
		{"TRACE", "debug"},
		{"VERBOSE", "debug"},
		// unknown variants
		{"", "unknown"},
		{"CUSTOM_LEVEL", "unknown"},
		// whitespace trimming
		{"   ERROR   ", "error"},
	}

	for _, tc := range cases {
		tc := tc // capture range var
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, NormaliseLevel(tc.input))
		})
	}
}

func TestParse_StructuredJSON(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"timestamp":"2026-03-21T10:30:00Z","level":"ERROR","service":"auth-service","message":"login failed","fields":{"user_id":"abc123","attempt":3}}`)
	msg := domain.RawMessage{
		Payload:   payload,
		Topic:     "test-topic",
		Partition: 0,
		Offset:    42,
	}

	entry, err := Parse(msg)

	assert.NoError(t, err)
	assert.NotEmpty(t, entry.ID, "ID should be a non-empty UUID")

	expectedTS, _ := time.Parse(time.RFC3339, "2026-03-21T10:30:00Z")
	assert.Equal(t, expectedTS, entry.Timestamp)

	assert.Equal(t, "error", entry.Level, "level should be normalised from ERROR to error")
	assert.Equal(t, "auth-service", entry.Service)
	assert.Equal(t, "login failed", entry.Message)
	assert.Equal(t, "abc123", entry.Fields["user_id"])
	assert.Equal(t, float64(3), entry.Fields["attempt"], "JSON numbers decode as float64")
	assert.Equal(t, string(payload), entry.RawSource)
	assert.Equal(t, "test-topic", entry.Source.Topic)
	assert.Equal(t, int64(42), entry.Source.Offset)
}

func TestParse_PlainTextFallback(t *testing.T) {
	t.Parallel()
	payload := []byte("2026-03-21 10:30:00 ERROR something went wrong")
	msg := domain.RawMessage{
		Payload:   payload,
		Topic:     "logs",
		Partition: 1,
		Offset:    99,
	}

	entry, err := Parse(msg)

	assert.Error(t, err, "plain-text input should return a parse error")
	assert.NotEmpty(t, entry.ID, "ID should still be generated on parse error")
	assert.Equal(t, "unknown", entry.Level)
	assert.Equal(t, "", entry.Service, "service should be empty for plain-text fallback")
	assert.Equal(t, "2026-03-21 10:30:00 ERROR something went wrong", entry.Message)
	assert.Equal(t, string(payload), entry.RawSource)
	assert.Equal(t, int64(99), entry.Source.Offset)
}

func TestParse_MalformedJSON(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"timestamp":"2026-03-21T10:30:00Z","level":"ER`)
	msg := domain.RawMessage{Payload: payload}

	// Must not panic
	entry, err := Parse(msg)

	assert.Error(t, err, "truncated JSON should return a parse error")
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "unknown", entry.Level, "level should be unknown for malformed JSON fallback")
	assert.Equal(t, string(payload), entry.Message)
}

func TestParse_EmptyPayload(t *testing.T) {
	t.Parallel()
	payload := []byte("")
	msg := domain.RawMessage{Payload: payload}

	// Must not panic
	entry, err := Parse(msg)

	assert.Error(t, err, "empty payload should return a parse error")
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "unknown", entry.Level)
}

func TestParse_JSONMissingTimestamp(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"level":"info","service":"api","message":"request received"}`)
	msg := domain.RawMessage{Payload: payload}

	entry, err := Parse(msg)

	assert.NoError(t, err)
	assert.False(t, entry.Timestamp.IsZero(), "Timestamp should use time.Now() fallback when missing from JSON")
	assert.Equal(t, "info", entry.Level)
	assert.Equal(t, "api", entry.Service)
}

func TestParse_SourcePreserved(t *testing.T) {
	t.Parallel()
	msg := domain.RawMessage{
		Payload:   []byte(`{"level":"info","message":"test"}`),
		Topic:     "my-topic",
		Partition: 7,
		Offset:    12345,
		Timestamp: time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
	}

	entry, err := Parse(msg)

	assert.NoError(t, err)
	assert.Equal(t, "my-topic", entry.Source.Topic)
	assert.Equal(t, int32(7), entry.Source.Partition)
	assert.Equal(t, int64(12345), entry.Source.Offset)
}

func TestProcessMessage_ErrorMetering(t *testing.T) {
	// Read the counter value before
	before := testutil.ToFloat64(metrics.ParseErrorsTotal)

	// Create a plain-text (non-JSON) message that will trigger a parse error
	msg := domain.RawMessage{
		Payload: []byte("this is not JSON"),
		Topic:   "test",
	}

	// Create a no-op Zap logger (suppresses output in tests)
	logger := zap.NewNop()

	// Call ProcessMessage — should increment ParseErrorsTotal
	entry := ProcessMessage(msg, logger)

	// Assert the entry is still valid (never dropped)
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "unknown", entry.Level)
	assert.Equal(t, "this is not JSON", entry.Message)

	// Assert ParseErrorsTotal was incremented
	after := testutil.ToFloat64(metrics.ParseErrorsTotal)
	assert.Equal(t, before+1, after, "ParseErrorsTotal should increment by 1 on parse error")

	// Also test that valid JSON does NOT increment the counter
	beforeValid := testutil.ToFloat64(metrics.ParseErrorsTotal)
	validMsg := domain.RawMessage{
		Payload: []byte(`{"level":"info","message":"ok"}`),
	}
	_ = ProcessMessage(validMsg, logger)
	afterValid := testutil.ToFloat64(metrics.ParseErrorsTotal)
	assert.Equal(t, beforeValid, afterValid, "ParseErrorsTotal should NOT increment on valid JSON")
}
