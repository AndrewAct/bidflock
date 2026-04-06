package budget

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/models"
	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
	"github.com/redis/go-redis/v9"
)

// Redis key formats
const (
	keyDailyBudget  = "budget:daily:%s"   // campaign_id
	keyTotalSpend   = "budget:total:%s"   // campaign_id
	keyHourlyBucket = "budget:hourly:%s:%d" // campaign_id, hour
)

// Lua script for atomic check-and-deduct.
// Returns 1 on success, 0 if insufficient budget.
const luaDeduct = `
local key = KEYS[1]
local amount = tonumber(ARGV[1])
local current = tonumber(redis.call('GET', key) or '0')
if current < amount then
  return 0
end
redis.call('INCRBYFLOAT', key, -amount)
return 1
`

type Service struct {
	redis  *redisclient.Client
	pacing *PacingController
	limiter *FrequencyLimiter
	logger *slog.Logger
}

func NewService(rc *redisclient.Client, logger *slog.Logger) *Service {
	return &Service{
		redis:   rc,
		pacing:  NewPacingController(rc, logger),
		limiter: NewFrequencyLimiter(rc),
		logger:  logger,
	}
}

// SyncCampaign updates Redis budget config from a Kafka campaign event.
func (s *Service) SyncCampaign(ctx context.Context, event *models.CampaignEvent) error {
	if event.Campaign == nil {
		return nil
	}
	c := event.Campaign

	// Only initialize budget key if it doesn't exist (don't reset mid-day spend)
	budgetKey := fmt.Sprintf(keyDailyBudget, c.ID)
	exists, err := s.redis.Exists(ctx, budgetKey)
	if err != nil {
		return err
	}
	if !exists {
		// Store as float in cents-precision using SET with NX
		if err := s.redis.Raw().SetNX(ctx, budgetKey, c.DailyBudget, 26*time.Hour).Err(); err != nil {
			return err
		}
	}
	s.logger.Info("synced campaign budget", "campaign_id", c.ID, "daily_budget", c.DailyBudget)
	return nil
}

// CheckBudget verifies a campaign has budget available and isn't frequency-capped.
func (s *Service) CheckBudget(ctx context.Context, campaignID, userID string, bidAmount float64) (bool, string, float64, float64) {
	budgetKey := fmt.Sprintf(keyDailyBudget, campaignID)

	remaining, err := s.redis.Raw().Get(ctx, budgetKey).Float64()
	if err == redis.Nil {
		return false, "no budget configured", 0, 0
	}
	if err != nil {
		s.logger.Error("budget check redis error", "err", err)
		return false, "internal error", 0, 0
	}

	if remaining < bidAmount {
		return false, "insufficient daily budget", remaining, 0
	}

	// Frequency cap check
	if userID != "" {
		capped, err := s.limiter.IsCapped(ctx, campaignID, userID)
		if err != nil {
			s.logger.Warn("frequency cap check failed", "err", err)
		} else if capped {
			return false, "frequency cap reached", remaining, 0
		}
	}

	pacingMult := s.pacing.GetMultiplier(ctx, campaignID, remaining)
	return true, "", remaining, pacingMult
}

// DeductBudget atomically deducts spend. Returns success + remaining budget.
func (s *Service) DeductBudget(ctx context.Context, campaignID, bidID string, amount float64) (bool, float64) {
	budgetKey := fmt.Sprintf(keyDailyBudget, campaignID)

	result, err := s.redis.RunScript(ctx, luaDeduct, []string{budgetKey}, amount)
	if err != nil {
		s.logger.Error("budget deduct script error", "campaign_id", campaignID, "err", err)
		return false, 0
	}

	success := result.(int64) == 1
	if !success {
		return false, 0
	}

	remaining, _ := s.redis.Raw().Get(ctx, budgetKey).Float64()

	// Track hourly spend for pacing
	hour := time.Now().Hour()
	hourKey := fmt.Sprintf(keyHourlyBucket, campaignID, hour)
	s.redis.Raw().IncrByFloat(ctx, hourKey, amount)
	s.redis.Raw().Expire(ctx, hourKey, 2*time.Hour)

	return true, remaining
}

// ConsumeKafkaEvents processes campaign-updates topic to sync budget config.
func (s *Service) ConsumeKafkaEvents(ctx context.Context, record *kafka.MessageHandler) error {
	return nil // wired in main.go via kafka.Consumer
}

func (s *Service) HandleKafkaRecord(ctx context.Context, data []byte) error {
	var event models.CampaignEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal campaign event: %w", err)
	}
	if event.Type == models.CampaignDeleted {
		// Clean up budget key on campaign delete
		budgetKey := fmt.Sprintf(keyDailyBudget, event.CampaignID)
		return s.redis.Del(ctx, budgetKey)
	}
	return s.SyncCampaign(ctx, &event)
}
