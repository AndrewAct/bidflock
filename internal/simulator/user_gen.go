package simulator

import (
	"fmt"
	"math/rand"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/google/uuid"
)

// Distributions match real-world ad traffic demographics.

var ageGroups = []struct {
	label string
	yob   [2]int // [min, max] year of birth range
	weight float64
}{
	{"18-24", [2]int{2000, 2006}, 0.30},
	{"25-34", [2]int{1990, 1999}, 0.35},
	{"35-44", [2]int{1980, 1989}, 0.20},
	{"45+", [2]int{1960, 1979}, 0.15},
}

var genders = []struct {
	value  string
	weight float64
}{
	{"M", 0.48},
	{"F", 0.50},
	{"O", 0.02},
}

var interestPool = []string{
	"sports", "gaming", "fashion", "tech", "food",
	"travel", "finance", "education", "health", "entertainment",
	"automotive", "beauty", "home", "parenting", "music",
}

var geoDistribution = []struct {
	country string
	weight  float64
}{
	{"US", 0.40},
	{"JP", 0.15},
	{"GB", 0.10},
	{"DE", 0.08},
	{"BR", 0.07},
	{"KR", 0.04},
	{"FR", 0.04},
	{"IN", 0.05},
	{"AU", 0.04},
	{"CA", 0.03},
}

var deviceDistribution = []struct {
	os         string
	deviceType int // OpenRTB devicetype
	weight     float64
}{
	{"iOS", 4, 0.45},
	{"Android", 4, 0.50},
	{"Windows", 2, 0.04},
	{"macOS", 2, 0.01},
}

var sspPartners = []string{
	"mock-adx",
	"mock-tiktok-exchange",
	"mock-pangle",
	"mock-applovin",
	"mock-unity-ads",
	"mock-ironsource",
	"mock-mopub",
	"mock-pubmatic",
}

// UserGen generates realistic user profiles.
type UserGen struct {
	rng *rand.Rand
}

func NewUserGen(seed int64) *UserGen {
	return &UserGen{rng: rand.New(rand.NewSource(seed))}
}

func (g *UserGen) Generate() *models.User {
	return &models.User{
		ID:        uuid.New().String(),
		YOB:       g.pickYOB(),
		Gender:    g.pickWeighted(genders),
		Geo:       &models.Geo{Country: g.pickWeightedGeo()},
		Interests: g.pickInterests(3),
	}
}

func (g *UserGen) pickYOB() int {
	idx := g.pickWeightedIndex(ageGroupWeights())
	group := ageGroups[idx]
	return group.yob[0] + g.rng.Intn(group.yob[1]-group.yob[0]+1)
}

func (g *UserGen) pickWeighted(items interface{}) string {
	switch v := items.(type) {
	case []struct {
		value  string
		weight float64
	}:
		weights := make([]float64, len(v))
		for i, item := range v {
			weights[i] = item.weight
		}
		return v[g.pickWeightedIndex(weights)].value
	}
	return ""
}

func (g *UserGen) pickWeightedGeo() string {
	weights := make([]float64, len(geoDistribution))
	for i, g := range geoDistribution {
		weights[i] = g.weight
	}
	return geoDistribution[g.pickWeightedIndex(weights)].country
}

func (g *UserGen) pickInterests(n int) []string {
	// Pick n distinct interests
	pool := make([]string, len(interestPool))
	copy(pool, interestPool)
	g.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	return pool[:n]
}

func (g *UserGen) pickWeightedIndex(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	r := g.rng.Float64() * total
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if r <= cumulative {
			return i
		}
	}
	return len(weights) - 1
}

// DeviceInfo returns a random device matching our distribution.
func (g *UserGen) DeviceInfo() *models.Device {
	weights := make([]float64, len(deviceDistribution))
	for i, d := range deviceDistribution {
		weights[i] = d.weight
	}
	idx := g.pickWeightedIndex(weights)
	d := deviceDistribution[idx]
	return &models.Device{
		OS:         d.os,
		DeviceType: d.deviceType,
		IP:         fmt.Sprintf("%d.%d.%d.%d", g.rng.Intn(256), g.rng.Intn(256), g.rng.Intn(256), g.rng.Intn(256)),
		UA:         userAgent(d.os),
	}
}

func (g *UserGen) PickSSP() string {
	return sspPartners[g.rng.Intn(len(sspPartners))]
}

func ageGroupWeights() []float64 {
	w := make([]float64, len(ageGroups))
	for i, g := range ageGroups {
		w[i] = g.weight
	}
	return w
}

func userAgent(os string) string {
	switch os {
	case "iOS":
		return "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15"
	case "Android":
		return "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36"
	default:
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
}
