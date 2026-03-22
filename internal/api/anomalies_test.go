package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/log-analytics/server/internal/domain"
)

func TestHandleListAnomalies_ValidQuery(t *testing.T) {
	store := &mockAnomalyStore{
		anomalies: []domain.Anomaly{
			{
				ID:          "anom-1",
				RuleID:      "error_rate",
				Severity:    "high",
				Service:     "api",
				Description: "error spike",
				DetectedAt:  time.Now(),
			},
		},
		total: 1,
	}
	s := newTestServer(&mockLogStore{}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies?type=error_rate&severity=high", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body PagedResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, int64(1), body.Total)
	assert.Equal(t, 1, body.Page)
	assert.Equal(t, 20, body.Size)

	data, ok := body.Data.([]interface{})
	require.True(t, ok, "Data should be a JSON array")
	assert.Len(t, data, 1)
}

func TestHandleListAnomalies_WithPagination(t *testing.T) {
	store := &mockAnomalyStore{
		anomalies: []domain.Anomaly{
			{ID: "anom-1", RuleID: "rule1", Severity: "low", Service: "svc", DetectedAt: time.Now()},
			{ID: "anom-2", RuleID: "rule2", Severity: "medium", Service: "svc", DetectedAt: time.Now()},
		},
		total: 30,
	}
	s := newTestServer(&mockLogStore{}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies?page=3&size=5", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body PagedResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, 3, body.Page)
	assert.Equal(t, 5, body.Size)
	assert.Equal(t, int64(30), body.Total)
}

func TestHandleListAnomalies_InvalidPage(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies?page=xyz", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleListAnomalies_StoreError(t *testing.T) {
	store := &mockAnomalyStore{
		err: errors.New("es error"),
	}
	s := newTestServer(&mockLogStore{}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleGetAnomaly_Found(t *testing.T) {
	store := &mockAnomalyStore{
		anomalies: []domain.Anomaly{
			{ID: "anom-abc", RuleID: "error_rate", Severity: "high", Service: "api", DetectedAt: time.Now()},
		},
	}
	s := newTestServer(&mockLogStore{}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies/anom-abc", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body domain.Anomaly
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "anom-abc", body.ID)
}

func TestHandleGetAnomaly_NotFound(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies/nonexistent", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body.Error, "anomaly not found")
}

func TestHandleGetAnomaly_StoreError(t *testing.T) {
	store := &mockAnomalyStore{
		err: errors.New("es timeout"),
	}
	s := newTestServer(&mockLogStore{}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anomalies/anom-abc", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
