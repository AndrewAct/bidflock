package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

type MessageHandler func(ctx context.Context, record *kgo.Record) error

type Consumer struct {
	client  *kgo.Client
	logger  *slog.Logger
	handler MessageHandler
}

func NewConsumer(brokers []string, groupID string, topics []string, logger *slog.Logger, handler MessageHandler) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return &Consumer{client: client, logger: logger, handler: handler}, nil
}

// Run polls for messages until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				c.logger.Error("kafka fetch error", "topic", e.Topic, "partition", e.Partition, "err", e.Err)
			}
			continue
		}

		fetches.EachRecord(func(r *kgo.Record) {
			if err := c.handler(ctx, r); err != nil {
				c.logger.Error("message handler failed",
					"topic", r.Topic,
					"offset", r.Offset,
					"err", err,
				)
			}
		})

		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			c.logger.Error("commit offsets failed", "err", err)
		}
	}
}

func (c *Consumer) Close() {
	c.client.Close()
}
