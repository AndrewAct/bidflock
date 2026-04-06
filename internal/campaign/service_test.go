package campaign

import (
	"context"
	"testing"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCampaign(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)

	cases := []struct {
		name    string
		input   models.Campaign
		wantErr bool
	}{
		{
			name: "valid campaign",
			input: models.Campaign{
				Name: "Test Campaign", AdvertiserID: "adv-1",
				DailyBudget: 100, BaseBid: 2.0, EndDate: future,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			input: models.Campaign{
				AdvertiserID: "adv-1", DailyBudget: 100, BaseBid: 2.0, EndDate: future,
			},
			wantErr: true,
		},
		{
			name: "missing advertiser_id",
			input: models.Campaign{
				Name: "Test", DailyBudget: 100, BaseBid: 2.0, EndDate: future,
			},
			wantErr: true,
		},
		{
			name: "zero daily budget",
			input: models.Campaign{
				Name: "Test", AdvertiserID: "adv-1", DailyBudget: 0, BaseBid: 2.0, EndDate: future,
			},
			wantErr: true,
		},
		{
			name: "negative base bid",
			input: models.Campaign{
				Name: "Test", AdvertiserID: "adv-1", DailyBudget: 100, BaseBid: -1.0, EndDate: future,
			},
			wantErr: true,
		},
		{
			name: "end date in past",
			input: models.Campaign{
				Name: "Test", AdvertiserID: "adv-1",
				DailyBudget: 100, BaseBid: 2.0,
				EndDate: time.Now().Add(-time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCampaign(&tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_PublishEvent_DoesNotPanic(t *testing.T) {
	// Just verify the error path in publishEvent doesn't panic
	logger := newTestLogger()
	svc := &Service{
		repo:      nil,
		publisher: &Publisher{producer: nil},
		logger:    logger,
	}
	// Should not panic even with nil producer
	assert.NotPanics(t, func() {
		svc.publishEvent(context.Background(), models.CampaignCreated, "test-id", nil)
	})
}
