package kafka

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/log-analytics/server/internal/domain"
)

// TestConsumer_ImplementsMessageConsumer is a compile-time check that Consumer
// satisfies the domain.MessageConsumer interface.
func TestConsumer_ImplementsMessageConsumer(t *testing.T) {
	var _ domain.MessageConsumer = (*Consumer)(nil)
}

// TestConsumer_MarkAfterSend_CodeOrdering verifies the critical ordering
// invariant (INGEST-02): c.out <- msg must appear BEFORE MarkCommitRecords
// in the source code to ensure at-least-once semantics.
func TestConsumer_MarkAfterSend_CodeOrdering(t *testing.T) {
	src, err := os.ReadFile("consumer.go")
	require.NoError(t, err)

	source := string(src)

	sendIdx := strings.Index(source, "c.out <- msg")
	markIdx := strings.Index(source, "MarkCommitRecords")

	require.NotEqual(t, -1, sendIdx, "c.out <- msg not found in consumer.go")
	require.NotEqual(t, -1, markIdx, "MarkCommitRecords not found in consumer.go")
	assert.Less(t, sendIdx, markIdx,
		"CRITICAL: c.out <- msg must appear BEFORE MarkCommitRecords for at-least-once semantics (INGEST-02)")
}

// TestConsumer_Messages_ReturnsChannel verifies that Messages() returns the
// output channel and that messages sent to it can be received via Messages().
func TestConsumer_Messages_ReturnsChannel(t *testing.T) {
	out := make(chan domain.RawMessage, 10)
	c := &Consumer{out: out}
	ch := c.Messages()
	assert.NotNil(t, ch, "Messages() should return a non-nil channel")

	// Verify it's the same underlying channel.
	testMsg := domain.RawMessage{Topic: "test"}
	out <- testMsg
	received := <-ch
	assert.Equal(t, "test", received.Topic)
}

// TestConsumer_RunClosesChannel verifies the graceful shutdown contract (INGEST-03):
// Run must defer close(c.out) so consumers of Messages() see channel closure
// when the context is cancelled.
func TestConsumer_RunClosesChannel(t *testing.T) {
	src, err := os.ReadFile("consumer.go")
	require.NoError(t, err)
	assert.Contains(t, string(src), "defer close(c.out)",
		"Run must defer close(c.out) for graceful shutdown (INGEST-03)")
}
