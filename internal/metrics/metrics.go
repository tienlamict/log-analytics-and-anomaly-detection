package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	LogsConsumedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_consumed_total",
			Help: "Total Kafka records consumed.",
		},
		[]string{"topic", "partition"},
	)

	LogsProcessedDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "logs_processed_duration_seconds",
		Help:    "End-to-end processing latency per log entry.",
		Buckets: prometheus.DefBuckets,
	})

	AnomaliesDetectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "anomalies_detected_total",
			Help: "Anomalies detected by rule.",
		},
		[]string{"rule"},
	)

	ESWriteErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "elasticsearch_write_errors_total",
		Help: "Elasticsearch bulk indexer write failures.",
	})

	KafkaConsumerLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Kafka consumer group lag per partition.",
		},
		[]string{"topic", "partition"},
	)

	ParseErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "parse_errors_total",
		Help: "Total log parse errors (plain-text fallback triggered).",
	})
)

func init() {
	prometheus.MustRegister(
		LogsConsumedTotal,
		LogsProcessedDuration,
		AnomaliesDetectedTotal,
		ESWriteErrorsTotal,
		KafkaConsumerLag,
		ParseErrorsTotal,
	)
}
