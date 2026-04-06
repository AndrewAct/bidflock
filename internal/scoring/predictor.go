package scoring

import (
	"math"
)

// LogisticRegressor performs CTR/CVR prediction using logistic regression.
// Weights are hand-tuned priors; replace with trained weights from a model pipeline.
type LogisticRegressor struct {
	weights []float64
	bias    float64
}

// NewCTRPredictor returns a predictor with reasonable priors for CTR.
// Feature order: [AgeGroup, GenderMale, GeoUS, DeviceMobile, AdTypeBanner, AdTypeVideo,
//                 HourOfDay, DayOfWeek, CampaignHistoricalCTR*10, AdHistoricalCTR*10]
func NewCTRPredictor() *LogisticRegressor {
	return &LogisticRegressor{
		// Positive weights = feature increases CTR probability
		weights: []float64{
			0.05,  // AgeGroup: 25-34 slightly better
			0.10,  // GenderMale: slight positive for demo
			0.30,  // GeoUS: higher engagement in US
			0.40,  // DeviceMobile: mobile converts better
			0.20,  // AdTypeBanner: lower than video but more volume
			0.50,  // AdTypeVideo: highest engagement
			-0.10, // HourOfDay: varies, mild negative default (peaks handled by historical)
			0.05,  // DayOfWeek: weekdays slightly better
			2.00,  // CampaignHistoricalCTR * 10 (scaled)
			2.50,  // AdHistoricalCTR * 10 (most predictive)
		},
		bias: -3.0, // base: sigmoid(-3) ≈ 0.047 → ~5% base CTR
	}
}

// NewCVRPredictor returns a predictor for conversion rate.
func NewCVRPredictor() *LogisticRegressor {
	return &LogisticRegressor{
		weights: []float64{
			0.10,  // AgeGroup
			0.05,  // GenderMale
			0.25,  // GeoUS
			0.30,  // DeviceMobile
			0.15,  // AdTypeBanner
			0.35,  // AdTypeVideo
			-0.05, // HourOfDay
			0.10,  // DayOfWeek
			1.50,  // CampaignHistoricalCTR * 10
			1.80,  // AdHistoricalCTR * 10
		},
		bias: -4.0, // base: sigmoid(-4) ≈ 0.018 → ~2% base CVR
	}
}

// Predict runs logistic regression on a feature vector.
// Returns probability in [0, 1].
func (lr *LogisticRegressor) Predict(fv FeatureVector) float64 {
	features := fvToSlice(fv)
	z := lr.bias
	for i, w := range lr.weights {
		if i < len(features) {
			z += w * features[i]
		}
	}
	return sigmoid(z)
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func fvToSlice(fv FeatureVector) []float64 {
	return []float64{
		fv.AgeGroup,
		fv.GenderMale,
		fv.GeoUS,
		fv.DeviceMobile,
		fv.AdTypeBanner,
		fv.AdTypeVideo,
		fv.HourOfDay,
		fv.DayOfWeek,
		fv.CampaignHistoricalCTR * 10, // scaled for weight magnitude
		fv.AdHistoricalCTR * 10,
	}
}
