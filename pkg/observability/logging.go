package observability

import (
	"log/slog"
	"os"
)

// NewLogger creates a structured JSON logger for a service.
func NewLogger(service string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler).With("service", service)
}

// Standard log field keys for consistent querying across services.
const (
	FieldRequestID  = "request_id"
	FieldCampaignID = "campaign_id"
	FieldAdID       = "ad_id"
	FieldUserID     = "user_id"
	FieldSSPID      = "ssp_id"
	FieldBidPrice   = "bid_price"
	FieldLatencyMS  = "latency_ms"
	FieldError      = "error"
	FieldStatus     = "status"
)
