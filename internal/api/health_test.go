package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgmetrics "github.com/log-analytics/server/internal/metrics"
)

// TestHandleHealth_ESUnavailable verifies that a nil ES client causes /health to return 503.
// The nil-guard in handleHealth prevents a nil-pointer panic and returns "unhealthy".
func TestHandleHealth_ESUnavailable(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body.Error, "unhealthy")
}

// TestHandleReady_NotReady verifies that /ready returns 503 before SetReady is called.
func TestHandleReady_NotReady(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHandleReady_Ready verifies that /ready returns 200 after SetReady is called.
func TestHandleReady_Ready(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})
	s.SetReady()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "ready", body["status"])
}

// TestHandleMetrics verifies that /metrics serves Prometheus text with expected metric names.
// CounterVec metrics (logs_consumed_total, anomalies_detected_total) only appear in the
// text output after at least one label combination is observed. We observe one value on
// each Vec before hitting /metrics so the metric names appear in the output.
func TestHandleMetrics(t *testing.T) {
	// Observe one value on each Vec so it appears in the scrape output.
	pkgmetrics.LogsConsumedTotal.WithLabelValues("test-topic", "0").Add(0)
	pkgmetrics.AnomaliesDetectedTotal.WithLabelValues("test-rule").Add(0)

	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.True(t, strings.Contains(body, "logs_consumed_total"),
		"expected logs_consumed_total in metrics output")
	assert.True(t, strings.Contains(body, "anomalies_detected_total"),
		"expected anomalies_detected_total in metrics output")
}
