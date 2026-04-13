package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// LogIndexer implements domain.LogStore by bulk-indexing log entries to daily
// logs-{YYYY.MM.DD} indices using deterministic sha256 document IDs.
type LogIndexer struct {
	client  *elasticsearch.TypedClient
	indexer esutil.BulkIndexer
	logger  *zap.Logger
}

// NewLogIndexer constructs a LogIndexer backed by an esutil.BulkIndexer.
// The indexer is configured to flush at 5 MB or every 5 seconds.
func NewLogIndexer(client *elasticsearch.TypedClient, logger *zap.Logger) (*LogIndexer, error) {
	indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        client,
		Index:         "", // set per-item
		NumWorkers:    4,
		FlushBytes:    10_000_000,
		FlushInterval: 5 * time.Second,
		OnError: func(ctx context.Context, err error) {
			logger.Error("bulk indexer error", zap.Error(err))
			metrics.ESWriteErrorsTotal.Inc()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create log bulk indexer: %w", err)
	}
	return &LogIndexer{client: client, indexer: indexer, logger: logger}, nil
}

// IndexLog adds a log entry to the bulk indexer queue for the appropriate daily index.
// The document ID is sha256(partition:offset), ensuring idempotent reprocessing.
// Per-item write failures increment ESWriteErrorsTotal and are logged; the pipeline continues.
func (li *LogIndexer) IndexLog(ctx context.Context, entry domain.LogEntry) error {
	indexName := "logs-" + entry.Timestamp.UTC().Format("2006.01.02")
	docID := sha256HexID(fmt.Sprintf("%d:%d", entry.Source.Partition, entry.Source.Offset))

	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}

	return li.indexer.Add(ctx, esutil.BulkIndexerItem{
		Action:     "index",
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader(body),
		OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
			if err != nil {
				li.logger.Error("es log index failure", zap.Error(err))
			} else {
				li.logger.Error("es log index failure",
					zap.String("type", res.Error.Type),
					zap.String("reason", res.Error.Reason),
				)
			}
			metrics.ESWriteErrorsTotal.Inc()
		},
	})
}

// SearchLogs executes a bool/term/range query against logs-* and returns paginated results.
// Filters are applied for service, level, and time range when non-zero.
func (li *LogIndexer) SearchLogs(ctx context.Context, q domain.LogQuery) ([]domain.LogEntry, int64, error) {
	var filters []types.Query

	if q.Service != "" {
		filters = append(filters, types.Query{
			Term: map[string]types.TermQuery{"service": {Value: q.Service}},
		})
	}
	if q.Level != "" {
		filters = append(filters, types.Query{
			Term: map[string]types.TermQuery{"level": {Value: q.Level}},
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
			Range: map[string]types.RangeQuery{"@timestamp": drq},
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

	trackAll := types.TrackHits(true)
	req := &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{Filter: filters},
		},
		From:           &from,
		Size:           &size,
		TrackTotalHits: trackAll,
	}

	res, err := li.client.Search().Index("logs-*").Request(req).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("es search logs: %w", err)
	}

	var total int64
	if res.Hits.Total != nil {
		total = res.Hits.Total.Value
	}

	entries := make([]domain.LogEntry, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var entry domain.LogEntry
		if err := json.Unmarshal(hit.Source_, &entry); err != nil {
			return nil, 0, fmt.Errorf("unmarshal log entry: %w", err)
		}
		if hit.Id_ != nil {
			entry.ID = *hit.Id_
		}
		entries = append(entries, entry)
	}

	return entries, total, nil
}

// GetLog retrieves a single log entry by document ID using an IDs query.
// Returns domain.ErrNotFound if no document matches.
// Uses IDs query (not the Get API) because the index pattern logs-* requires search.
func (li *LogIndexer) GetLog(ctx context.Context, id string) (domain.LogEntry, error) {
	req := &search.Request{
		Query: &types.Query{
			Ids: &types.IdsQuery{Values: []string{id}},
		},
	}
	res, err := li.client.Search().Index("logs-*").Request(req).Do(ctx)
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("es get log: %w", err)
	}
	if len(res.Hits.Hits) == 0 {
		return domain.LogEntry{}, domain.ErrNotFound
	}

	var entry domain.LogEntry
	if err := json.Unmarshal(res.Hits.Hits[0].Source_, &entry); err != nil {
		return domain.LogEntry{}, fmt.Errorf("unmarshal log entry: %w", err)
	}
	if res.Hits.Hits[0].Id_ != nil {
		entry.ID = *res.Hits.Hits[0].Id_
	}
	return entry, nil
}

// Close flushes any buffered items and shuts down the bulk indexer workers.
// Call with a deadline context to avoid waiting indefinitely on shutdown.
func (li *LogIndexer) Close(ctx context.Context) error {
	return li.indexer.Close(ctx)
}

// sha256HexID returns the hex-encoded sha256 digest of the input string.
// Used to produce deterministic, fixed-length Elasticsearch document IDs.
func sha256HexID(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
