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

type ConversionHandler struct {
	writer *BatchWriter
}

func NewConversionHandler(writer *BatchWriter) *ConversionHandler {
	return &ConversionHandler{writer: writer}
}

func (h *ConversionHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event models.ConversionEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		observability.EventsProcessed.WithLabelValues("conversion", "parse_error").Inc()
		return fmt.Errorf("unmarshal conversion: %w", err)
	}

	h.writer.WriteConversion(ctx, ConversionRow{
		EventID:    event.EventID,
		ClickID:    event.ClickID,
		CampaignID: event.CampaignID,
		AdID:       event.AdID,
		UserID:     event.UserID,
		Value:      event.Value,
		Timestamp:  time.UnixMilli(event.Timestamp),
	})
	observability.EventsProcessed.WithLabelValues("conversion", "ok").Inc()
	return nil
}
