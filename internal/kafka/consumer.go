package kafka

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kzap"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/config"
	"github.com/log-analytics/server/internal/domain"
)

// Consumer implements domain.MessageConsumer using franz-go with at-least-once semantics.
// Offsets are committed only after a record has been successfully forwarded to the output channel.
type Consumer struct {
	client *kgo.Client
	out    chan domain.RawMessage
	logger *zap.Logger
}

// New creates a new KafkaConsumer. The initial offset is determined by cfg.InitialOffset:
// "newest" starts from the end of the partition; any other value defaults to the beginning.
func New(cfg config.KafkaConfig, logger *zap.Logger) (*Consumer, error) {
	var initialOffset kgo.Offset
	if cfg.InitialOffset == "newest" {
		initialOffset = kgo.NewOffset().AtEnd()
	} else {
		initialOffset = kgo.NewOffset().AtStart()
	}

	out := make(chan domain.RawMessage, 10000)

	onRevoked := func(ctx context.Context, cl *kgo.Client, _ map[string][]int32) {
		if err := cl.CommitMarkedOffsets(ctx); err != nil {
			logger.Error("commit on revoke failed", zap.Error(err))
		}
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.ConsumeResetOffset(initialOffset),
		kgo.AutoCommitMarks(),
		kgo.OnPartitionsRevoked(onRevoked),
		kgo.WithLogger(kzap.New(logger)),
		kgo.FetchMaxBytes(10<<20),         // 10 MB max per fetch — reduces fetch round-trips under burst
		kgo.FetchMaxPartitionBytes(1<<20), // 1 MB per partition
		kgo.MaxConcurrentFetches(3),       // parallel fetch requests to saturate Kafka I/O
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{client: client, out: out, logger: logger}, nil
}

// Run polls Kafka for records and forwards them to the output channel.
// Offsets are marked for commit only after a record is successfully sent to the channel.
// Run blocks until ctx is cancelled or the client is closed, then returns.
func (c *Consumer) Run(ctx context.Context) error {
	defer close(c.out)
	defer c.client.Close()
	for {
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fetches.EachError(func(t string, p int32, err error) {
			c.logger.Error("fetch error",
				zap.String("topic", t),
				zap.Int32("partition", p),
				zap.Error(err),
			)
		})
		fetches.EachRecord(func(r *kgo.Record) {
			msg := domain.RawMessage{
				Payload:   r.Value,
				Topic:     r.Topic,
				Partition: r.Partition,
				Offset:    r.Offset,
				Timestamp: r.Timestamp,
			}
			select {
			case c.out <- msg:
				c.client.MarkCommitRecords(r) // CRITICAL: Mark AFTER channel send succeeds
			case <-ctx.Done():
				return
			}
		})
	}
}

// Messages returns a read-only channel of raw messages consumed from Kafka.
// The channel is closed when Run returns.
func (c *Consumer) Messages() <-chan domain.RawMessage {
	return c.out
}
