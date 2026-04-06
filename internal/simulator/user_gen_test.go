package simulator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserGen_GeneratesValidUser(t *testing.T) {
	gen := NewUserGen(42)
	user := gen.Generate()

	assert.NotEmpty(t, user.ID)
	assert.NotZero(t, user.YOB)
	assert.GreaterOrEqual(t, user.YOB, 1960)
	assert.LessOrEqual(t, user.YOB, 2006)
	assert.Contains(t, []string{"M", "F", "O"}, user.Gender)
	assert.NotNil(t, user.Geo)
	assert.NotEmpty(t, user.Interests)
	assert.LessOrEqual(t, len(user.Interests), 3)
}

func TestUserGen_GeoDistribution(t *testing.T) {
	gen := NewUserGen(42)
	counts := make(map[string]int)

	const n = 10000
	for i := 0; i < n; i++ {
		user := gen.Generate()
		counts[user.Geo.Country]++
	}

	// US should be most common (~40%)
	usShare := float64(counts["US"]) / n
	assert.Greater(t, usShare, 0.35, "US should be ~40%% of traffic")
	assert.Less(t, usShare, 0.45, "US should be ~40%% of traffic")

	// Japan should be second (~15%)
	jpShare := float64(counts["JP"]) / n
	assert.Greater(t, jpShare, 0.10, "JP should be ~15%%")
	assert.Less(t, jpShare, 0.20, "JP should be ~15%%")
}

func TestUserGen_AgeDistribution(t *testing.T) {
	gen := NewUserGen(42)
	currentYear := 2024

	ageCounts := map[string]int{
		"18-24": 0,
		"25-34": 0,
		"35-44": 0,
		"45+":   0,
	}

	const n = 10000
	for i := 0; i < n; i++ {
		user := gen.Generate()
		age := currentYear - user.YOB
		switch {
		case age >= 18 && age <= 24:
			ageCounts["18-24"]++
		case age >= 25 && age <= 34:
			ageCounts["25-34"]++
		case age >= 35 && age <= 44:
			ageCounts["35-44"]++
		default:
			ageCounts["45+"]++
		}
	}

	// 25-34 should be most common (35%)
	share2534 := float64(ageCounts["25-34"]) / n
	assert.Greater(t, share2534, 0.30, "25-34 should be ~35%%")
	assert.Less(t, share2534, 0.40, "25-34 should be ~35%%")
}

func TestUserGen_Reproducible(t *testing.T) {
	gen1 := NewUserGen(12345)
	gen2 := NewUserGen(12345)

	user1 := gen1.Generate()
	user2 := gen2.Generate()

	// Same seed → same first user (except UUID which is random)
	assert.Equal(t, user1.YOB, user2.YOB)
	assert.Equal(t, user1.Gender, user2.Gender)
}

func TestRequestGen_GeneratesValidRequest(t *testing.T) {
	gen := NewRequestGen(42)
	req := gen.Generate()

	assert.NotEmpty(t, req.ID)
	assert.NotEmpty(t, req.Imp)
	assert.Greater(t, req.Imp[0].BidFloor, 0.0)
	assert.NotNil(t, req.Ext)
	assert.NotEmpty(t, req.Ext.SSPID)

	// Must have either site or app, not neither
	hasContext := req.Site != nil || req.App != nil
	assert.True(t, hasContext, "request must have site or app context")

	// Must have exactly one impression type
	imp := req.Imp[0]
	typeCount := 0
	if imp.Banner != nil { typeCount++ }
	if imp.Video != nil { typeCount++ }
	if imp.Native != nil { typeCount++ }
	assert.Equal(t, 1, typeCount, "impression must have exactly one ad type")
}
