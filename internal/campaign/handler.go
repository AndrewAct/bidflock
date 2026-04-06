package campaign

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/AndrewAct/bidflock/pkg/models"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/advertisers", func(r chi.Router) {
		r.Post("/", h.createAdvertiser)
		r.Get("/", h.listAdvertisers)
		r.Get("/{id}", h.getAdvertiser)
	})

	r.Route("/campaigns", func(r chi.Router) {
		r.Post("/", h.createCampaign)
		r.Get("/", h.listCampaigns)
		r.Get("/{id}", h.getCampaign)
		r.Put("/{id}", h.updateCampaign)
		r.Delete("/{id}", h.deleteCampaign)
	})

	r.Route("/ads", func(r chi.Router) {
		r.Post("/", h.createAd)
		r.Get("/{id}", h.getAd)
		r.Put("/{id}", h.updateAd)
		r.Delete("/{id}", h.deleteAd)
		r.Get("/campaign/{campaignId}", h.listAdsByCampaign)
	})

	return r
}

// --- Advertiser handlers ---

func (h *Handler) createAdvertiser(w http.ResponseWriter, r *http.Request) {
	var req models.Advertiser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	result, err := h.svc.CreateAdvertiser(r.Context(), &req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.svc.GetAdvertiser(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listAdvertisers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	results, err := h.svc.ListAdvertisers(r.Context(), limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// --- Campaign handlers ---

func (h *Handler) createCampaign(w http.ResponseWriter, r *http.Request) {
	var req models.Campaign
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	result, err := h.svc.CreateCampaign(r.Context(), &req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.svc.GetCampaign(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) updateCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.Campaign
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	result, err := h.svc.UpdateCampaign(r.Context(), id, &req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) deleteCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteCampaign(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	advertiserID := r.URL.Query().Get("advertiser_id")
	status := models.CampaignStatus(r.URL.Query().Get("status"))
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	results, err := h.svc.ListCampaigns(r.Context(), advertiserID, status, limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// --- Ad handlers ---

func (h *Handler) createAd(w http.ResponseWriter, r *http.Request) {
	var req models.Ad
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	result, err := h.svc.CreateAd(r.Context(), &req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getAd(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.svc.GetAd(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) updateAd(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.Ad
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	result, err := h.svc.UpdateAd(r.Context(), id, &req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) deleteAd(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteAd(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listAdsByCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "campaignId")
	results, err := h.svc.ListAdsByCampaign(r.Context(), campaignID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// --- helpers ---

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
	} else if errors.Is(err, ErrBadRequest) {
		writeError(w, http.StatusBadRequest, err.Error())
	} else {
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
