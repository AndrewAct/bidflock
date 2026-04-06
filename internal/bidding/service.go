package bidding

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	budgetpb "github.com/AndrewAct/bidflock/gen/go/budget"
	scoringpb "github.com/AndrewAct/bidflock/gen/go/scoring"
	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/models"
	redisclient "github.com/AndrewAct/bidflock/pkg/redis"
	"github.com/google/uuid"
)

type Config struct {
	AuctionType     AuctionType // FirstPrice | SecondPrice
	ScoringTimeout  time.Duration
	BudgetTimeout   time.Duration
}

func DefaultConfig() Config {
	return Config{
		AuctionType:    SecondPrice,
		ScoringTimeout: 20 * time.Millisecond,
		BudgetTimeout:  10 * time.Millisecond,
	}
}

type Service struct {
	redis    *redisclient.Client
	scoring  *ScoringClient
	budget   *BudgetClient
	producer *kafka.Producer
	config   Config
	logger   *slog.Logger
}

func NewService(
	rc *redisclient.Client,
	scoring *ScoringClient,
	budget *BudgetClient,
	producer *kafka.Producer,
	config Config,
	logger *slog.Logger,
) *Service {
	return &Service{
		redis:    rc,
		scoring:  scoring,
		budget:   budget,
		producer: producer,
		config:   config,
		logger:   logger,
	}
}

// ProcessBidRequest is the hot path. Target: <50ms P99.
func (s *Service) ProcessBidRequest(ctx context.Context, req *models.BidRequest) (*models.BidResponse, error) {
	start := time.Now()

	// Determine floor price from first impression (simplification for demo)
	floorPrice := 0.0
	if len(req.Imp) > 0 {
		floorPrice = req.Imp[0].BidFloor
	}

	sspID := ""
	if req.Ext != nil {
		sspID = req.Ext.SSPID
	}

	// Load active campaigns from Redis cache
	campaignIDs, err := s.getActiveCampaignIDs(ctx)
	if err != nil || len(campaignIDs) == 0 {
		s.publishNoBid(ctx, req, sspID, models.NBRNoAdFound, start)
		return noBidResponse(req.ID, models.NBRNoAdFound), nil
	}

	userID := ""
	if req.User != nil {
		userID = req.User.ID
	}

	// Build scoring request
	scoreReq := &scoringpb.ScoreRequest{
		RequestID:   req.ID,
		CampaignIDs: campaignIDs,
		BidRequest: scoringpb.BidRequest{
			ID:         req.ID,
			UserID:     userID,
			Geo:        userGeo(req),
			DeviceType: deviceType(req),
			SSPID:      sspID,
			Interests:  userInterests(req),
		},
	}

	// Call Scoring Service (with timeout)
	scoringCtx, cancelScoring := context.WithTimeout(ctx, s.config.ScoringTimeout)
	defer cancelScoring()

	scoreResp, err := s.scoring.ScoreAds(scoringCtx, scoreReq)
	if err != nil {
		s.logger.Warn("scoring failed, using no-bid", "request_id", req.ID, "err", err)
		s.publishNoBid(ctx, req, sspID, models.NBRTechnicalError, start)
		return noBidResponse(req.ID, models.NBRTechnicalError), nil
	}

	// Check budget for each candidate (parallel fan-out for production;
	// sequential here for simplicity — extend with errgroup for higher QPS)
	candidates := make([]models.BidCandidate, 0, len(scoreResp.Scores))
	for _, score := range scoreResp.Scores {
		budgetCtx, cancelBudget := context.WithTimeout(ctx, s.config.BudgetTimeout)
		budgetResp, err := s.budget.CheckBudget(budgetCtx, &budgetpb.CheckBudgetRequest{
			CampaignID: score.CampaignID,
			UserID:     userID,
			BidAmount:  score.EffectiveBid,
		})
		cancelBudget()

		budgetOK := err == nil && budgetResp.Allowed
		effectiveBid := score.EffectiveBid
		if budgetOK && budgetResp.PacingMultiplier > 0 {
			effectiveBid *= budgetResp.PacingMultiplier
		}

		candidates = append(candidates, models.BidCandidate{
			CampaignID:   score.CampaignID,
			AdID:         score.AdID,
			RawBid:       score.EffectiveBid,
			EffectiveBid: effectiveBid,
			PredictedCTR: score.PredictedCTR,
			BudgetOK:     budgetOK,
		})
	}

	winner, noBid := RunAuction(candidates, s.config.AuctionType, floorPrice)
	if noBid {
		s.publishNoBid(ctx, req, sspID, models.NBRBudgetConstraints, start)
		return noBidResponse(req.ID, models.NBRBudgetConstraints), nil
	}

	// Deduct the clearing price (not the bid price) from the winner's budget
	deductCtx, cancelDeduct := context.WithTimeout(ctx, 15*time.Millisecond)
	defer cancelDeduct()
	deductResp, err := s.budget.DeductBudget(deductCtx, &budgetpb.DeductBudgetRequest{
		CampaignID: winner.CampaignID,
		BidID:      req.ID,
		Amount:     winner.ClearingPrice / 1000, // CPM to per-impression cost
	})
	if err != nil || !deductResp.Success {
		s.publishNoBid(ctx, req, sspID, models.NBRBudgetConstraints, start)
		return noBidResponse(req.ID, models.NBRBudgetConstraints), nil
	}

	auctionType := "second_price"
	if s.config.AuctionType == FirstPrice {
		auctionType = "first_price"
	}

	// Publish bid result to Kafka asynchronously (don't block response)
	result := &models.BidResult{
		RequestID:         req.ID,
		AuctionType:       auctionType,
		Candidates:        candidates,
		Winner:            winner,
		AuctionDurationUS: time.Since(start).Microseconds(),
		SSPID:             sspID,
		Timestamp:         time.Now().UnixMilli(),
	}
	go func() {
		if err := s.producer.Publish(context.Background(), kafka.TopicBidResults, req.ID, result); err != nil {
			s.logger.Warn("publish bid result failed", "err", err)
		}
	}()

	bidID := uuid.New().String()
	return &models.BidResponse{
		ID:  req.ID,
		Cur: "USD",
		SeatBid: []models.SeatBid{
			{
				Seat: "bidflock",
				Bid: []models.Bid{
					{
						ID:    bidID,
						ImpID: req.Imp[0].ID,
						Price: winner.ClearingPrice,
						AdID:  winner.AdID,
						CID:   winner.CampaignID,
						CrID:  winner.AdID,
						NURL:  fmt.Sprintf("http://bidflock/win?bid=%s&price=${AUCTION_PRICE}", bidID),
					},
				},
			},
		},
	}, nil
}

