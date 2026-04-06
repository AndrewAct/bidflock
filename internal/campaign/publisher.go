package campaign

import (
	"context"

	"github.com/AndrewAct/bidflock/pkg/kafka"
	"github.com/AndrewAct/bidflock/pkg/models"
)

type Publisher struct {
	producer *kafka.Producer
}

func NewPublisher(producer *kafka.Producer) *Publisher {
	return &Publisher{producer: producer}
}

func (p *Publisher) PublishCampaignEvent(ctx context.Context, event *models.CampaignEvent) error {
	return p.producer.Publish(ctx, kafka.TopicCampaignUpdates, event.CampaignID, event)
}
