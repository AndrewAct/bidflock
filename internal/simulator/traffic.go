package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
)

type Pattern string

const (
	PatternSteady Pattern = "steady"
	PatternSpike  Pattern = "spike"  // ramps up to 3x, then back down
	PatternRamp   Pattern = "ramp"   // linearly ramps from 10% to 100% of QPS
	PatternDiurnal Pattern = "diurnal" // simulates day/night traffic curve
)

type TrafficConfig struct {
	TargetQPS int
	Duration  time.Duration
	Pattern   Pattern
	BidURL    string
}

type Stats struct {
	TotalRequests  int64
	TotalBids      int64
	TotalNoBids    int64
	TotalErrors    int64
	TotalLatencyMS int64 // sum for average
	LatencyBuckets [50]int64 // 2ms buckets: index i = 0-99ms range
}

// TrafficController drives bid requests at configurable QPS with traffic shaping.
type TrafficController struct {
	cfg      TrafficConfig
	reqGen   *RequestGen
	reporter *Reporter
	stats    Stats
	logger   *slog.Logger
}

func NewTrafficController(cfg TrafficConfig, seed int64, producer interface{}, log *slog.Logger) *TrafficController {
	logger = log
	return &TrafficController{
		cfg:    cfg,
		reqGen: NewRequestGen(seed),
		logger: log,
	}
}

// GetReporter returns the reporter after Run completes.
func (tc *TrafficController) GetReporter() *Reporter {
	return tc.reporter
}

func (tc *TrafficController) Run(ctx context.Context) error {
	tc.reporter = NewReporter(&tc.stats, tc.cfg.TargetQPS, logger)

	go tc.reporter.Run(ctx)

	deadline := time.Now().Add(tc.cfg.Duration)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	second := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil
			}
			targetQPS := tc.qpsForSecond(second)
			second++
			go tc.sendBurst(ctx, targetQPS)
		}
	}
}

func (tc *TrafficController) sendBurst(ctx context.Context, qps int) {
	if qps <= 0 {
		return
	}
	interval := time.Second / time.Duration(qps)
	var wg sync.WaitGroup
	for i := 0; i < qps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tc.sendRequest(ctx)
		}()
		time.Sleep(interval)
	}
	wg.Wait()
}

func (tc *TrafficController) sendRequest(ctx context.Context) {
	atomic.AddInt64(&tc.stats.TotalRequests, 1)
	req := tc.reqGen.Generate()

	start := time.Now()
	resp, err := sendBidRequest(tc.cfg.BidURL, req)
	latencyMS := time.Since(start).Milliseconds()
	atomic.AddInt64(&tc.stats.TotalLatencyMS, latencyMS)

	// Bucket into 2ms intervals for percentile tracking
	bucket := int(latencyMS / 2)
	if bucket >= len(tc.stats.LatencyBuckets) {
		bucket = len(tc.stats.LatencyBuckets) - 1
	}
	atomic.AddInt64(&tc.stats.LatencyBuckets[bucket], 1)

	if err != nil {
		atomic.AddInt64(&tc.stats.TotalErrors, 1)
		return
	}

	if len(resp.SeatBid) > 0 {
		atomic.AddInt64(&tc.stats.TotalBids, 1)
	} else {
		atomic.AddInt64(&tc.stats.TotalNoBids, 1)
	}
}

func (tc *TrafficController) qpsForSecond(second int) int {
	totalSeconds := int(tc.cfg.Duration.Seconds())
	baseQPS := tc.cfg.TargetQPS
	progress := float64(second) / float64(totalSeconds)

	switch tc.cfg.Pattern {
	case PatternSteady:
		return baseQPS

	case PatternRamp:
		return int(float64(baseQPS) * (0.1 + 0.9*progress))

	case PatternSpike:
		// Normal for 30%, spike to 3x for 20%, back to normal for 50%
		if progress < 0.30 {
			return baseQPS
		} else if progress < 0.50 {
			return baseQPS * 3
		}
		return baseQPS

	case PatternDiurnal:
		// Simulate hourly traffic: peak at 70% of duration, trough at start/end
		angle := progress * 2 * math.Pi
		multiplier := 0.3 + 0.7*math.Abs(math.Sin(angle))
		return int(float64(baseQPS) * multiplier)
	}
	return baseQPS
}

func sendBidRequest(url string, req *models.BidRequest) (*models.BidResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpResp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer httpResp.Body.Close()

	var resp models.BidResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}

// package-level logger for use in TrafficController
var logger *slog.Logger
