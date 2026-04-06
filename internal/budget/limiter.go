package budget

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
)

const (
	// Default: max 5 impressions per user per campaign per 24h
	defaultFreqCapLimit  = 5
	defaultFreqCapWindow = 24 * time.Hour

	keyFreqCap = "freqcap:%s:%s" // campaign_id:user_id
)

// FrequencyLimiter uses a Redis sorted set as a sliding window counter.
// Each impression is stored with timestamp as the score, allowing
// range queries to count impressions within a time window.
type FrequencyLimiter struct {
	redis *redisclient.Client
}

func NewFrequencyLimiter(rc *redisclient.Client) *FrequencyLimiter {
	return &FrequencyLimiter{redis: rc}
}

// IsCapped returns true if the user has hit the impression cap for this campaign.
func (f *FrequencyLimiter) IsCapped(ctx context.Context, campaignID, userID string) (bool, error) {
	key := fmt.Sprintf(keyFreqCap, campaignID, userID)
	now := time.Now()
	windowStart := now.Add(-defaultFreqCapWindow)

	// Remove expired entries outside the window
	if err := f.redis.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixMilli(), 10)); err != nil {
		return false, err
	}

	count, err := f.redis.ZCount(ctx, key, "-inf", "+inf")
	if err != nil {
		return false, err
	}
	return count >= defaultFreqCapLimit, nil
}

// RecordImpression adds an impression event to the sliding window.
func (f *FrequencyLimiter) RecordImpression(ctx context.Context, campaignID, userID, impressionID string) error {
	key := fmt.Sprintf(keyFreqCap, campaignID, userID)
	score := float64(time.Now().UnixMilli())
	if err := f.redis.ZAdd(ctx, key, score, impressionID); err != nil {
		return err
	}
	// TTL slightly longer than window to allow cleanup
	return f.redis.Expire(ctx, key, defaultFreqCapWindow+time.Hour)
}
