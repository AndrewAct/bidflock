package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/AndrewAct/bidflock/pkg/observability"
	"github.com/twmb/franz-go/pkg/kgo"
)

type ImpressionHandler struct {
	writer *BatchWriter
}

func NewImpressionHandler(writer *BatchWriter) *ImpressionHandler {
	return &ImpressionHandler{writer: writer}
}

func (h *ImpressionHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event models.ImpressionEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		observability.EventsProcessed.WithLabelValues("impression", "parse_error").Inc()
		return fmt.Errorf("unmarshal impression: %w", err)
	}

	h.writer.WriteImpression(ctx, ImpressionRow{
		EventID:    event.EventID,
		BidID:      event.BidID,
		RequestID:  event.RequestID,
		CampaignID: event.CampaignID,
		AdID:       event.AdID,
		UserID:     event.UserID,
		SSPID:      event.SSPID,
		Price:      event.Price,
		Timestamp:  time.UnixMilli(event.Timestamp),
	})
	observability.EventsProcessed.WithLabelValues("impression", "ok").Inc()
	return nil
}
