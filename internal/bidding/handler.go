package bidding

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/AndrewAct/bidflock/pkg/observability"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/bid", h.bid)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/health", h.health)
	return r
}

func (h *Handler) bid(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req models.BidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid bid request", "err", err)
		observability.BidRequestsTotal.With(prometheus.Labels{
			"ssp_id": "unknown",
			"status": "bad_request",
		}).Inc()
		writeJSON(w, http.StatusBadRequest, models.BidResponse{NBR: models.NBRInvalidRequest})
		return
	}

	if req.ID == "" || len(req.Imp) == 0 {
		writeJSON(w, http.StatusBadRequest, models.BidResponse{NBR: models.NBRInvalidRequest})
		return
	}

	sspID := "unknown"
	if req.Ext != nil {
		sspID = req.Ext.SSPID
	}

	resp, err := h.svc.ProcessBidRequest(r.Context(), &req)
	if err != nil {
		h.logger.Error("bid processing failed", "request_id", req.ID, "err", err)
		observability.BidRequestsTotal.With(prometheus.Labels{"ssp_id": sspID, "status": "error"}).Inc()
		writeJSON(w, http.StatusInternalServerError, models.BidResponse{ID: req.ID, NBR: models.NBRTechnicalError})
		return
	}

	status := "no_bid"
	if len(resp.SeatBid) > 0 {
		status = "bid"
	}
	observability.BidRequestsTotal.With(prometheus.Labels{"ssp_id": sspID, "status": status}).Inc()
	observability.BidAuctionDuration.With(prometheus.Labels{"auction_type": "second_price"}).
		Observe(time.Since(start).Seconds())

	h.logger.Info("bid processed",
		observability.FieldRequestID, req.ID,
		observability.FieldSSPID, sspID,
		observability.FieldStatus, status,
		observability.FieldLatencyMS, time.Since(start).Milliseconds(),
	)

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
