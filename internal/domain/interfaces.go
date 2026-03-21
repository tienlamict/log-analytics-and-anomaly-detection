package domain

import "context"

type MessageConsumer interface {
	Run(ctx context.Context) error
	Messages() <-chan RawMessage
}

type Parser interface {
	Parse(msg RawMessage) (LogEntry, error)
}

type Enricher interface {
	Enrich(entry LogEntry) LogEntry
}

type Detector interface {
	Name() string
	Evaluate(entry LogEntry) (Anomaly, bool)
	Reset()
}

type AlertChannel interface {
	Send(ctx context.Context, anomaly Anomaly) error
	Name() string
}

type LogStore interface {
	IndexLog(ctx context.Context, entry LogEntry) error
	SearchLogs(ctx context.Context, query LogQuery) ([]LogEntry, int64, error)
	GetLog(ctx context.Context, id string) (LogEntry, error)
}

type AnomalyStore interface {
	IndexAnomaly(ctx context.Context, anomaly Anomaly) error
	SearchAnomalies(ctx context.Context, query AnomalyQuery) ([]Anomaly, int64, error)
	GetAnomaly(ctx context.Context, id string) (Anomaly, error)
}
