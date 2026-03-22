package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// LogIndexer implements domain.LogStore by bulk-indexing log entries to daily
// logs-{YYYY.MM.DD} indices using deterministic sha256 document IDs.
type LogIndexer struct {
	indexer esutil.BulkIndexer
	logger  *zap.Logger
}

// NewLogIndexer constructs a LogIndexer backed by an esutil.BulkIndexer.
// The indexer is configured to flush at 5 MB or every 5 seconds.
func NewLogIndexer(client *elasticsearch.TypedClient, logger *zap.Logger) (*LogIndexer, error) {
	indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        client,
		Index:         "", // set per-item
		NumWorkers:    2,
		FlushBytes:    5_000_000,
		FlushInterval: 5 * time.Second,
		OnError: func(ctx context.Context, err error) {
			logger.Error("bulk indexer error", zap.Error(err))
			metrics.ESWriteErrorsTotal.Inc()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create log bulk indexer: %w", err)
	}
	return &LogIndexer{indexer: indexer, logger: logger}, nil
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

// SearchLogs is not implemented in Phase 3; returns not-implemented error (Phase 4).
func (li *LogIndexer) SearchLogs(_ context.Context, _ domain.LogQuery) ([]domain.LogEntry, int64, error) {
	return nil, 0, errors.New("not implemented")
}

// GetLog is not implemented in Phase 3; returns not-implemented error (Phase 4).
func (li *LogIndexer) GetLog(_ context.Context, _ string) (domain.LogEntry, error) {
	return domain.LogEntry{}, errors.New("not implemented")
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
