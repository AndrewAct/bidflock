package budget

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
)

// PacingController implements smooth budget pacing.
// Target: spend budget evenly across the day.
// Algorithm: compare actual spend rate to target rate, adjust bid multiplier.
type PacingController struct {
	redis  *redisclient.Client
	logger *slog.Logger
}

func NewPacingController(rc *redisclient.Client, logger *slog.Logger) *PacingController {
	return &PacingController{redis: rc, logger: logger}
}

// GetMultiplier returns a bid price multiplier (0.0-1.0) based on pacing.
// 1.0 = full speed, <1.0 = throttle down, 0.0 = stop bidding.
func (p *PacingController) GetMultiplier(ctx context.Context, campaignID string, remainingBudget float64) float64 {
	info := p.getPacingInfo(ctx, campaignID, remainingBudget)
	if info == nil {
		return 1.0
	}

	if info.TargetRate == 0 {
		return 1.0
	}

	// Ratio of actual spend rate to target
	ratio := info.SpendRate / info.TargetRate

	switch {
	case ratio > 1.2:
		// Spending too fast — throttle significantly
		return 0.3
	case ratio > 1.05:
		// Slightly over target — gentle throttle
		return 0.7
	case ratio < 0.8:
		// Spending too slow — allow full speed
		return 1.0
	default:
		return 1.0
	}
}

type PacingInfo struct {
	SpendRate       float64 // actual spend per hour
	TargetRate      float64 // target spend per hour
	PacingMultiplier float64
	DailySpendSoFar  float64
	DailyBudget      float64
}

func (p *PacingController) GetPacingInfo(ctx context.Context, campaignID string, remainingBudget float64) *PacingInfo {
	return p.getPacingInfo(ctx, campaignID, remainingBudget)
}

func (p *PacingController) getPacingInfo(ctx context.Context, campaignID string, remainingBudget float64) *PacingInfo {
	// Get daily budget from Redis
	budgetKey := fmt.Sprintf(keyDailyBudget, campaignID)
	dailyBudget, err := p.redis.Raw().Get(ctx, budgetKey).Float64()
	if err != nil {
		return nil
	}

	now := time.Now()
	currentHour := now.Hour()
	minutesIntoHour := float64(now.Minute())

	// Sum last 3 hours of spend to estimate current rate
	var recentSpend float64
	for i := 0; i < 3; i++ {
		h := (currentHour - i + 24) % 24
		key := fmt.Sprintf(keyHourlyBucket, campaignID, h)
		v, _ := p.redis.Raw().Get(ctx, key).Float64()
		recentSpend += v
	}
	spendRate := recentSpend / 3.0 // avg hourly spend

	// Target: spend remaining budget evenly across remaining hours of the day
	remainingHours := float64(24-currentHour) - minutesIntoHour/60.0
	if remainingHours <= 0 {
		remainingHours = 0.5
	}

	targetRate := remainingBudget / remainingHours
	dailySpend := dailyBudget - remainingBudget

	mult := p.GetMultiplier(ctx, campaignID, remainingBudget)

	return &PacingInfo{
		SpendRate:        spendRate,
		TargetRate:       targetRate,
		PacingMultiplier: mult,
		DailySpendSoFar:  dailySpend,
		DailyBudget:      dailyBudget,
	}
}
