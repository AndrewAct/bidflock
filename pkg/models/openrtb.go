// Package models contains OpenRTB 2.6 aligned bid request/response types
// used by the bidding HTTP API and traffic simulator.
package models

import "encoding/json"

// AuctionType mirrors OpenRTB at field.
const (
	AuctionFirstPrice  = 1
	AuctionSecondPrice = 2
)

// NoBid reason codes (OpenRTB 2.6 section 7.2)
const (
	NBRUnknownError      = 0
	NBRTechnicalError    = 1
	NBRInvalidRequest    = 2
	NBRKnownWebSpider    = 3
	NBRSuspectedNonHuman = 4
	NBRBelowFloor        = 5
	NBRNoAdFound         = 7
	NBRFrequencyCapped   = 8
	NBRBudgetConstraints = 9
	NBRTimeout           = 10
)

type BidRequest struct {
	ID     string  `json:"id"`
	Imp    []Imp   `json:"imp"`
	Site   *Site   `json:"site,omitempty"`
	App    *App    `json:"app,omitempty"`
	User   *User   `json:"user,omitempty"`
	Device *Device `json:"device,omitempty"`
	AT     int     `json:"at,omitempty"`   // auction type: 1=first, 2=second
	TMax   int     `json:"tmax,omitempty"` // max latency ms
	WSeat  []string `json:"wseat,omitempty"`
	Cur    []string `json:"cur,omitempty"`
	Ext    *BidRequestExt `json:"ext,omitempty"`
}

type BidRequestExt struct {
	SSPID    string `json:"ssp_id"`
	Exchange string `json:"exchange"`
}

type Imp struct {
	ID          string          `json:"id"`
	Banner      *Banner         `json:"banner,omitempty"`
	Video       *Video          `json:"video,omitempty"`
	Native      *Native         `json:"native,omitempty"`
	BidFloor    float64         `json:"bidfloor,omitempty"`
	BidFloorCur string          `json:"bidfloorcur,omitempty"`
	Secure      int             `json:"secure,omitempty"`
	Ext         json.RawMessage `json:"ext,omitempty"`
}

type Banner struct {
	W     int   `json:"w,omitempty"`
	H     int   `json:"h,omitempty"`
	BType []int `json:"btype,omitempty"`
	BAttr []int `json:"battr,omitempty"`
	Pos   int   `json:"pos,omitempty"`
}

type Video struct {
	MIMEs       []string `json:"mimes"`
	MinDuration int      `json:"minduration,omitempty"`
	MaxDuration int      `json:"maxduration,omitempty"`
	Protocols   []int    `json:"protocols,omitempty"`
	W           int      `json:"w,omitempty"`
	H           int      `json:"h,omitempty"`
	StartDelay  int      `json:"startdelay,omitempty"`
	Linearity   int      `json:"linearity,omitempty"`
	API         []int    `json:"api,omitempty"`
}

type Native struct {
	Request string `json:"request"`
	Ver     string `json:"ver,omitempty"`
	API     []int  `json:"api,omitempty"`
}

type Site struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Domain    string    `json:"domain,omitempty"`
	Cat       []string  `json:"cat,omitempty"`
	Page      string    `json:"page,omitempty"`
	Ref       string    `json:"ref,omitempty"`
	Publisher *Publisher `json:"publisher,omitempty"`
}

type App struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Bundle    string    `json:"bundle,omitempty"`
	Cat       []string  `json:"cat,omitempty"`
	Ver       string    `json:"ver,omitempty"`
	Publisher *Publisher `json:"publisher,omitempty"`
	StoreURL  string    `json:"storeurl,omitempty"`
}

type Publisher struct {
	ID     string   `json:"id,omitempty"`
	Name   string   `json:"name,omitempty"`
	Cat    []string `json:"cat,omitempty"`
	Domain string   `json:"domain,omitempty"`
}

type User struct {
	ID        string   `json:"id,omitempty"`
	YOB       int      `json:"yob,omitempty"`   // year of birth
	Gender    string   `json:"gender,omitempty"` // "M", "F", "O"
	Geo       *Geo     `json:"geo,omitempty"`
	Interests []string `json:"interests,omitempty"`
}

type Device struct {
	UA         string `json:"ua,omitempty"`
	IP         string `json:"ip,omitempty"`
	Geo        *Geo   `json:"geo,omitempty"`
	Make       string `json:"make,omitempty"`
	Model      string `json:"model,omitempty"`
	OS         string `json:"os,omitempty"`
	OSV        string `json:"osv,omitempty"`
	DeviceType int    `json:"devicetype,omitempty"` // 1=mobile, 2=pc, 4=phone, 5=tablet
	JS         int    `json:"js,omitempty"`
	Language   string `json:"language,omitempty"`
	Carrier    string `json:"carrier,omitempty"`
	IFA        string `json:"ifa,omitempty"` // IDFA/GAID
}

type Geo struct {
	Lat     float64 `json:"lat,omitempty"`
	Lon     float64 `json:"lon,omitempty"`
	Country string  `json:"country,omitempty"` // ISO 3166-1 alpha-2
	Region  string  `json:"region,omitempty"`
	City    string  `json:"city,omitempty"`
	ZIP     string  `json:"zip,omitempty"`
}

type BidResponse struct {
	ID      string    `json:"id"`
	SeatBid []SeatBid `json:"seatbid,omitempty"`
	BidID   string    `json:"bidid,omitempty"`
	Cur     string    `json:"cur,omitempty"`
	NBR     int       `json:"nbr,omitempty"` // no-bid reason
}

type SeatBid struct {
	Bid  []Bid  `json:"bid"`
	Seat string `json:"seat,omitempty"`
}

type Bid struct {
	ID           string          `json:"id"`
	ImpID        string          `json:"impid"`
	Price        float64         `json:"price"`
	AdID         string          `json:"adid,omitempty"`
	NURL         string          `json:"nurl,omitempty"`
	BURL         string          `json:"burl,omitempty"`
	AdM          string          `json:"adm,omitempty"`
	ADomain      []string        `json:"adomain,omitempty"`
	CID          string          `json:"cid,omitempty"`  // campaign ID
	CrID         string          `json:"crid,omitempty"` // creative ID
	W            int             `json:"w,omitempty"`
	H            int             `json:"h,omitempty"`
	Ext          json.RawMessage `json:"ext,omitempty"`
}

// BidResult is published to Kafka after each auction.
type BidResult struct {
	RequestID         string         `json:"request_id"`
	AuctionType       string         `json:"auction_type"` // "first_price" | "second_price"
	Candidates        []BidCandidate `json:"candidates"`
	Winner            *BidCandidate  `json:"winner,omitempty"`
	NoBid             bool           `json:"no_bid"`
	NoBidReason       int            `json:"no_bid_reason,omitempty"`
	AuctionDurationUS int64          `json:"auction_duration_us"`
	SSPID             string         `json:"ssp_id"`
	Timestamp         int64          `json:"timestamp"`
}

type BidCandidate struct {
	CampaignID    string  `json:"campaign_id"`
	AdID          string  `json:"ad_id"`
	RawBid        float64 `json:"raw_bid"`
	EffectiveBid  float64 `json:"effective_bid"`
	ClearingPrice float64 `json:"clearing_price,omitempty"`
	PredictedCTR  float64 `json:"predicted_ctr"`
	BudgetOK      bool    `json:"budget_ok"`
}
