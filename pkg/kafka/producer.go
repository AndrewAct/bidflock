package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	TopicBidRequests    = "bid-requests"
	TopicBidResults     = "bid-results"
	TopicImpressions    = "impressions"
	TopicClicks         = "clicks"
	TopicConversions    = "conversions"
	TopicCampaignUpdates = "campaign-updates"
)

type Producer struct {
	client *kgo.Client
	logger *slog.Logger
}

func NewProducer(brokers []string, logger *slog.Logger) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ProducerBatchMaxBytes(1<<20), // 1MB
		kgo.RecordRetries(3),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	return &Producer{client: client, logger: logger}, nil
}

func (p *Producer) Publish(ctx context.Context, topic string, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	rec := &kgo.Record{
		Topic: topic,
		Value: data,
	}
	if key != "" {
		rec.Key = []byte(key)
	}

	if err := p.client.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return fmt.Errorf("produce to %s: %w", topic, err)
	}
	return nil
}

func (p *Producer) Close() {
	p.client.Close()
}
