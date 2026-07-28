package api

import (
	"encoding/json"
	"net/http"

	"github.com/ebash/dock-pilot/backend/internal/panelbackup"
)

type BackupsHandler struct {
	svc *panelbackup.Service
}

func NewBackupsHandler(svc *panelbackup.Service) *BackupsHandler {
	return &BackupsHandler{svc: svc}
}

func (h *BackupsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *BackupsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req panelbackup.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, panelbackup.ErrInvalidInput)
		return
	}
	row, err := h.svc.UpdateSettings(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *BackupsHandler) ListFull(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListFullBackups(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []panelbackup.FullBackupInfo{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *BackupsHandler) CreateFull(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.CreateFullBackup(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *BackupsHandler) RestoreFull(w http.ResponseWriter, r *http.Request) {
	var req panelbackup.RestoreFullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, panelbackup.ErrInvalidInput)
		return
	}
	if err := h.svc.RestoreFullBackup(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
