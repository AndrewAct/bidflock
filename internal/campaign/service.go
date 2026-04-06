package campaign

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrBadRequest = errors.New("bad request")
)

type Service struct {
	repo      *Repository
	publisher *Publisher
	logger    *slog.Logger
}

func NewService(repo *Repository, publisher *Publisher, logger *slog.Logger) *Service {
	return &Service{repo: repo, publisher: publisher, logger: logger}
}

// --- Advertiser ---

func (s *Service) CreateAdvertiser(ctx context.Context, req *models.Advertiser) (*models.Advertiser, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name required", ErrBadRequest)
	}
	req.ID = uuid.New().String()
	if err := s.repo.CreateAdvertiser(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Service) GetAdvertiser(ctx context.Context, id string) (*models.Advertiser, error) {
	return s.repo.GetAdvertiser(ctx, id)
}

func (s *Service) ListAdvertisers(ctx context.Context, limit, offset int64) ([]models.Advertiser, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListAdvertisers(ctx, limit, offset)
}

// --- Campaign ---

func (s *Service) CreateCampaign(ctx context.Context, req *models.Campaign) (*models.Campaign, error) {
	if err := validateCampaign(req); err != nil {
		return nil, err
	}
	req.ID = uuid.New().String()
	if req.Status == "" {
		req.Status = models.CampaignStatusDraft
	}
	if err := s.repo.CreateCampaign(ctx, req); err != nil {
		return nil, err
	}
	s.publishEvent(ctx, models.CampaignCreated, req.ID, req)
	return req, nil
}

func (s *Service) GetCampaign(ctx context.Context, id string) (*models.Campaign, error) {
	return s.repo.GetCampaign(ctx, id)
}

func (s *Service) UpdateCampaign(ctx context.Context, id string, req *models.Campaign) (*models.Campaign, error) {
	if err := validateCampaign(req); err != nil {
		return nil, err
	}
	req.ID = id
	if err := s.repo.UpdateCampaign(ctx, id, req); err != nil {
		return nil, err
	}
	s.publishEvent(ctx, models.CampaignUpdated, id, req)
	return req, nil
}

func (s *Service) DeleteCampaign(ctx context.Context, id string) error {
	if err := s.repo.DeleteCampaign(ctx, id); err != nil {
		return err
	}
	s.publishEvent(ctx, models.CampaignDeleted, id, nil)
	return nil
}

func (s *Service) ListCampaigns(ctx context.Context, advertiserID string, status models.CampaignStatus, limit, offset int64) ([]models.Campaign, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListCampaigns(ctx, advertiserID, status, limit, offset)
}

// --- Ad ---

func (s *Service) CreateAd(ctx context.Context, req *models.Ad) (*models.Ad, error) {
	if req.LandingURL == "" {
		return nil, fmt.Errorf("%w: landing_url required", ErrBadRequest)
	}
	req.ID = uuid.New().String()

	// Link ad to campaign
	campaign, err := s.repo.GetCampaign(ctx, req.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign %s: %w", req.CampaignID, err)
	}
	if err := s.repo.CreateAd(ctx, req); err != nil {
		return nil, err
	}

	// Update campaign's ad list and publish event
	campaign.AdIDs = append(campaign.AdIDs, req.ID)
	_ = s.repo.UpdateCampaign(ctx, campaign.ID, campaign)
	s.publishEvent(ctx, models.CampaignUpdated, campaign.ID, campaign)
	return req, nil
}

func (s *Service) GetAd(ctx context.Context, id string) (*models.Ad, error) {
	return s.repo.GetAd(ctx, id)
}

func (s *Service) UpdateAd(ctx context.Context, id string, req *models.Ad) (*models.Ad, error) {
	req.ID = id
	if err := s.repo.UpdateAd(ctx, id, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Service) DeleteAd(ctx context.Context, id string) error {
	return s.repo.DeleteAd(ctx, id)
}

func (s *Service) ListAdsByCampaign(ctx context.Context, campaignID string) ([]models.Ad, error) {
	return s.repo.ListAdsByCampaign(ctx, campaignID)
}

// --- helpers ---

func (s *Service) publishEvent(ctx context.Context, eventType models.CampaignEventType, campaignID string, campaign *models.Campaign) {
	if s.publisher == nil || s.publisher.producer == nil {
		return
	}
	event := &models.CampaignEvent{
		Type:       eventType,
		CampaignID: campaignID,
		Campaign:   campaign,
		Timestamp:  time.Now().UnixMilli(),
	}
	if err := s.publisher.PublishCampaignEvent(ctx, event); err != nil {
		s.logger.Error("failed to publish campaign event",
			"event_type", eventType,
			"campaign_id", campaignID,
			"err", err,
		)
	}
}

func validateCampaign(c *models.Campaign) error {
	if c.Name == "" {
		return fmt.Errorf("%w: name required", ErrBadRequest)
	}
	if c.AdvertiserID == "" {
		return fmt.Errorf("%w: advertiser_id required", ErrBadRequest)
	}
	if c.DailyBudget <= 0 {
		return fmt.Errorf("%w: daily_budget must be positive", ErrBadRequest)
	}
	if c.BaseBid <= 0 {
		return fmt.Errorf("%w: base_bid must be positive", ErrBadRequest)
	}
	if c.EndDate.IsZero() || c.EndDate.Before(time.Now()) {
		return fmt.Errorf("%w: end_date must be in the future", ErrBadRequest)
	}
	return nil
}
