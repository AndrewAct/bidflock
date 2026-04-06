package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/AndrewAct/bidflock/gen/go/scoring"
	"github.com/AndrewAct/bidflock/pkg/models"
	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
)

const keyCampaignCache = "campaign:%s"

type Service struct {
	redis     *redisclient.Client
	assembler *FeatureAssembler
	ctr       *LogisticRegressor
	cvr       *LogisticRegressor
	logger    *slog.Logger
}

func NewService(rc *redisclient.Client, logger *slog.Logger) *Service {
	return &Service{
		redis:     rc,
		assembler: NewFeatureAssembler(rc, logger),
		ctr:       NewCTRPredictor(),
		cvr:       NewCVRPredictor(),
		logger:    logger,
	}
}

// ScoreAds returns CTR/CVR predictions and effective bids for candidate campaigns.
func (s *Service) ScoreAds(ctx context.Context, req *scoring.ScoreRequest) (*scoring.ScoreResponse, error) {
	resp := &scoring.ScoreResponse{RequestID: req.RequestID}

	for _, campaignID := range req.CampaignIDs {
		campaign, err := s.getCampaignCache(ctx, campaignID)
		if err != nil {
			s.logger.Warn("campaign cache miss", "campaign_id", campaignID, "err", err)
			continue
		}

		for _, adID := range campaign.AdIDs {
			fv := s.assembler.Assemble(ctx, &req.BidRequest, campaignID, adID)

			// Adjust feature vector based on campaign targeting
			fv.AdTypeBanner, fv.AdTypeVideo = adTypeFromCampaign(campaign)

			predictedCTR := s.ctr.Predict(fv)
			predictedCVR := s.cvr.Predict(fv)

			// eCPM = base_bid * predicted_ctr
			// Cap at bid ceiling
			effectiveBid := campaign.BaseBid * predictedCTR * 1000 // CPM conversion
			if campaign.BidCeiling > 0 && effectiveBid > campaign.BidCeiling {
				effectiveBid = campaign.BidCeiling
			}

			qualityScore := predictedCTR * effectiveBid // simple quality score

			resp.Scores = append(resp.Scores, scoring.AdScore{
				CampaignID:   campaignID,
				AdID:         adID,
				PredictedCTR: predictedCTR,
				PredictedCVR: predictedCVR,
				EffectiveBid: effectiveBid,
				QualityScore: qualityScore,
			})
		}
	}

	return resp, nil
}

// SyncCampaign caches campaign config in Redis (consumed from Kafka).
func (s *Service) SyncCampaign(ctx context.Context, event *models.CampaignEvent) error {
	if event.Type == models.CampaignDeleted {
		key := fmt.Sprintf(keyCampaignCache, event.CampaignID)
		return s.redis.Del(ctx, key)
	}
	if event.Campaign == nil {
		return nil
	}

	cache := models.CampaignCache{
		ID:          event.Campaign.ID,
		Status:      event.Campaign.Status,
		DailyBudget: event.Campaign.DailyBudget,
		TotalBudget: event.Campaign.TotalBudget,
		BidCeiling:  event.Campaign.BidCeiling,
		BaseBid:     event.Campaign.BaseBid,
		BidStrategy: event.Campaign.BidStrategy,
		Targeting:   event.Campaign.Targeting,
		AdIDs:       event.Campaign.AdIDs,
		StartDate:   event.Campaign.StartDate.Unix(),
		EndDate:     event.Campaign.EndDate.Unix(),
	}

	key := fmt.Sprintf(keyCampaignCache, event.Campaign.ID)
	return s.redis.Set(ctx, key, cache, 0) // no expiry — evicted on campaign.deleted
}

func (s *Service) HandleKafkaRecord(ctx context.Context, data []byte) error {
	var event models.CampaignEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return s.SyncCampaign(ctx, &event)
}

func (s *Service) getCampaignCache(ctx context.Context, campaignID string) (*models.CampaignCache, error) {
	key := fmt.Sprintf(keyCampaignCache, campaignID)
	var c models.CampaignCache
	if err := s.redis.Get(ctx, key, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
