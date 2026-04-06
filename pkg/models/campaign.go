package models

import "time"

type CampaignStatus string

const (
	CampaignStatusActive   CampaignStatus = "active"
	CampaignStatusPaused   CampaignStatus = "paused"
	CampaignStatusEnded    CampaignStatus = "ended"
	CampaignStatusDraft    CampaignStatus = "draft"
)

type BidStrategy string

const (
	BidStrategyMaxBid    BidStrategy = "max_bid"
	BidStrategyTargetCPA BidStrategy = "target_cpa"
)

type AdType string

const (
	AdTypeBanner AdType = "banner"
	AdTypeVideo  AdType = "video"
	AdTypeNative AdType = "native"
)

type Advertiser struct {
	ID        string    `json:"id" bson:"_id"`
	Name      string    `json:"name" bson:"name"`
	Domain    string    `json:"domain" bson:"domain"`
	Industry  string    `json:"industry" bson:"industry"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type Campaign struct {
	ID           string         `json:"id" bson:"_id"`
	AdvertiserID string         `json:"advertiser_id" bson:"advertiser_id"`
	Name         string         `json:"name" bson:"name"`
	Status       CampaignStatus `json:"status" bson:"status"`
	BidStrategy  BidStrategy    `json:"bid_strategy" bson:"bid_strategy"`
	DailyBudget  float64        `json:"daily_budget" bson:"daily_budget"`
	TotalBudget  float64        `json:"total_budget" bson:"total_budget"`
	BidCeiling   float64        `json:"bid_ceiling" bson:"bid_ceiling"` // max CPM in USD
	BaseBid      float64        `json:"base_bid" bson:"base_bid"`
	Targeting    TargetingRules `json:"targeting" bson:"targeting"`
	AdIDs        []string       `json:"ad_ids" bson:"ad_ids"`
	StartDate    time.Time      `json:"start_date" bson:"start_date"`
	EndDate      time.Time      `json:"end_date" bson:"end_date"`
	DayParting   []int          `json:"day_parting,omitempty" bson:"day_parting,omitempty"` // hours 0-23
	CreatedAt    time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" bson:"updated_at"`
}

type TargetingRules struct {
	AgeMin     int      `json:"age_min,omitempty" bson:"age_min,omitempty"`
	AgeMax     int      `json:"age_max,omitempty" bson:"age_max,omitempty"`
	Genders    []string `json:"genders,omitempty" bson:"genders,omitempty"`     // "male", "female", "unknown"
	Interests  []string `json:"interests,omitempty" bson:"interests,omitempty"` // IAB categories
	Geos       []string `json:"geos,omitempty" bson:"geos,omitempty"`           // ISO 3166-1 alpha-2
	DeviceTypes []string `json:"device_types,omitempty" bson:"device_types,omitempty"` // "ios", "android", "desktop"
	OSTypes    []string `json:"os_types,omitempty" bson:"os_types,omitempty"`
	AdTypes    []AdType `json:"ad_types,omitempty" bson:"ad_types,omitempty"`
}

type Ad struct {
	ID           string    `json:"id" bson:"_id"`
	CampaignID   string    `json:"campaign_id" bson:"campaign_id"`
	AdvertiserID string    `json:"advertiser_id" bson:"advertiser_id"`
	Type         AdType    `json:"type" bson:"type"`
	Title        string    `json:"title" bson:"title"`
	Description  string    `json:"description" bson:"description"`
	ImageURL     string    `json:"image_url,omitempty" bson:"image_url,omitempty"`
	VideoURL     string    `json:"video_url,omitempty" bson:"video_url,omitempty"`
	LandingURL   string    `json:"landing_url" bson:"landing_url"`
	Width        int       `json:"width,omitempty" bson:"width,omitempty"`
	Height       int       `json:"height,omitempty" bson:"height,omitempty"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
}

// CampaignEvent is published to Kafka on campaign mutations.
type CampaignEvent struct {
	Type       CampaignEventType `json:"type"`
	CampaignID string            `json:"campaign_id"`
	Campaign   *Campaign         `json:"campaign,omitempty"`
	Timestamp  int64             `json:"timestamp"` // unix millis
}

type CampaignEventType string

const (
	CampaignCreated CampaignEventType = "campaign.created"
	CampaignUpdated CampaignEventType = "campaign.updated"
	CampaignDeleted CampaignEventType = "campaign.deleted"
)

// CampaignCache is the Redis-cached representation used on the hot path.
type CampaignCache struct {
	ID          string         `json:"id"`
	Status      CampaignStatus `json:"status"`
	DailyBudget float64        `json:"daily_budget"`
	TotalBudget float64        `json:"total_budget"`
	BidCeiling  float64        `json:"bid_ceiling"`
	BaseBid     float64        `json:"base_bid"`
	BidStrategy BidStrategy    `json:"bid_strategy"`
	Targeting   TargetingRules `json:"targeting"`
	AdIDs       []string       `json:"ad_ids"`
	StartDate   int64          `json:"start_date"` // unix seconds
	EndDate     int64          `json:"end_date"`   // unix seconds
}
