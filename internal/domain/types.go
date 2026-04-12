package domain

import "time"

type RawMessage struct {
	Payload   []byte
	Topic     string
	Partition int32
	Offset    int64
	Timestamp time.Time
}

type LogEntry struct {
	ID        string         `json:"id,omitempty"`
	Timestamp time.Time      `json:"@timestamp"`
	Level     string         `json:"level"`     // normalised: "error", "warn", "info", "debug", "unknown"
	Service   string         `json:"service"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	RawSource string         `json:"raw_source,omitempty"`
	Source    RawMessage      `json:"-"`
}

type Anomaly struct {
	ID          string     `json:"id,omitempty"`
	RuleID      string     `json:"rule_id"`
	Severity    string     `json:"severity"`   // "low", "medium", "high", "critical"
	Service     string     `json:"service"`
	Description string     `json:"description"`
	Evidence    []LogEntry `json:"evidence,omitempty"`
	DetectedAt  time.Time  `json:"detected_at"`
}

type LogQuery struct {
	Service  string
	Level    string
	From     time.Time
	To       time.Time
	Page     int
	Size     int
}

type AnomalyQuery struct {
	Type     string
	Service  string
	Severity string
	From     time.Time
	To       time.Time
	Page     int
	Size     int
}
