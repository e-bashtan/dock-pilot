package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/healthcheck"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
)

type SitesHandler struct {
	sites *sitesvc.Service
}

func NewSitesHandler(sites *sitesvc.Service) *SitesHandler {
	return &SitesHandler{sites: sites}
}

func (h *SitesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req sitesvc.CreateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	site, err := h.sites.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, site)
}

func (h *SitesHandler) List(w http.ResponseWriter, r *http.Request) {
	sites, err := h.sites.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if sites == nil {
		sites = []sitesvc.SiteListItem{}
	}
	writeJSON(w, http.StatusOK, sites)
}

func (h *SitesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	site, err := h.sites.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, site)
}

func (h *SitesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	var req sitesvc.UpdateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	site, err := h.sites.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, site)
}

func (h *SitesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	if err := h.sites.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SitesHandler) Health(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}
	result, err := h.sites.Health(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SitesHandler) HealthAll(w http.ResponseWriter, r *http.Request) {
	results, err := h.sites.HealthAll(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if results == nil {
		results = []healthcheck.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *SitesHandler) StreamContainerLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming not supported"))
		return
	}

	tail := sitesvc.ParseLogTail(r.URL.Query().Get("tail"), 300)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_ = h.sites.StreamContainerLogs(r.Context(), id, tail, w, flusher)
}

func (h *SitesHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	h.containerAction(w, r, h.sites.StartContainer)
}

func (h *SitesHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	h.containerAction(w, r, h.sites.StopContainer)
}

func (h *SitesHandler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	h.containerAction(w, r, h.sites.RestartContainer)
}

func (h *SitesHandler) containerAction(
	w http.ResponseWriter,
	r *http.Request,
	fn func(context.Context, uuid.UUID) (sitesvc.ContainerActionResponse, error),
) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}
	resp, err := fn(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
