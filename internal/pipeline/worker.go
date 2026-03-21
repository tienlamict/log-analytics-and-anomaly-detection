package pipeline

import (
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// ProcessMessage calls Parse on the given RawMessage, increments the
// parse_errors_total metric and logs a warning on parse error, then
// returns the LogEntry regardless. It never drops messages.
func ProcessMessage(msg domain.RawMessage, logger *zap.Logger) domain.LogEntry {
	entry, err := Parse(msg)
	if err != nil {
		metrics.ParseErrorsTotal.Inc()
		logger.Warn("parse error, stored as plain-text fallback",
			zap.Error(err),
			zap.String("topic", msg.Topic),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
		)
	}
	return entry
}
