package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgmetrics "github.com/log-analytics/server/internal/metrics"
)

func TestAllMetricsRegistered(t *testing.T) {
	// Verify all 6 metrics are registered by attempting to re-register each
	// collector using the exact same variable exported from internal/metrics.
	// The blank import triggers init() which registers them all.
	// Attempting to re-register returns AlreadyRegisteredError, proving
	// the metric is present in the default registry.
	collectors := map[string]prometheus.Collector{
		"logs_consumed_total":              pkgmetrics.LogsConsumedTotal,
		"logs_processed_duration_seconds":  pkgmetrics.LogsProcessedDuration,
		"anomalies_detected_total":         pkgmetrics.AnomaliesDetectedTotal,
		"elasticsearch_write_errors_total": pkgmetrics.ESWriteErrorsTotal,
		"kafka_consumer_lag":               pkgmetrics.KafkaConsumerLag,
		"parse_errors_total":               pkgmetrics.ParseErrorsTotal,
	}

	for name, collector := range collectors {
		err := prometheus.DefaultRegisterer.Register(collector)
		require.Error(t, err, "expected registration error for %q (metric should already be registered)", name)
		var are prometheus.AlreadyRegisteredError
		assert.ErrorAs(t, err, &are, "metric %q should be already registered, got: %v", name, err)
		_ = strings.Contains(err.Error(), name) // ensure name is in scope for debugging
	}
}
