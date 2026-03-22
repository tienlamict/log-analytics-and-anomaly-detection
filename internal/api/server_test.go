package api

import (
	"context"
	"testing"

	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// mockLogStore is an in-memory implementation of domain.LogStore for tests.
type mockLogStore struct {
	entries []domain.LogEntry
	total   int64
	err     error
}

func (m *mockLogStore) IndexLog(_ context.Context, _ domain.LogEntry) error { return m.err }

func (m *mockLogStore) SearchLogs(_ context.Context, _ domain.LogQuery) ([]domain.LogEntry, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.entries, m.total, nil
}

func (m *mockLogStore) GetLog(_ context.Context, id string) (domain.LogEntry, error) {
	if m.err != nil {
		return domain.LogEntry{}, m.err
	}
	for _, e := range m.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.LogEntry{}, domain.ErrNotFound
}

// mockAnomalyStore is an in-memory implementation of domain.AnomalyStore for tests.
type mockAnomalyStore struct {
	anomalies []domain.Anomaly
	total     int64
	err       error
}

func (m *mockAnomalyStore) IndexAnomaly(_ context.Context, _ domain.Anomaly) error { return m.err }

func (m *mockAnomalyStore) SearchAnomalies(_ context.Context, _ domain.AnomalyQuery) ([]domain.Anomaly, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.anomalies, m.total, nil
}

func (m *mockAnomalyStore) GetAnomaly(_ context.Context, id string) (domain.Anomaly, error) {
	if m.err != nil {
		return domain.Anomaly{}, m.err
	}
	for _, a := range m.anomalies {
		if a.ID == id {
			return a, nil
		}
	}
	return domain.Anomaly{}, domain.ErrNotFound
}

// newTestServer constructs a Server with mock stores and a no-op logger.
// esClient is nil so health endpoint returns 503; tests that need different
// health behaviour should call NewServer directly.
func newTestServer(logStore domain.LogStore, anomStore domain.AnomalyStore) *Server {
	return NewServer(logStore, anomStore, nil, zap.NewNop(), 0)
}
