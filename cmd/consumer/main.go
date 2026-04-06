package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AndrewAct/bidflock/internal/consumer"
	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/observability"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logger := observability.NewLogger("consumer", slog.LevelInfo)

	kafkaBrokers := []string{envOr("KAFKA_BROKERS", "localhost:9092")}
	clickhouseDSN := envOr("CLICKHOUSE_ADDR", "localhost:9000")

	writer, err := consumer.NewBatchWriter(clickhouseDSN, 500, 5*time.Second, logger)
	if err != nil {
		logger.Error("clickhouse connect failed", "err", err)
		os.Exit(1)
	}
	defer writer.Close()

	impHandler := consumer.NewImpressionHandler(writer)
	clickHandler := consumer.NewClickHandler(writer)
	convHandler := consumer.NewConversionHandler(writer)

	// Route each Kafka topic to its handler
	dispatcher := func(ctx context.Context, record *kgo.Record) error {
		switch record.Topic {
		case kafka.TopicImpressions:
			return impHandler.Handle(ctx, record)
		case kafka.TopicClicks:
			return clickHandler.Handle(ctx, record)
		case kafka.TopicConversions:
			return convHandler.Handle(ctx, record)
		}
		return nil
	}

	c, err := kafka.NewConsumer(
		kafkaBrokers,
		"ad-event-consumer",
		[]string{kafka.TopicImpressions, kafka.TopicClicks, kafka.TopicConversions},
		logger,
		dispatcher,
	)
	if err != nil {
		logger.Error("kafka consumer failed", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		logger.Info("ad event consumer started")
		if err := c.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("consumer run error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	logger.Info("shutting down consumer — flushing pending batches")
	cancel()
	time.Sleep(2 * time.Second) // let goroutines drain
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
