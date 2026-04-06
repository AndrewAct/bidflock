package models

// AdEvent types published to Kafka for attribution.
type EventType string

const (
	EventImpression EventType = "impression"
	EventClick      EventType = "click"
	EventConversion EventType = "conversion"
)

type ImpressionEvent struct {
	EventID    string  `json:"event_id"`
	BidID      string  `json:"bid_id"`
	RequestID  string  `json:"request_id"`
	CampaignID string  `json:"campaign_id"`
	AdID       string  `json:"ad_id"`
	UserID     string  `json:"user_id"`
	SSPID      string  `json:"ssp_id"`
	Price      float64 `json:"price"`
	Timestamp  int64   `json:"timestamp"`
}

type ClickEvent struct {
	EventID      string `json:"event_id"`
	ImpressionID string `json:"impression_id"`
	CampaignID   string `json:"campaign_id"`
	AdID         string `json:"ad_id"`
	UserID       string `json:"user_id"`
	Timestamp    int64  `json:"timestamp"`
}

type ConversionEvent struct {
	EventID      string  `json:"event_id"`
	ClickID      string  `json:"click_id"`
	CampaignID   string  `json:"campaign_id"`
	AdID         string  `json:"ad_id"`
	UserID       string  `json:"user_id"`
	Value        float64 `json:"value"` // conversion value in USD
	Timestamp    int64   `json:"timestamp"`
}
