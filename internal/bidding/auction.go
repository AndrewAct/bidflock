package bidding

import (
	"sort"

	"github.com/AndrewAct/bidflock/pkg/models"
)

// AuctionType determines clearing price logic.
type AuctionType int

const (
	FirstPrice  AuctionType = 1
	SecondPrice AuctionType = 2
)

// RunAuction selects a winner from eligible candidates.
// All candidates must already have BudgetOK == true.
func RunAuction(candidates []models.BidCandidate, auctionType AuctionType, floorPrice float64) (winner *models.BidCandidate, noBid bool) {
	eligible := make([]models.BidCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.BudgetOK && c.EffectiveBid >= floorPrice {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil, true
	}

	// Sort by effective bid descending
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].EffectiveBid > eligible[j].EffectiveBid
	})

	winner = &eligible[0]

	switch auctionType {
	case SecondPrice:
		// Winner pays second-highest price (or floor if only one bidder)
		if len(eligible) > 1 {
			winner.ClearingPrice = eligible[1].EffectiveBid
		} else {
			winner.ClearingPrice = floorPrice
		}
	case FirstPrice:
		winner.ClearingPrice = winner.EffectiveBid
	}

	// Enforce floor
	if winner.ClearingPrice < floorPrice {
		winner.ClearingPrice = floorPrice
	}

	return winner, false
}
