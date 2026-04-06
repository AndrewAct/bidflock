package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Bidding metrics
	BidRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bidflock_bid_requests_total",
		Help: "Total number of bid requests received",
	}, []string{"ssp_id", "status"})

	BidAuctionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bidflock_bid_auction_duration_seconds",
		Help:    "Duration of auction processing",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25},
	}, []string{"auction_type"})

	BidWinPrice = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bidflock_bid_win_price_dollars",
		Help:    "Winning bid price distribution",
		Buckets: prometheus.LinearBuckets(0, 0.5, 20), // 0 to $10 CPM
	}, []string{"campaign_id"})

	// Budget metrics
	BudgetDeductions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bidflock_budget_deductions_total",
		Help: "Total budget deduction operations",
	}, []string{"campaign_id", "status"})

	BudgetRemaining = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bidflock_budget_remaining_dollars",
		Help: "Remaining daily budget per campaign",
	}, []string{"campaign_id"})

	// Scoring metrics
	ScoringDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bidflock_scoring_duration_seconds",
		Help:    "Duration of ad scoring",
		Buckets: []float64{.0001, .0005, .001, .005, .01, .05},
	}, []string{"service"})

	// Event consumer metrics
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bidflock_events_processed_total",
		Help: "Total events processed by consumer",
	}, []string{"event_type", "status"})

	// gRPC metrics
	GRPCRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bidflock_grpc_request_duration_seconds",
		Help:    "Duration of outbound gRPC calls",
		Buckets: []float64{.001, .005, .01, .025, .05},
	}, []string{"service", "method", "status"})

	// Campaign cache metrics
	CampaignCacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bidflock_campaign_cache_total",
		Help: "Campaign cache hit/miss counts",
	}, []string{"result"}) // "hit" | "miss"

	// Frequency cap metrics
	FrequencyCapChecks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bidflock_frequency_cap_checks_total",
		Help: "Frequency cap check results",
	}, []string{"result"}) // "allowed" | "capped"
)
