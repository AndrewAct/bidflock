// +build ignore

// seed.go populates MongoDB and Redis with realistic mock data:
//   - 50 advertisers across diverse industries
//   - 200 campaigns with varied budgets, targeting, and bid strategies
//   - 500 ad creatives (banner, video, native)
//
// Usage: go run scripts/seed.go
// Or via Makefile: make seed

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/google/uuid"
)

var campaignBaseURL = envOr("CAMPAIGN_URL", "http://localhost:8082")

var industries = []string{
	"e-commerce", "gaming", "fintech", "education", "health",
	"travel", "automotive", "fashion", "food", "entertainment",
}

var geoTargets = [][]string{
	{"US"}, {"JP"}, {"GB", "DE", "FR"}, {"US", "CA"},
	{"BR"}, {"IN"}, {"US", "GB"}, {"JP", "KR"},
}

var interestTargets = [][]string{
	{"sports", "gaming"}, {"fashion", "beauty"}, {"tech", "finance"},
	{"food", "travel"}, {"education", "health"}, {"gaming", "entertainment"},
}

func main() {
	rng := rand.New(rand.NewSource(42))
	fmt.Println("seeding bidflock...")

	// Create 50 advertisers
	advertisers := make([]*models.Advertiser, 0, 50)
	for i := 0; i < 50; i++ {
		industry := industries[i%len(industries)]
		adv := &models.Advertiser{
			Name:     fmt.Sprintf("%s Corp %d", ucFirst(industry), i+1),
			Domain:   fmt.Sprintf("%s%d.example.com", industry, i+1),
			Industry: industry,
		}
		created, err := postJSON[models.Advertiser]("/advertisers", adv)
		if err != nil {
			fmt.Printf("  advertiser %d failed: %v\n", i+1, err)
			continue
		}
		advertisers = append(advertisers, created)
	}
	fmt.Printf("  created %d advertisers\n", len(advertisers))

	if len(advertisers) == 0 {
		fmt.Fprintln(os.Stderr, "no advertisers created — is the campaign service running?")
		os.Exit(1)
	}

	// Create 200 campaigns
	campaigns := make([]*models.Campaign, 0, 200)
	for i := 0; i < 200; i++ {
		adv := advertisers[i%len(advertisers)]
		dailyBudget := 100.0 + rng.Float64()*9900.0 // $100 - $10K
		baseBid := 0.5 + rng.Float64()*9.5           // $0.50 - $10 CPM

		strategy := models.BidStrategyMaxBid
		if rng.Float64() < 0.3 {
			strategy = models.BidStrategyTargetCPA
		}

		geos := geoTargets[rng.Intn(len(geoTargets))]
		interests := interestTargets[rng.Intn(len(interestTargets))]
		ageMin := 18 + rng.Intn(20)
		ageMax := ageMin + 10 + rng.Intn(25)
		if ageMax > 65 {
			ageMax = 65
		}

		c := &models.Campaign{
			AdvertiserID: adv.ID,
			Name:         fmt.Sprintf("%s Campaign %d", adv.Industry, i+1),
			Status:       models.CampaignStatusActive,
			BidStrategy:  strategy,
			DailyBudget:  dailyBudget,
			TotalBudget:  dailyBudget * 30,
			BidCeiling:   baseBid * 2,
			BaseBid:      baseBid,
			Targeting: models.TargetingRules{
				AgeMin:    ageMin,
				AgeMax:    ageMax,
				Geos:      geos,
				Interests: interests,
				DeviceTypes: pickDeviceTypes(rng),
			},
			StartDate: time.Now().AddDate(0, 0, -rng.Intn(30)),
			EndDate:   time.Now().AddDate(0, rng.Intn(3)+1, 0),
		}

		created, err := postJSON[models.Campaign]("/campaigns", c)
		if err != nil {
			fmt.Printf("  campaign %d failed: %v\n", i+1, err)
			continue
		}
		campaigns = append(campaigns, created)
	}
	fmt.Printf("  created %d campaigns\n", len(campaigns))

	// Create 500 ads (2-3 per campaign)
	adCount := 0
	adTypes := []models.AdType{models.AdTypeBanner, models.AdTypeVideo, models.AdTypeNative}
	for i, c := range campaigns {
		count := 2 + rng.Intn(2)
		for j := 0; j < count && adCount < 500; j++ {
			adType := adTypes[rng.Intn(len(adTypes))]
			ad := &models.Ad{
				CampaignID:   c.ID,
				AdvertiserID: c.AdvertiserID,
				Type:         adType,
				Title:        fmt.Sprintf("Ad %d-%d: %s Offer", i+1, j+1, ucFirst(string(adType))),
				Description:  fmt.Sprintf("Best %s deals — campaign %d", string(adType), i+1),
				LandingURL:   fmt.Sprintf("https://landing.example.com/c/%s/a/%d", c.ID, j+1),
				ImageURL:     fmt.Sprintf("https://cdn.example.com/creatives/%s/%d.jpg", string(adType), adCount),
			}
			switch adType {
			case models.AdTypeBanner:
				ad.Width, ad.Height = 320, 50
			case models.AdTypeVideo:
				ad.Width, ad.Height = 640, 480
				ad.VideoURL = fmt.Sprintf("https://cdn.example.com/videos/%s.mp4", uuid.New())
			}

			_, err := postJSON[models.Ad]("/ads", ad)
			if err == nil {
				adCount++
			}
		}
	}
	fmt.Printf("  created %d ads\n", adCount)
	fmt.Println("\nSeed complete. Run 'make simulate' to start sending traffic.")
}

func postJSON[T any](path string, body interface{}) (*T, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(campaignBaseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http post %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d from %s", resp.StatusCode, path)
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func pickDeviceTypes(rng *rand.Rand) []string {
	all := []string{"ios", "android", "desktop"}
	r := rng.Float64()
	if r < 0.5 {
		return []string{"ios", "android"}
	}
	if r < 0.8 {
		return all
	}
	return []string{"desktop"}
}

func ucFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
