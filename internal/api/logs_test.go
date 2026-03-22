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

func TestHandleListLogs_ValidQuery(t *testing.T) {
	store := &mockLogStore{
		entries: []domain.LogEntry{
			{ID: "log-1", Service: "api", Level: "error", Message: "test error", Timestamp: time.Now()},
		},
		total: 1,
	}
	s := newTestServer(store, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?service=api&level=error", nil)
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

func TestHandleListLogs_WithPagination(t *testing.T) {
	store := &mockLogStore{
		entries: []domain.LogEntry{
			{ID: "log-1", Service: "svc", Level: "info", Message: "msg1", Timestamp: time.Now()},
			{ID: "log-2", Service: "svc", Level: "info", Message: "msg2", Timestamp: time.Now()},
		},
		total: 50,
	}
	s := newTestServer(store, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?page=2&size=10", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body PagedResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, 2, body.Page)
	assert.Equal(t, 10, body.Size)
	assert.Equal(t, int64(50), body.Total)
}

func TestHandleListLogs_InvalidPage(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?page=abc", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body.Error, "page")
}

func TestHandleListLogs_InvalidSize(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?size=abc", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleListLogs_InvalidFrom(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?from=not-a-date", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body.Error, "from")
}

func TestHandleListLogs_StoreError(t *testing.T) {
	store := &mockLogStore{
		err: errors.New("es connection failed"),
	}
	s := newTestServer(store, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	// Must NOT expose the raw ES error; should be a generic message.
	assert.Equal(t, "internal error", body.Error)
}

func TestHandleGetLog_Found(t *testing.T) {
	store := &mockLogStore{
		entries: []domain.LogEntry{
			{ID: "abc-123", Service: "svc", Level: "info", Message: "found", Timestamp: time.Now()},
		},
	}
	s := newTestServer(store, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/abc-123", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body domain.LogEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "abc-123", body.ID)
}

func TestHandleGetLog_NotFound(t *testing.T) {
	s := newTestServer(&mockLogStore{}, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/nonexistent", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var body ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "log entry not found", body.Error)
}

func TestHandleGetLog_StoreError(t *testing.T) {
	store := &mockLogStore{
		err: errors.New("es timeout"),
	}
	s := newTestServer(store, &mockAnomalyStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/abc-123", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
