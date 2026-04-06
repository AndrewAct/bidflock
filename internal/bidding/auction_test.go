package bidding

import (
	"testing"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAuction_SecondPrice(t *testing.T) {
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 5.00, BudgetOK: true},
		{CampaignID: "c2", EffectiveBid: 3.50, BudgetOK: true},
		{CampaignID: "c3", EffectiveBid: 2.00, BudgetOK: true},
	}

	winner, noBid := RunAuction(candidates, SecondPrice, 0.30)

	require.False(t, noBid)
	require.NotNil(t, winner)
	assert.Equal(t, "c1", winner.CampaignID)
	// Second-price: winner pays second-highest bid
	assert.Equal(t, 3.50, winner.ClearingPrice)
}

func TestRunAuction_FirstPrice(t *testing.T) {
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 5.00, BudgetOK: true},
		{CampaignID: "c2", EffectiveBid: 3.50, BudgetOK: true},
	}

	winner, noBid := RunAuction(candidates, FirstPrice, 0.30)

	require.False(t, noBid)
	assert.Equal(t, "c1", winner.CampaignID)
	// First-price: winner pays their own bid
	assert.Equal(t, 5.00, winner.ClearingPrice)
}

func TestRunAuction_SingleBidder_PaysFloor(t *testing.T) {
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 5.00, BudgetOK: true},
	}

	winner, noBid := RunAuction(candidates, SecondPrice, 1.00)

	require.False(t, noBid)
	assert.Equal(t, "c1", winner.CampaignID)
	// Only bidder pays the floor price in second-price auction
	assert.Equal(t, 1.00, winner.ClearingPrice)
}

func TestRunAuction_NoBidders(t *testing.T) {
	candidates := []models.BidCandidate{}

	winner, noBid := RunAuction(candidates, SecondPrice, 0.30)

	assert.True(t, noBid)
	assert.Nil(t, winner)
}

func TestRunAuction_AllBudgetExhausted(t *testing.T) {
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 5.00, BudgetOK: false},
		{CampaignID: "c2", EffectiveBid: 3.00, BudgetOK: false},
	}

	winner, noBid := RunAuction(candidates, SecondPrice, 0.30)

	assert.True(t, noBid)
	assert.Nil(t, winner)
}

func TestRunAuction_BelowFloor(t *testing.T) {
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 0.10, BudgetOK: true},
	}

	winner, noBid := RunAuction(candidates, SecondPrice, 0.50)

	// Bid below floor should result in no-bid
	assert.True(t, noBid)
	assert.Nil(t, winner)
}

func TestRunAuction_ClearingPriceEnforcesFloor(t *testing.T) {
	// Two bidders where second bid is below floor
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 5.00, BudgetOK: true},
		{CampaignID: "c2", EffectiveBid: 0.10, BudgetOK: true}, // below floor
	}

	winner, noBid := RunAuction(candidates, SecondPrice, 1.00)

	require.False(t, noBid)
	assert.Equal(t, "c1", winner.CampaignID)
	// c2 is filtered out (below floor), so single bidder pays floor
	assert.Equal(t, 1.00, winner.ClearingPrice)
}

func TestRunAuction_MixedBudgetOK(t *testing.T) {
	candidates := []models.BidCandidate{
		{CampaignID: "c1", EffectiveBid: 10.00, BudgetOK: false}, // highest bid but no budget
		{CampaignID: "c2", EffectiveBid: 5.00, BudgetOK: true},
		{CampaignID: "c3", EffectiveBid: 3.00, BudgetOK: true},
	}

	winner, noBid := RunAuction(candidates, SecondPrice, 0.30)

	require.False(t, noBid)
	assert.Equal(t, "c2", winner.CampaignID)
	assert.Equal(t, 3.00, winner.ClearingPrice)
}
