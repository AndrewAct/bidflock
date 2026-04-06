package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AndrewAct/bidflock/internal/scoring"
	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/observability"
	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logger := observability.NewLogger("scoring", slog.LevelInfo)

	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := []string{envOr("KAFKA_BROKERS", "localhost:9092")}
	grpcAddr := envOr("GRPC_ADDR", ":8084")

	rc := redisclient.NewClient(redisAddr, redisclient.DBFeatureStore)
	if err := rc.Ping(context.Background()); err != nil {
		logger.Error("redis connect failed", "err", err)
		os.Exit(1)
	}

	svc := scoring.NewService(rc, logger)
	grpcServer := scoring.NewGRPCServer(svc, logger)

	consumer, err := kafka.NewConsumer(
		kafkaBrokers,
		"scoring-service",
		[]string{kafka.TopicCampaignUpdates},
		logger,
		func(ctx context.Context, record *kgo.Record) error {
			return svc.HandleKafkaRecord(ctx, record.Value)
		},
	)
	if err != nil {
		logger.Error("kafka consumer failed", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("kafka consumer error", "err", err)
		}
	}()

	go func() {
		if err := grpcServer.Serve(grpcAddr); err != nil {
			logger.Error("grpc serve failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	logger.Info("shutting down scoring service")
	cancel()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
