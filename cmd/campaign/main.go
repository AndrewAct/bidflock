package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AndrewAct/bidflock/internal/campaign"
	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	logger := observability.NewLogger("campaign", slog.LevelInfo)

	mongoURI := envOr("MONGO_URI", "mongodb://localhost:27017")
	kafkaBrokers := []string{envOr("KAFKA_BROKERS", "localhost:9092")}
	listenAddr := envOr("LISTEN_ADDR", ":8082")

	// MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.Error("mongodb connect failed", "err", err)
		os.Exit(1)
	}
	if err := mongoClient.Ping(ctx, nil); err != nil {
		logger.Error("mongodb ping failed", "err", err)
		os.Exit(1)
	}
	db := mongoClient.Database("bidflock")

	// Kafka producer
	producer, err := kafka.NewProducer(kafkaBrokers, logger)
	if err != nil {
		logger.Error("kafka producer failed", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	repo := campaign.NewRepository(db)
	if err := repo.CreateIndexes(context.Background()); err != nil {
		logger.Warn("create indexes failed", "err", err)
	}

	publisher := campaign.NewPublisher(producer)
	svc := campaign.NewService(repo, publisher, logger)
	handler := campaign.NewHandler(svc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Mount("/", handler.Routes())
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("campaign service listening", "addr", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	logger.Info("shutting down campaign service")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	mongoClient.Disconnect(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
