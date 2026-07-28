package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func (h *BackupsHandler) StreamRestoreFull(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("s3_key"))
	if key == "" {
		writeError(w, panelbackup.ErrInvalidInput)
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

	writeLog := func(level, message string) {
		payload, _ := json.Marshal(map[string]string{
			"level":   level,
			"message": message,
			"at":      time.Now().UTC().Format(time.RFC3339),
		})
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
		flusher.Flush()
	}

	err := h.svc.RestoreFullBackupWithLog(r.Context(), panelbackup.RestoreFullRequest{S3Key: key}, writeLog)
	if err != nil {
		writeLog("error", err.Error())
		_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"failed\"}\n\n")
		flusher.Flush()
		return
	}
	_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"succeeded\"}\n\n")
	flusher.Flush()
}
