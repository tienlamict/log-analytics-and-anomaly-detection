package elasticsearch

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// bulkSuccessResponse returns a successful bulk API response.
func bulkSuccessResponse() *http.Response {
	body := `{"errors":false,"items":[{"index":{"_id":"test","status":200,"result":"created"}}]}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Elastic-Product": []string{"Elasticsearch"},
			"Content-Type":      []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
}

// bulkFailureResponse returns a bulk API response with a per-item error.
func bulkFailureResponse() *http.Response {
	body := `{"errors":true,"items":[{"index":{"_id":"test","status":400,"error":{"type":"mapper_parsing_exception","reason":"failed to parse"}}}]}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Elastic-Product": []string{"Elasticsearch"},
			"Content-Type":      []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestLogIndexer_IndexLog_Success(t *testing.T) {
	var requestCount int32

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&requestCount, 1)
		return bulkSuccessResponse(), nil
	})

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: transport,
	})
	require.NoError(t, err)

	logger := zap.NewNop()
	indexer, err := NewLogIndexer(client, logger)
	require.NoError(t, err)

	entry := domain.LogEntry{
		ID:        "test-log-001",
		Timestamp: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		Level:     "info",
		Service:   "api",
		Message:   "request processed",
		Source: domain.RawMessage{
			Partition: 0,
			Offset:    42,
		},
	}

	err = indexer.IndexLog(t.Context(), entry)
	require.NoError(t, err)

	err = indexer.Close(t.Context())
	require.NoError(t, err)

	require.Greater(t, atomic.LoadInt32(&requestCount), int32(0), "expected at least one bulk request")
}

func TestLogIndexer_IndexLog_DeterministicID(t *testing.T) {
	id1 := sha256HexID("0:42")
	id2 := sha256HexID("0:42")
	require.Equal(t, id1, id2, "same input must produce same ID")

	id3 := sha256HexID("0:43")
	require.NotEqual(t, id1, id3, "different offsets must produce different IDs")
}

func TestLogIndexer_IndexLog_DailyIndex(t *testing.T) {
	var capturedBody string

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		capturedBody = string(body)
		return bulkSuccessResponse(), nil
	})

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: transport,
	})
	require.NoError(t, err)

	logger := zap.NewNop()
	indexer, err := NewLogIndexer(client, logger)
	require.NoError(t, err)

	entry := domain.LogEntry{
		ID:        "test-log-002",
		Timestamp: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		Level:     "info",
		Service:   "api",
		Message:   "daily index test",
		Source: domain.RawMessage{
			Partition: 0,
			Offset:    100,
		},
	}

	err = indexer.IndexLog(t.Context(), entry)
	require.NoError(t, err)

	err = indexer.Close(t.Context())
	require.NoError(t, err)

	require.True(t, strings.Contains(capturedBody, `"logs-2026.03.21"`),
		"bulk NDJSON action line should contain logs-2026.03.21, got: %q", capturedBody)
}

func TestLogIndexer_OnFailure_IncrementsMetric(t *testing.T) {
	before := testutil.ToFloat64(metrics.ESWriteErrorsTotal)

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return bulkFailureResponse(), nil
	})

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: transport,
	})
	require.NoError(t, err)

	logger := zap.NewNop()
	indexer, err := NewLogIndexer(client, logger)
	require.NoError(t, err)

	entry := domain.LogEntry{
		ID:        "test-log-003",
		Timestamp: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		Level:     "error",
		Service:   "api",
		Message:   "failure test",
		Source: domain.RawMessage{
			Partition: 0,
			Offset:    200,
		},
	}

	err = indexer.IndexLog(t.Context(), entry)
	require.NoError(t, err)

	err = indexer.Close(t.Context())
	require.NoError(t, err)

	after := testutil.ToFloat64(metrics.ESWriteErrorsTotal)
	require.Greater(t, after, before, "ESWriteErrorsTotal should have been incremented on per-item failure")
}
