package alert

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
)

// mockAnomalyStore records IndexAnomaly calls and optionally returns an error.
type mockAnomalyStore struct {
	indexed []domain.Anomaly
	err     error
}

func (m *mockAnomalyStore) IndexAnomaly(_ context.Context, a domain.Anomaly) error {
	m.indexed = append(m.indexed, a)
	return m.err
}

func (m *mockAnomalyStore) SearchAnomalies(_ context.Context, _ domain.AnomalyQuery) ([]domain.Anomaly, int64, error) {
	return nil, 0, nil
}

func (m *mockAnomalyStore) GetAnomaly(_ context.Context, _ string) (domain.Anomaly, error) {
	return domain.Anomaly{}, nil
}

// mockAlertChannel records Send calls and optionally returns an error.
type mockAlertChannel struct {
	sent []domain.Anomaly
	err  error
}

func (m *mockAlertChannel) Send(_ context.Context, a domain.Anomaly) error {
	m.sent = append(m.sent, a)
	return m.err
}

func (m *mockAlertChannel) Name() string { return "mock" }

func TestDispatcher_FanOut_BothReceive(t *testing.T) {
	ch := make(chan domain.Anomaly, 1)
	anomaly := domain.Anomaly{ID: "a1", RuleID: "rule-1"}
	ch <- anomaly
	close(ch)

	store := &mockAnomalyStore{}
	alert := &mockAlertChannel{}
	d := NewDispatcher(ch, store, alert, zap.NewNop())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(store.indexed) != 1 {
		t.Fatalf("expected 1 indexed anomaly, got %d", len(store.indexed))
	}
	if store.indexed[0].ID != anomaly.ID {
		t.Errorf("expected anomaly ID %q, got %q", anomaly.ID, store.indexed[0].ID)
	}

	if len(alert.sent) != 1 {
		t.Fatalf("expected 1 sent alert, got %d", len(alert.sent))
	}
	if alert.sent[0].ID != anomaly.ID {
		t.Errorf("expected anomaly ID %q, got %q", anomaly.ID, alert.sent[0].ID)
	}
}

func TestDispatcher_IndexFailure_AlertStillSent(t *testing.T) {
	ch := make(chan domain.Anomaly, 1)
	anomaly := domain.Anomaly{ID: "a2", RuleID: "rule-2"}
	ch <- anomaly
	close(ch)

	store := &mockAnomalyStore{err: errors.New("es down")}
	alert := &mockAlertChannel{}
	d := NewDispatcher(ch, store, alert, zap.NewNop())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(store.indexed) != 1 {
		t.Fatalf("expected IndexAnomaly to be called once (even though it returned error), got %d calls", len(store.indexed))
	}
	if len(alert.sent) != 1 {
		t.Fatalf("expected alert to be sent despite index failure, got %d sent", len(alert.sent))
	}
}

func TestDispatcher_AlertFailure_IndexStillDone(t *testing.T) {
	ch := make(chan domain.Anomaly, 1)
	anomaly := domain.Anomaly{ID: "a3", RuleID: "rule-3"}
	ch <- anomaly
	close(ch)

	store := &mockAnomalyStore{}
	alert := &mockAlertChannel{err: errors.New("alert channel down")}
	d := NewDispatcher(ch, store, alert, zap.NewNop())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(store.indexed) != 1 {
		t.Fatalf("expected index to succeed despite alert failure, got %d indexed", len(store.indexed))
	}
	if len(alert.sent) != 1 {
		t.Fatalf("expected Send to be called once (even though it returned error), got %d calls", len(alert.sent))
	}
}

func TestDispatcher_ChannelClosed_ExitsCleanly(t *testing.T) {
	ch := make(chan domain.Anomaly)
	close(ch) // closed with no anomalies

	store := &mockAnomalyStore{}
	alert := &mockAlertChannel{}
	d := NewDispatcher(ch, store, alert, zap.NewNop())

	err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("expected nil on channel close, got %v", err)
	}
}

func TestDispatcher_ContextCancelled_ExitsWithError(t *testing.T) {
	ch := make(chan domain.Anomaly) // never closed, never sends

	store := &mockAnomalyStore{}
	alert := &mockAlertChannel{}
	d := NewDispatcher(ch, store, alert, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := d.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDispatcher_MultipleAnomalies(t *testing.T) {
	ch := make(chan domain.Anomaly, 3)
	anomalies := []domain.Anomaly{
		{ID: "a1", RuleID: "rule-1"},
		{ID: "a2", RuleID: "rule-2"},
		{ID: "a3", RuleID: "rule-3"},
	}
	for _, a := range anomalies {
		ch <- a
	}
	close(ch)

	store := &mockAnomalyStore{}
	alert := &mockAlertChannel{}
	d := NewDispatcher(ch, store, alert, zap.NewNop())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(store.indexed) != 3 {
		t.Fatalf("expected 3 indexed anomalies, got %d", len(store.indexed))
	}
	if len(alert.sent) != 3 {
		t.Fatalf("expected 3 sent alerts, got %d", len(alert.sent))
	}
}
