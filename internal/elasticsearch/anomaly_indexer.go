package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esutil"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// AnomalyIndexer implements domain.AnomalyStore by bulk-indexing anomalies to the
// fixed "anomalies" index using the anomaly UUID as the document ID.
type AnomalyIndexer struct {
	indexer esutil.BulkIndexer
	logger  *zap.Logger
}

// NewAnomalyIndexer constructs an AnomalyIndexer backed by an esutil.BulkIndexer.
// Uses a fixed "anomalies" index with 1 worker (lower volume than logs).
func NewAnomalyIndexer(client *elasticsearch.TypedClient, logger *zap.Logger) (*AnomalyIndexer, error) {
	indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        client,
		Index:         "anomalies",
		NumWorkers:    1,
		FlushBytes:    1_000_000,
		FlushInterval: 3 * time.Second,
		OnError: func(ctx context.Context, err error) {
			logger.Error("anomaly bulk indexer error", zap.Error(err))
			metrics.ESWriteErrorsTotal.Inc()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create anomaly bulk indexer: %w", err)
	}
	return &AnomalyIndexer{indexer: indexer, logger: logger}, nil
}

// IndexAnomaly adds an anomaly to the bulk indexer queue.
// The document ID is anomaly.ID (UUID assigned by DetectorEngine).
// Per-item write failures increment ESWriteErrorsTotal and are logged; the pipeline continues.
func (ai *AnomalyIndexer) IndexAnomaly(ctx context.Context, anomaly domain.Anomaly) error {
	body, err := json.Marshal(anomaly)
	if err != nil {
		return fmt.Errorf("marshal anomaly: %w", err)
	}

	return ai.indexer.Add(ctx, esutil.BulkIndexerItem{
		Action:     "index",
		DocumentID: anomaly.ID,
		Body:       bytes.NewReader(body),
		OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
			if err != nil {
				ai.logger.Error("es anomaly index failure", zap.Error(err))
			} else {
				ai.logger.Error("es anomaly index failure",
					zap.String("type", res.Error.Type),
					zap.String("reason", res.Error.Reason),
				)
			}
			metrics.ESWriteErrorsTotal.Inc()
		},
	})
}

// SearchAnomalies is not implemented in Phase 3; returns not-implemented error (Phase 4).
func (ai *AnomalyIndexer) SearchAnomalies(_ context.Context, _ domain.AnomalyQuery) ([]domain.Anomaly, int64, error) {
	return nil, 0, errors.New("not implemented")
}

// GetAnomaly is not implemented in Phase 3; returns not-implemented error (Phase 4).
func (ai *AnomalyIndexer) GetAnomaly(_ context.Context, _ string) (domain.Anomaly, error) {
	return domain.Anomaly{}, errors.New("not implemented")
}

// Close flushes any buffered anomaly items and shuts down the bulk indexer workers.
// Call with a deadline context to avoid waiting indefinitely on shutdown.
func (ai *AnomalyIndexer) Close(ctx context.Context) error {
	return ai.indexer.Close(ctx)
}
