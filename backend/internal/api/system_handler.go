package api

import (
	"encoding/json"
	"net/http"

	"github.com/ebash/barn/backend/internal/system"
)

type SystemHandler struct {
	system *system.Service
}

func NewSystemHandler(svc *system.Service) *SystemHandler {
	return &SystemHandler{system: svc}
}

func (h *SystemHandler) Status(w http.ResponseWriter, r *http.Request) {
	st, err := h.system.Status(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *SystemHandler) Processes(w http.ResponseWriter, r *http.Request) {
	st, err := h.system.Processes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *SystemHandler) DockerDirs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.system.DockerDirs(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []system.DirUsage{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *SystemHandler) HostInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.system.GetHostInfo(r.Context()))
}

func (h *SystemHandler) PruneDocker(w http.ResponseWriter, r *http.Request) {
	result, err := h.system.PruneDocker(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SystemHandler) UpdateInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.system.GetUpdateInfo(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *SystemHandler) StartUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	result, err := h.system.StartUpgrade(r.Context(), body.Target)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *SystemHandler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.system.GetUpgradeJob(r.Context()))
}
