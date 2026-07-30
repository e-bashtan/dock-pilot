package api

import (
	"encoding/json"
	"net/http"

	notifpkg "github.com/ebash/barn/backend/internal/notifications"
)

type NotificationsHandler struct {
	notifications *notifpkg.Service
}

func NewNotificationsHandler(notifications *notifpkg.Service) *NotificationsHandler {
	return &NotificationsHandler{notifications: notifications}
}

func (h *NotificationsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.notifications.GetSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *NotificationsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req notifpkg.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, notifpkg.ErrInvalidInput)
		return
	}

	settings, err := h.notifications.UpdateSettings(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *NotificationsHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	if err := h.notifications.SendTest(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
