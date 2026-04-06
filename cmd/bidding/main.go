package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AndrewAct/bidflock/internal/bidding"
	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/observability"
	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logger := observability.NewLogger("bidding", slog.LevelInfo)

	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := []string{envOr("KAFKA_BROKERS", "localhost:9092")}
	scoringAddr := envOr("SCORING_ADDR", "localhost:8084")
	budgetAddr := envOr("BUDGET_ADDR", "localhost:8083")
	listenAddr := envOr("LISTEN_ADDR", ":8081")

	rc := redisclient.NewClient(redisAddr, redisclient.DBAuction)
	if err := rc.Ping(context.Background()); err != nil {
		logger.Error("redis connect failed", "err", err)
		os.Exit(1)
	}

	scoringClient, err := bidding.NewScoringClient(scoringAddr)
	if err != nil {
		logger.Error("scoring client failed", "err", err)
		os.Exit(1)
	}

	budgetClient, err := bidding.NewBudgetClient(budgetAddr)
	if err != nil {
		logger.Error("budget client failed", "err", err)
		os.Exit(1)
	}

	producer, err := kafka.NewProducer(kafkaBrokers, logger)
	if err != nil {
		logger.Error("kafka producer failed", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	cfg := bidding.DefaultConfig()
	svc := bidding.NewService(rc, scoringClient, budgetClient, producer, cfg, logger)
	handler := bidding.NewHandler(svc, logger)

	// Consume campaign-updates to maintain active campaign IDs
	consumer, err := kafka.NewConsumer(
		kafkaBrokers,
		"bidding-service",
		[]string{kafka.TopicCampaignUpdates},
		logger,
		func(ctx context.Context, record *kgo.Record) error {
			return svc.SyncCampaign(ctx, record.Value)
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

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(100 * time.Millisecond)) // enforce max latency
	r.Mount("/", handler.Routes())

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	}

	go func() {
		logger.Info("bidding service listening", "addr", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	logger.Info("shutting down bidding service")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
