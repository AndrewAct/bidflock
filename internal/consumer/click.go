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

type ClickHandler struct {
	writer *BatchWriter
}

func NewClickHandler(writer *BatchWriter) *ClickHandler {
	return &ClickHandler{writer: writer}
}

func (h *ClickHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event models.ClickEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		observability.EventsProcessed.WithLabelValues("click", "parse_error").Inc()
		return fmt.Errorf("unmarshal click: %w", err)
	}

	h.writer.WriteClick(ctx, ClickRow{
		EventID:      event.EventID,
		ImpressionID: event.ImpressionID,
		CampaignID:   event.CampaignID,
		AdID:         event.AdID,
		UserID:       event.UserID,
		Timestamp:    time.UnixMilli(event.Timestamp),
	})
	observability.EventsProcessed.WithLabelValues("click", "ok").Inc()
	return nil
}
