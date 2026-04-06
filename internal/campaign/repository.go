package campaign

import (
	"context"
	"fmt"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	advertisers *mongo.Collection
	campaigns   *mongo.Collection
	ads         *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		advertisers: db.Collection("advertisers"),
		campaigns:   db.Collection("campaigns"),
		ads:         db.Collection("ads"),
	}
}

func (r *Repository) CreateIndexes(ctx context.Context) error {
	_, err := r.campaigns.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "advertiser_id", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "start_date", Value: 1}, {Key: "end_date", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("campaign indexes: %w", err)
	}
	_, err = r.ads.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "campaign_id", Value: 1}}},
		{Keys: bson.D{{Key: "advertiser_id", Value: 1}}},
	})
	return err
}

// --- Advertiser CRUD ---

func (r *Repository) CreateAdvertiser(ctx context.Context, adv *models.Advertiser) error {
	adv.CreatedAt = time.Now()
	_, err := r.advertisers.InsertOne(ctx, adv)
	return err
}

func (r *Repository) GetAdvertiser(ctx context.Context, id string) (*models.Advertiser, error) {
	var adv models.Advertiser
	err := r.advertisers.FindOne(ctx, bson.M{"_id": id}).Decode(&adv)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	return &adv, err
}

func (r *Repository) ListAdvertisers(ctx context.Context, limit, offset int64) ([]models.Advertiser, error) {
	opts := options.Find().SetLimit(limit).SetSkip(offset).SetSort(bson.D{{Key: "name", Value: 1}})
	cur, err := r.advertisers.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var results []models.Advertiser
	return results, cur.All(ctx, &results)
}

// --- Campaign CRUD ---

func (r *Repository) CreateCampaign(ctx context.Context, c *models.Campaign) error {
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	_, err := r.campaigns.InsertOne(ctx, c)
	return err
}

func (r *Repository) GetCampaign(ctx context.Context, id string) (*models.Campaign, error) {
	var c models.Campaign
	err := r.campaigns.FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	return &c, err
}

func (r *Repository) UpdateCampaign(ctx context.Context, id string, update *models.Campaign) error {
	update.UpdatedAt = time.Now()
	result, err := r.campaigns.ReplaceOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteCampaign(ctx context.Context, id string) error {
	result, err := r.campaigns.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListCampaigns(ctx context.Context, advertiserID string, status models.CampaignStatus, limit, offset int64) ([]models.Campaign, error) {
	filter := bson.M{}
	if advertiserID != "" {
		filter["advertiser_id"] = advertiserID
	}
	if status != "" {
		filter["status"] = status
	}
	opts := options.Find().SetLimit(limit).SetSkip(offset).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.campaigns.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var results []models.Campaign
	return results, cur.All(ctx, &results)
}

func (r *Repository) ListActiveCampaigns(ctx context.Context) ([]models.Campaign, error) {
	now := time.Now()
	filter := bson.M{
		"status":     models.CampaignStatusActive,
		"start_date": bson.M{"$lte": now},
		"end_date":   bson.M{"$gte": now},
	}
	cur, err := r.campaigns.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var results []models.Campaign
	return results, cur.All(ctx, &results)
}

// --- Ad CRUD ---

func (r *Repository) CreateAd(ctx context.Context, ad *models.Ad) error {
	ad.CreatedAt = time.Now()
	ad.UpdatedAt = time.Now()
	_, err := r.ads.InsertOne(ctx, ad)
	return err
}

func (r *Repository) GetAd(ctx context.Context, id string) (*models.Ad, error) {
	var ad models.Ad
	err := r.ads.FindOne(ctx, bson.M{"_id": id}).Decode(&ad)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	return &ad, err
}

func (r *Repository) UpdateAd(ctx context.Context, id string, update *models.Ad) error {
	update.UpdatedAt = time.Now()
	result, err := r.ads.ReplaceOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteAd(ctx context.Context, id string) error {
	result, err := r.ads.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListAdsByCampaign(ctx context.Context, campaignID string) ([]models.Ad, error) {
	cur, err := r.ads.Find(ctx, bson.M{"campaign_id": campaignID})
	if err != nil {
		return nil, err
	}
	var results []models.Ad
	return results, cur.All(ctx, &results)
}
