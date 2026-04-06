package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AndrewAct/bidflock/internal/simulator"
	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/observability"
)

func main() {
	qps := flag.Int("qps", 100, "target requests per second")
	duration := flag.Duration("duration", 60*time.Second, "simulation duration (e.g. 30s, 5m)")
	pattern := flag.String("pattern", "steady", "traffic pattern: steady|spike|ramp|diurnal")
	bidURL := flag.String("bid-url", "http://localhost:8081/bid", "bidding service endpoint")
	kafkaBrokers := flag.String("kafka-brokers", "localhost:9092", "comma-separated Kafka brokers")
	outputDir := flag.String("output", ".", "directory to save benchmark results (JSON+CSV)")
	seed := flag.Int64("seed", time.Now().UnixNano(), "random seed for reproducibility")
	flag.Parse()

	logger := observability.NewLogger("simulator", slog.LevelInfo)

	// Kafka producer for event simulation
	producer, err := kafka.NewProducer([]string{*kafkaBrokers}, logger)
	if err != nil {
		logger.Warn("kafka producer unavailable — event simulation disabled", "err", err)
		producer = nil
	}
	if producer != nil {
		defer producer.Close()
	}

	pat := simulator.Pattern(*pattern)
	switch pat {
	case simulator.PatternSteady, simulator.PatternSpike, simulator.PatternRamp, simulator.PatternDiurnal:
	default:
		fmt.Fprintf(os.Stderr, "unknown pattern %q, using steady\n", *pattern)
		pat = simulator.PatternSteady
	}

	cfg := simulator.TrafficConfig{
		TargetQPS: *qps,
		Duration:  *duration,
		Pattern:   pat,
		BidURL:    *bidURL,
	}

	fmt.Printf("bidflock simulator\n")
	fmt.Printf("  target QPS: %d | duration: %s | pattern: %s\n", *qps, *duration, pat)
	fmt.Printf("  bid URL: %s\n\n", *bidURL)

	tc := simulator.NewTrafficController(cfg, *seed, producer, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		fmt.Println("\ninterrupted — stopping simulation")
		cancel()
	}()

	if err := tc.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("simulation failed", "err", err)
		os.Exit(1)
	}

	reporter := tc.GetReporter()
	reporter.Summary()

	// Save results
	ts := time.Now().Format("20060102-150405")
	jsonPath := filepath.Join(*outputDir, fmt.Sprintf("benchmark-%s.json", ts))
	csvPath := filepath.Join(*outputDir, fmt.Sprintf("benchmark-%s.csv", ts))

	if err := reporter.SaveJSON(jsonPath); err != nil {
		logger.Warn("save JSON failed", "err", err)
	} else {
		fmt.Printf("\nResults saved to %s\n", jsonPath)
	}
	if err := reporter.SaveCSV(csvPath); err != nil {
		logger.Warn("save CSV failed", "err", err)
	} else {
		fmt.Printf("Results saved to %s\n", csvPath)
	}
}
