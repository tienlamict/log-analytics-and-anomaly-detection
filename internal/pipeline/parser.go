package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/log-analytics/server/internal/domain"
)

// NormaliseLevel converts raw log level strings to canonical values:
// "error", "warn", "info", "debug", or "unknown".
func NormaliseLevel(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "ERROR", "ERR", "FATAL", "CRITICAL", "CRIT":
		return "error"
	case "WARN", "WARNING":
		return "warn"
	case "INFO", "INFORMATION":
		return "info"
	case "DEBUG", "TRACE", "VERBOSE":
		return "debug"
	default:
		return "unknown"
	}
}

// Parse attempts to parse msg.Payload as structured JSON. On success it
// returns a fully-populated LogEntry with nil error. On failure it returns
// a plain-text fallback LogEntry alongside a non-nil error. Parse ALWAYS
// returns a valid (non-nil) LogEntry — it never drops messages.
func Parse(msg domain.RawMessage) (domain.LogEntry, error) {
	var structured struct {
		Timestamp string         `json:"timestamp"`
		Level     string         `json:"level"`
		Service   string         `json:"service"`
		Message   string         `json:"message"`
		Fields    map[string]any `json:"fields"`
	}

	if err := json.Unmarshal(msg.Payload, &structured); err != nil {
		// Plain-text fallback: store the raw payload as the message.
		entry := domain.LogEntry{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			Level:     "unknown",
			Service:   "",
			Message:   string(msg.Payload),
			Fields:    nil,
			RawSource: string(msg.Payload),
			Source:    msg,
		}
		return entry, fmt.Errorf("not valid JSON, stored as plain-text: %w", err)
	}

	ts, err := time.Parse(time.RFC3339, structured.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	entry := domain.LogEntry{
		ID:        uuid.NewString(),
		Timestamp: ts,
		Level:     NormaliseLevel(structured.Level),
		Service:   structured.Service,
		Message:   structured.Message,
		Fields:    structured.Fields,
		RawSource: string(msg.Payload),
		Source:    msg,
	}
	return entry, nil
}

// LogParser implements the domain.Parser interface using the package-level
// Parse function.
type LogParser struct{}

// NewParser constructs a new LogParser.
func NewParser() *LogParser {
	return &LogParser{}
}

// Parse satisfies domain.Parser.
func (p *LogParser) Parse(msg domain.RawMessage) (domain.LogEntry, error) {
	return Parse(msg)
}
