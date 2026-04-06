package scoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCTRPredictor_OutputRange(t *testing.T) {
	predictor := NewCTRPredictor()

	// Test various feature combinations — output must always be in (0, 1)
	cases := []FeatureVector{
		// High engagement: US, mobile, video, high historical CTR
		{AgeGroup: 1, GenderMale: 1, GeoUS: 1, DeviceMobile: 1, AdTypeVideo: 1, HourOfDay: 0.5, DayOfWeek: 0.3, CampaignHistoricalCTR: 0.05, AdHistoricalCTR: 0.06},
		// Low engagement: non-US, desktop, banner, no history
		{AgeGroup: 0, GenderMale: 0, GeoUS: 0, DeviceMobile: 0, AdTypeBanner: 1, HourOfDay: 0.1, DayOfWeek: 0.1, CampaignHistoricalCTR: 0.01, AdHistoricalCTR: 0.01},
		// Zero vector
		{},
		// All ones
		{AgeGroup: 1, GenderMale: 1, GeoUS: 1, DeviceMobile: 1, AdTypeBanner: 1, AdTypeVideo: 1, HourOfDay: 1, DayOfWeek: 1, CampaignHistoricalCTR: 0.1, AdHistoricalCTR: 0.1},
	}

	for _, fv := range cases {
		ctr := predictor.Predict(fv)
		assert.Greater(t, ctr, 0.0, "CTR must be > 0")
		assert.Less(t, ctr, 1.0, "CTR must be < 1")
	}
}

func TestCTRPredictor_HighEngagementBeatsLow(t *testing.T) {
	predictor := NewCTRPredictor()

	highEngagement := FeatureVector{
		GeoUS: 1, DeviceMobile: 1, AdTypeVideo: 1,
		CampaignHistoricalCTR: 0.08, AdHistoricalCTR: 0.09,
	}
	lowEngagement := FeatureVector{
		GeoUS: 0, DeviceMobile: 0, AdTypeBanner: 1,
		CampaignHistoricalCTR: 0.01, AdHistoricalCTR: 0.01,
	}

	highCTR := predictor.Predict(highEngagement)
	lowCTR := predictor.Predict(lowEngagement)

	assert.Greater(t, highCTR, lowCTR, "high engagement features should yield higher CTR")
}

func TestCTRPredictor_HistoricalCTRDominates(t *testing.T) {
	predictor := NewCTRPredictor()

	// Same base features, different historical CTR
	base := FeatureVector{GeoUS: 0.5, DeviceMobile: 0.5, AdTypeBanner: 1, HourOfDay: 0.5, DayOfWeek: 0.5}

	highHistory := base
	highHistory.CampaignHistoricalCTR = 0.10
	highHistory.AdHistoricalCTR = 0.12

	lowHistory := base
	lowHistory.CampaignHistoricalCTR = 0.01
	lowHistory.AdHistoricalCTR = 0.01

	assert.Greater(t, predictor.Predict(highHistory), predictor.Predict(lowHistory))
}

func TestCVRPredictor_LowerThanCTR(t *testing.T) {
	ctr := NewCTRPredictor()
	cvr := NewCVRPredictor()

	fv := FeatureVector{GeoUS: 1, DeviceMobile: 1, AdTypeBanner: 1, HourOfDay: 0.5, DayOfWeek: 0.5, CampaignHistoricalCTR: 0.03, AdHistoricalCTR: 0.03}

	// CVR should be lower than CTR (conversions are rarer than clicks)
	assert.Less(t, cvr.Predict(fv), ctr.Predict(fv), "CVR should be lower than CTR")
}

func TestSigmoid(t *testing.T) {
	assert.InDelta(t, 0.5, sigmoid(0), 0.001, "sigmoid(0) = 0.5")
	assert.Greater(t, sigmoid(10.0), 0.99, "sigmoid(10) ≈ 1")
	assert.Less(t, sigmoid(-10.0), 0.01, "sigmoid(-10) ≈ 0")
}
