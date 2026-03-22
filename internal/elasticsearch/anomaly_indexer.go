package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esutil"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// AnomalyIndexer implements domain.AnomalyStore by bulk-indexing anomalies to the
// fixed "anomalies" index using the anomaly UUID as the document ID.
type AnomalyIndexer struct {
	client  *elasticsearch.TypedClient
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
	return &AnomalyIndexer{client: client, indexer: indexer, logger: logger}, nil
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

// SearchAnomalies executes a bool/term/range query against the anomalies index.
// AnomalyQuery.Type maps to the ES "rule_id" field. Time range uses the "detected_at" field.
func (ai *AnomalyIndexer) SearchAnomalies(ctx context.Context, q domain.AnomalyQuery) ([]domain.Anomaly, int64, error) {
	var filters []types.Query

	if q.Type != "" {
		filters = append(filters, types.Query{
			Term: map[string]types.TermQuery{"rule_id": {Value: q.Type}},
		})
	}
	if q.Service != "" {
		filters = append(filters, types.Query{
			Term: map[string]types.TermQuery{"service": {Value: q.Service}},
		})
	}
	if q.Severity != "" {
		filters = append(filters, types.Query{
			Term: map[string]types.TermQuery{"severity": {Value: q.Severity}},
		})
	}
	if !q.From.IsZero() || !q.To.IsZero() {
		drq := types.DateRangeQuery{}
		if !q.From.IsZero() {
			s := q.From.UTC().Format(time.RFC3339)
			drq.Gte = &s
		}
		if !q.To.IsZero() {
			s := q.To.UTC().Format(time.RFC3339)
			drq.Lte = &s
		}
		filters = append(filters, types.Query{
			Range: map[string]types.RangeQuery{"detected_at": drq},
		})
	}

	// Clamp pagination
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.Size
	if size < 1 {
		size = 20
	}
	if size > 1000 {
		size = 1000
	}
	from := (page - 1) * size

	req := &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{Filter: filters},
		},
		From: &from,
		Size: &size,
	}

	res, err := ai.client.Search().Index("anomalies").Request(req).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("es search anomalies: %w", err)
	}

	var total int64
	if res.Hits.Total != nil {
		total = res.Hits.Total.Value
	}

	anomalies := make([]domain.Anomaly, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var anomaly domain.Anomaly
		if err := json.Unmarshal(hit.Source_, &anomaly); err != nil {
			return nil, 0, fmt.Errorf("unmarshal anomaly: %w", err)
		}
		if hit.Id_ != nil {
			anomaly.ID = *hit.Id_
		}
		anomalies = append(anomalies, anomaly)
	}

	return anomalies, total, nil
}

// GetAnomaly retrieves a single anomaly by document ID using the ES Get API.
// Returns domain.ErrNotFound if the document does not exist.
func (ai *AnomalyIndexer) GetAnomaly(ctx context.Context, id string) (domain.Anomaly, error) {
	res, err := ai.client.Get("anomalies", id).Do(ctx)
	if err != nil {
		return domain.Anomaly{}, fmt.Errorf("es get anomaly: %w", err)
	}
	if !res.Found {
		return domain.Anomaly{}, domain.ErrNotFound
	}

	var anomaly domain.Anomaly
	if err := json.Unmarshal(res.Source_, &anomaly); err != nil {
		return domain.Anomaly{}, fmt.Errorf("unmarshal anomaly: %w", err)
	}
	anomaly.ID = res.Id_
	return anomaly, nil
}

// Close flushes any buffered anomaly items and shuts down the bulk indexer workers.
// Call with a deadline context to avoid waiting indefinitely on shutdown.
func (ai *AnomalyIndexer) Close(ctx context.Context) error {
	return ai.indexer.Close(ctx)
}
