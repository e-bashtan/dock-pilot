package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	deploysvc "github.com/ebash/barn/backend/internal/deployments"
)

type DeploymentsHandler struct {
	deployments *deploysvc.Service
}

func NewDeploymentsHandler(deployments *deploysvc.Service) *DeploymentsHandler {
	return &DeploymentsHandler{deployments: deployments}
}

func (h *DeploymentsHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	siteID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, deploysvc.ErrNotFound)
		return
	}

	dep, err := h.deployments.StartDeploy(r.Context(), siteID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, dep)
}

func (h *DeploymentsHandler) ListBySite(w http.ResponseWriter, r *http.Request) {
	siteID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, deploysvc.ErrNotFound)
		return
	}

	deps, err := h.deployments.ListBySite(r.Context(), siteID)
	if err != nil {
		writeError(w, err)
		return
	}
	if deps == nil {
		deps = []deploysvc.DeploymentResponse{}
	}
	writeJSON(w, http.StatusOK, deps)
}

func (h *DeploymentsHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	depID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, deploysvc.ErrNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := h.deployments.StreamLogs(r.Context(), depID, w, flusher); err != nil {
		// Client disconnect is common; only log server errors.
		return
	}
}
