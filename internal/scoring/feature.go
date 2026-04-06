package scoring

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AndrewAct/bidflock/gen/go/scoring"
	"github.com/AndrewAct/bidflock/pkg/models"
	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
)

const keyFeature = "feature:%s:%s" // feature_name:entity_id

// FeatureVector is the assembled feature set for CTR/CVR prediction.
type FeatureVector struct {
	// User features
	AgeGroup    float64 // 0=18-24, 1=25-34, 2=35-44, 3=45+
	GenderMale  float64 // 1.0 if male
	GeoUS       float64 // 1.0 if US
	DeviceMobile float64 // 1.0 if mobile (iOS or Android)

	// Ad/Campaign features
	AdTypeBanner float64 // 1.0 if banner
	AdTypeVideo  float64 // 1.0 if video

	// Contextual features
	HourOfDay  float64 // 0-23, normalized to 0-1
	DayOfWeek  float64 // 0-6, normalized to 0-1

	// Historical features (from Redis feature store)
	CampaignHistoricalCTR float64 // smoothed CTR from past auctions
	AdHistoricalCTR       float64
}

// FeatureAssembler pulls features from the Redis feature store and request context.
type FeatureAssembler struct {
	redis  *redisclient.Client
	logger *slog.Logger
}

func NewFeatureAssembler(rc *redisclient.Client, logger *slog.Logger) *FeatureAssembler {
	return &FeatureAssembler{redis: rc, logger: logger}
}

func (f *FeatureAssembler) Assemble(ctx context.Context, req *scoring.BidRequest, campaignID, adID string) FeatureVector {
	now := time.Now()
	fv := FeatureVector{
		HourOfDay: float64(now.Hour()) / 23.0,
		DayOfWeek: float64(now.Weekday()) / 6.0,
	}

	// User features
	if req.UserID != "" {
		fv.GeoUS = geoScore(req.Geo)
		fv.DeviceMobile = deviceScore(req.DeviceType)
		fv.AgeGroup = 1.0 // default 25-34 if unknown
	}

	// Historical CTR from feature store (best-effort, non-blocking)
	fv.CampaignHistoricalCTR = f.getHistoricalCTR(ctx, "campaign", campaignID)
	fv.AdHistoricalCTR = f.getHistoricalCTR(ctx, "ad", adID)

	return fv
}

// UpdateCTR updates the smoothed CTR for a campaign or ad in the feature store.
// Uses exponential moving average: new = alpha*observed + (1-alpha)*old
func (f *FeatureAssembler) UpdateCTR(ctx context.Context, entityType, entityID string, observedCTR float64) {
	key := fmt.Sprintf(keyFeature, "ctr:"+entityType, entityID)
	const alpha = 0.1
	old := f.getHistoricalCTR(ctx, entityType, entityID)
	updated := alpha*observedCTR + (1-alpha)*old
	f.redis.Set(ctx, key, updated, 7*24*time.Hour)
}

func (f *FeatureAssembler) getHistoricalCTR(ctx context.Context, entityType, entityID string) float64 {
	key := fmt.Sprintf(keyFeature, "ctr:"+entityType, entityID)
	var ctr float64
	if err := f.redis.Get(ctx, key, &ctr); err != nil {
		return 0.02 // default prior: 2% CTR
	}
	return ctr
}

func geoScore(geo string) float64 {
	switch geo {
	case "US":
		return 1.0
	case "GB", "CA", "AU":
		return 0.8
	default:
		return 0.4
	}
}

func deviceScore(deviceType string) float64 {
	switch deviceType {
	case "ios", "android":
		return 1.0
	default:
		return 0.5
	}
}

func adTypeFromCampaign(c *models.CampaignCache) (float64, float64) {
	for _, t := range c.Targeting.AdTypes {
		if t == models.AdTypeVideo {
			return 0.0, 1.0
		}
	}
	return 1.0, 0.0 // default banner
}
