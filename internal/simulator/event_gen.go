package simulator

import (
	"context"
	"math/rand"
	"time"

	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/google/uuid"
)

// Funnel probabilities matching industry benchmarks.
const (
	probWinToImpression = 0.90 // 10% lost to timeout/render failure
	ctrBanner           = 0.015 // 1.5% CTR
	ctrVideo            = 0.030 // 3.0% CTR
	ctrNative           = 0.020 // 2.0% CTR
	cvrEcommerce        = 0.08
	cvrGaming           = 0.15
	cvrFintech          = 0.05
	cvrDefault          = 0.07
)

// EventGen simulates the post-bid impression → click → conversion funnel.
type EventGen struct {
	producer *kafka.Producer
	rng      *rand.Rand
}

func NewEventGen(producer *kafka.Producer, seed int64) *EventGen {
	return &EventGen{
		producer: producer,
		rng:      rand.New(rand.NewSource(seed)),
	}
}

// SimulateFunnel fires impression/click/conversion events for a winning bid.
func (g *EventGen) SimulateFunnel(ctx context.Context, winner *models.BidCandidate, bidID, userID, sspID string) {
	// Win → Impression
	if g.rng.Float64() > probWinToImpression {
		return // lost to render failure
	}

	impID := uuid.New().String()
	imp := &models.ImpressionEvent{
		EventID:    impID,
		BidID:      bidID,
		RequestID:  bidID,
		CampaignID: winner.CampaignID,
		AdID:       winner.AdID,
		UserID:     userID,
		SSPID:      sspID,
		Price:      winner.ClearingPrice,
		Timestamp:  time.Now().UnixMilli(),
	}
	g.producer.Publish(ctx, kafka.TopicImpressions, winner.CampaignID, imp)

	// Impression → Click
	ctr := ctrBanner // default
	if g.rng.Float64() > ctr {
		return
	}

	clickID := uuid.New().String()
	click := &models.ClickEvent{
		EventID:      clickID,
		ImpressionID: impID,
		CampaignID:   winner.CampaignID,
		AdID:         winner.AdID,
		UserID:       userID,
		Timestamp:    time.Now().UnixMilli(),
	}
	g.producer.Publish(ctx, kafka.TopicClicks, winner.CampaignID, click)

	// Click → Conversion
	cvr := cvrDefault
	if g.rng.Float64() > cvr {
		return
	}

	conversion := &models.ConversionEvent{
		EventID:    uuid.New().String(),
		ClickID:    clickID,
		CampaignID: winner.CampaignID,
		AdID:       winner.AdID,
		UserID:     userID,
		Value:      10.0 + g.rng.Float64()*90.0, // $10-$100 conversion value
		Timestamp:  time.Now().UnixMilli(),
	}
	g.producer.Publish(ctx, kafka.TopicConversions, winner.CampaignID, conversion)
}
