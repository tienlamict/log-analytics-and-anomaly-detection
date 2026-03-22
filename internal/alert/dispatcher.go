package alert

import (
	"context"

	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
)

// Dispatcher consumes detected anomalies from the detector engine and fans them
// out to both the AnomalyStore (persistence) and an AlertChannel (notification).
// Failure in one output does not prevent the other from receiving the anomaly.
type Dispatcher struct {
	anomalyCh    <-chan domain.Anomaly
	anomalyStore domain.AnomalyStore
	alertChannel domain.AlertChannel
	logger       *zap.Logger
}

// NewDispatcher constructs a Dispatcher wired to the provided anomaly channel,
// store, and alert channel. The anomaly channel must only emit post-cooldown
// anomalies (cooldown is handled upstream by DetectorEngine).
func NewDispatcher(anomalyCh <-chan domain.Anomaly, anomalyStore domain.AnomalyStore, alertChannel domain.AlertChannel, logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		anomalyCh:    anomalyCh,
		anomalyStore: anomalyStore,
		alertChannel: alertChannel,
		logger:       logger,
	}
}

// Run reads anomalies from the channel and forwards each to both the AnomalyStore
// and AlertChannel. Errors from either output are logged but do not halt dispatch.
// Returns nil when the anomaly channel is closed; returns ctx.Err() on cancellation.
func (d *Dispatcher) Run(ctx context.Context) error {
	for {
		select {
		case anomaly, ok := <-d.anomalyCh:
			if !ok {
				return nil // channel closed, clean exit
			}
			// Both operations happen regardless of the other's outcome.
			if err := d.anomalyStore.IndexAnomaly(ctx, anomaly); err != nil {
				d.logger.Error("index anomaly failed",
					zap.Error(err),
					zap.String("anomaly_id", anomaly.ID),
					zap.String("rule_id", anomaly.RuleID),
				)
				// do not return — continue dispatching
			}
			if err := d.alertChannel.Send(ctx, anomaly); err != nil {
				d.logger.Error("alert send failed",
					zap.Error(err),
					zap.String("anomaly_id", anomaly.ID),
					zap.String("rule_id", anomaly.RuleID),
				)
				// do not return — continue dispatching
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
