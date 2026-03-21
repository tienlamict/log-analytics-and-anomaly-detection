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
	ID        string
	Timestamp time.Time
	Level     string         // normalised: "error", "warn", "info", "debug", "unknown"
	Service   string
	Message   string
	Fields    map[string]any
	RawSource string
	Source    RawMessage
}

type Anomaly struct {
	ID          string
	RuleID      string
	Severity    string        // "low", "medium", "high", "critical"
	Service     string
	Description string
	Evidence    []LogEntry
	DetectedAt  time.Time
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