// SyncCampaign updates the campaign ID list cache (called from Kafka consumer).
func (s *Service) SyncCampaign(ctx context.Context, data []byte) error {
	var event models.CampaignEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	// The bidding service only needs the set of active campaign IDs.
	// Detailed config (bids, targeting) is handled by scoring/budget services.
	return s.syncCampaignIDs(ctx, &event)
}

func (s *Service) syncCampaignIDs(ctx context.Context, event *models.CampaignEvent) error {
	const activeCampaignsKey = "bidding:active_campaigns"
	switch event.Type {
	case models.CampaignCreated, models.CampaignUpdated:
		if event.Campaign != nil && event.Campaign.Status == models.CampaignStatusActive {
			s.redis.Raw().SAdd(ctx, activeCampaignsKey, event.CampaignID)
		} else {
			s.redis.Raw().SRem(ctx, activeCampaignsKey, event.CampaignID)
		}
	case models.CampaignDeleted:
		s.redis.Raw().SRem(ctx, activeCampaignsKey, event.CampaignID)
	}
	return nil
}

func (s *Service) getActiveCampaignIDs(ctx context.Context) ([]string, error) {
	return s.redis.Raw().SMembers(ctx, "bidding:active_campaigns").Result()
}

func (s *Service) publishNoBid(ctx context.Context, req *models.BidRequest, sspID string, reason int, start time.Time) {
	result := &models.BidResult{
		RequestID:         req.ID,
		NoBid:             true,
		NoBidReason:       reason,
		AuctionDurationUS: time.Since(start).Microseconds(),
		SSPID:             sspID,
		Timestamp:         time.Now().UnixMilli(),
	}
	go func() {
		s.producer.Publish(context.Background(), kafka.TopicBidResults, req.ID, result)
	}()
}

func noBidResponse(requestID string, nbr int) *models.BidResponse {
	return &models.BidResponse{ID: requestID, NBR: nbr}
}

func userGeo(req *models.BidRequest) string {
	if req.Device != nil && req.Device.Geo != nil {
		return req.Device.Geo.Country
	}
	if req.User != nil && req.User.Geo != nil {
		return req.User.Geo.Country
	}
	return ""
}

func deviceType(req *models.BidRequest) string {
	if req.Device == nil {
		return ""
	}
	switch req.Device.OS {
	case "iOS":
		return "ios"
	case "Android":
		return "android"
	default:
		return "desktop"
	}
}

func userInterests(req *models.BidRequest) []string {
	if req.User != nil {
		return req.User.Interests
	}
	return nil
}
