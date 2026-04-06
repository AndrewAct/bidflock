package simulator

import (
	"fmt"
	"math/rand"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/google/uuid"
)

// Ad slot configurations matching real-world SSP inventory.
var adSlots = []struct {
	slotType string
	width    int
	height   int
	floorCPM float64 // USD CPM
	weight   float64
}{
	{"banner", 320, 50, 0.30, 0.35},   // mobile banner
	{"banner", 300, 250, 0.50, 0.25},   // medium rectangle
	{"banner", 728, 90, 0.40, 0.10},    // leaderboard
	{"video", 640, 480, 1.50, 0.20},    // interstitial video
	{"native", 0, 0, 0.60, 0.10},       // native ad
}

var appBundles = []string{
	"com.example.hypercasual", "com.example.socialmedia",
	"com.example.newsreader", "com.example.weatherapp",
	"com.example.musicplayer", "com.example.fooddelivery",
	"com.example.rideshare", "com.example.fintech",
}

var siteDomains = []string{
	"news.example.com", "sports.example.com",
	"tech.example.com", "lifestyle.example.com",
	"entertainment.example.com",
}

// RequestGen builds OpenRTB 2.6 compliant bid requests.
type RequestGen struct {
	userGen *UserGen
	rng     *rand.Rand
}

func NewRequestGen(seed int64) *RequestGen {
	return &RequestGen{
		userGen: NewUserGen(seed),
		rng:     rand.New(rand.NewSource(seed + 1)),
	}
}

func (g *RequestGen) Generate() *models.BidRequest {
	user := g.userGen.Generate()
	device := g.userGen.DeviceInfo()
	sspID := g.userGen.PickSSP()

	// Geo from user
	if user.Geo != nil && device.Geo == nil {
		device.Geo = user.Geo
	}

	// Pick an ad slot
	slotIdx := g.pickSlotIndex()
	slot := adSlots[slotIdx]

	req := &models.BidRequest{
		ID:     uuid.New().String(),
		User:   user,
		Device: device,
		AT:     models.AuctionSecondPrice,
		TMax:   150, // 150ms max
		Cur:    []string{"USD"},
		Ext: &models.BidRequestExt{
			SSPID:    sspID,
			Exchange: sspID,
		},
		Imp: []models.Imp{
			{
				ID:          uuid.New().String(),
				BidFloor:    slot.floorCPM,
				BidFloorCur: "USD",
				Secure:      1,
			},
		},
	}

	// Fill impression type
	switch slot.slotType {
	case "banner":
		req.Imp[0].Banner = &models.Banner{W: slot.width, H: slot.height, Pos: 1}
	case "video":
		req.Imp[0].Video = &models.Video{
			MIMEs:       []string{"video/mp4"},
			MinDuration: 5,
			MaxDuration: 30,
			Protocols:   []int{2, 3},
			W:           slot.width,
			H:           slot.height,
		}
	case "native":
		req.Imp[0].Native = &models.Native{Request: `{"ver":"1.2"}`, Ver: "1.2"}
	}

	// Assign App or Site depending on device type
	if device.OS == "iOS" || device.OS == "Android" {
		req.App = g.genApp()
	} else {
		req.Site = g.genSite()
	}

	return req
}

func (g *RequestGen) genApp() *models.App {
	bundle := appBundles[g.rng.Intn(len(appBundles))]
	return &models.App{
		ID:     fmt.Sprintf("app-%d", g.rng.Intn(100)),
		Name:   bundle,
		Bundle: bundle,
		Cat:    []string{"IAB9"},
		Publisher: &models.Publisher{
			ID:   g.userGen.PickSSP(),
			Name: "Mock Publisher",
		},
	}
}

func (g *RequestGen) genSite() *models.Site {
	domain := siteDomains[g.rng.Intn(len(siteDomains))]
	return &models.Site{
		ID:     fmt.Sprintf("site-%d", g.rng.Intn(100)),
		Name:   domain,
		Domain: domain,
		Cat:    []string{"IAB12"},
		Page:   fmt.Sprintf("https://%s/article/%d", domain, g.rng.Intn(10000)),
		Publisher: &models.Publisher{
			ID:     g.userGen.PickSSP(),
			Name:   "Mock Publisher",
			Domain: domain,
		},
	}
}

func (g *RequestGen) pickSlotIndex() int {
	weights := make([]float64, len(adSlots))
	for i, s := range adSlots {
		weights[i] = s.weight
	}
	return g.userGen.pickWeightedIndex(weights)
}
