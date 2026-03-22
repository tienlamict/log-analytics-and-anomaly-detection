package elasticsearch

import (
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

func TestAnomalyIndexer_IndexAnomaly_Success(t *testing.T) {
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
	indexer, err := NewAnomalyIndexer(client, logger)
	require.NoError(t, err)

	anomaly := domain.Anomaly{
		ID:          "test-anomaly-001",
		RuleID:      "error_rate",
		Severity:    "high",
		Service:     "api",
		Description: "error rate spike detected",
		DetectedAt:  time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	err = indexer.IndexAnomaly(t.Context(), anomaly)
	require.NoError(t, err)

	err = indexer.Close(t.Context())
	require.NoError(t, err)

	require.Greater(t, atomic.LoadInt32(&requestCount), int32(0), "expected at least one bulk request")
}

func TestAnomalyIndexer_IndexAnomaly_UsesAnomalyID(t *testing.T) {
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
	indexer, err := NewAnomalyIndexer(client, logger)
	require.NoError(t, err)

	const anomalyID = "test-anomaly-uuid-42"
	anomaly := domain.Anomaly{
		ID:          anomalyID,
		RuleID:      "auth_burst",
		Severity:    "critical",
		Service:     "auth",
		Description: "auth failure burst",
		DetectedAt:  time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC),
	}

	err = indexer.IndexAnomaly(t.Context(), anomaly)
	require.NoError(t, err)

	err = indexer.Close(t.Context())
	require.NoError(t, err)

	// The bulk NDJSON action line contains the document ID.
	require.True(t, strings.Contains(capturedBody, anomalyID),
		"bulk NDJSON should contain anomaly ID %q, got: %q", anomalyID, capturedBody)
}

func TestAnomalyIndexer_OnFailure_IncrementsMetric(t *testing.T) {
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
	indexer, err := NewAnomalyIndexer(client, logger)
	require.NoError(t, err)

	anomaly := domain.Anomaly{
		ID:          "test-anomaly-failure",
		RuleID:      "error_rate",
		Severity:    "medium",
		Service:     "worker",
		Description: "failure metric test",
		DetectedAt:  time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
	}

	err = indexer.IndexAnomaly(t.Context(), anomaly)
	require.NoError(t, err)

	err = indexer.Close(t.Context())
	require.NoError(t, err)

	after := testutil.ToFloat64(metrics.ESWriteErrorsTotal)
	require.Greater(t, after, before, "ESWriteErrorsTotal should have been incremented on per-item failure")
}
