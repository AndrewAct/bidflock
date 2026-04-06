package simulator

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync/atomic"
	"time"
)

// Reporter prints live stats during simulation and saves results.
type Reporter struct {
	stats     *Stats
	targetQPS int
	startTime time.Time
	ticker    *time.Ticker
	results   []SecondStats
}

type SecondStats struct {
	Second    int     `json:"second"`
	QPS       float64 `json:"qps"`
	BidRate   float64 `json:"bid_rate"`
	ErrorRate float64 `json:"error_rate"`
	P50MS     int64   `json:"p50_ms"`
	P95MS     int64   `json:"p95_ms"`
	P99MS     int64   `json:"p99_ms"`
}

func NewReporter(stats *Stats, targetQPS int, logger interface{}) *Reporter {
	return &Reporter{
		stats:     stats,
		targetQPS: targetQPS,
		startTime: time.Now(),
		ticker:    time.NewTicker(time.Second),
	}
}

func (r *Reporter) Run(ctx context.Context) {
	prevRequests := int64(0)
	second := 0

	for {
		select {
		case <-ctx.Done():
			r.ticker.Stop()
			return
		case <-r.ticker.C:
			cur := atomic.LoadInt64(&r.stats.TotalRequests)
			qps := float64(cur - prevRequests)
			prevRequests = cur

			bids := atomic.LoadInt64(&r.stats.TotalBids)
			errors := atomic.LoadInt64(&r.stats.TotalErrors)
			total := cur

			bidRate := 0.0
			if total > 0 {
				bidRate = float64(bids) / float64(total) * 100
			}
			errorRate := 0.0
			if total > 0 {
				errorRate = float64(errors) / float64(total) * 100
			}

			p50, p95, p99 := r.percentiles()

			ss := SecondStats{
				Second:    second,
				QPS:       qps,
				BidRate:   bidRate,
				ErrorRate: errorRate,
				P50MS:     p50,
				P95MS:     p95,
				P99MS:     p99,
			}
			r.results = append(r.results, ss)
			second++

			fmt.Printf("\r[%3ds] QPS:%6.0f  Bids:%5.1f%%  Err:%4.1f%%  P50:%3dms  P95:%3dms  P99:%3dms",
				second, qps, bidRate, errorRate, p50, p95, p99)
		}
	}
}

// percentiles calculates P50/P95/P99 from the bucket histogram.
func (r *Reporter) percentiles() (p50, p95, p99 int64) {
	buckets := make([]int64, len(r.stats.LatencyBuckets))
	total := int64(0)
	for i := range buckets {
		buckets[i] = atomic.LoadInt64(&r.stats.LatencyBuckets[i])
		total += buckets[i]
	}
	if total == 0 {
		return 0, 0, 0
	}

	targets := []float64{0.50, 0.95, 0.99}
	results := make([]int64, 3)
	cumulative := int64(0)
	ti := 0

	for i, count := range buckets {
		cumulative += count
		for ti < len(targets) && float64(cumulative)/float64(total) >= targets[ti] {
			results[ti] = int64(i) * 2 // 2ms per bucket
			ti++
		}
		if ti >= len(targets) {
			break
		}
	}
	return results[0], results[1], results[2]
}

// SaveJSON writes benchmark results to a JSON file.
func (r *Reporter) SaveJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r.results)
}

// SaveCSV writes benchmark results to a CSV file for charting.
func (r *Reporter) SaveCSV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"second", "qps", "bid_rate", "error_rate", "p50_ms", "p95_ms", "p99_ms"})
	for _, s := range r.results {
		w.Write([]string{
			fmt.Sprintf("%d", s.Second),
			fmt.Sprintf("%.1f", s.QPS),
			fmt.Sprintf("%.2f", s.BidRate),
			fmt.Sprintf("%.2f", s.ErrorRate),
			fmt.Sprintf("%d", s.P50MS),
			fmt.Sprintf("%d", s.P95MS),
			fmt.Sprintf("%d", s.P99MS),
		})
	}
	return nil
}

// Summary prints a final summary table.
func (r *Reporter) Summary() {
	total := atomic.LoadInt64(&r.stats.TotalRequests)
	bids := atomic.LoadInt64(&r.stats.TotalBids)
	noBids := atomic.LoadInt64(&r.stats.TotalNoBids)
	errors := atomic.LoadInt64(&r.stats.TotalErrors)
	elapsed := time.Since(r.startTime)

	avgQPS := float64(total) / elapsed.Seconds()
	p50, p95, p99 := r.percentiles()

	// Peak QPS from results
	peakQPS := 0.0
	for _, s := range r.results {
		if s.QPS > peakQPS {
			peakQPS = s.QPS
		}
	}

	// Avg latency
	totalLatency := atomic.LoadInt64(&r.stats.TotalLatencyMS)
	avgLatency := int64(0)
	if total > 0 {
		avgLatency = totalLatency / total
	}

	// Sort results for percentile QPS
	qpsVals := make([]float64, len(r.results))
	for i, s := range r.results {
		qpsVals[i] = s.QPS
	}
	sort.Float64s(qpsVals)

	fmt.Println("\n\n─────────────────────────────────────────")
	fmt.Printf("  Bidflock Benchmark Summary\n")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  Duration:     %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Total Bids:   %d\n", total)
	fmt.Printf("  Won Bids:     %d (%.1f%%)\n", bids, pct(bids, total))
	fmt.Printf("  No Bids:      %d (%.1f%%)\n", noBids, pct(noBids, total))
	fmt.Printf("  Errors:       %d (%.1f%%)\n", errors, pct(errors, total))
	fmt.Println()
	fmt.Printf("  Avg QPS:      %.0f\n", avgQPS)
	fmt.Printf("  Peak QPS:     %.0f\n", peakQPS)
	fmt.Println()
	fmt.Printf("  Avg Latency:  %dms\n", avgLatency)
	fmt.Printf("  P50 Latency:  %dms\n", p50)
	fmt.Printf("  P95 Latency:  %dms\n", p95)
	fmt.Printf("  P99 Latency:  %dms\n", p99)
	fmt.Println("─────────────────────────────────────────")
}

func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
