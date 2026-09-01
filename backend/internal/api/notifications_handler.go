package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	notifpkg "github.com/ebash/barn/backend/internal/notifications"
)

type NotificationsHandler struct {
	notifications *notifpkg.Service
	tunnel        *notifpkg.TunnelManager
}

func NewNotificationsHandler(notifications *notifpkg.Service, tunnel *notifpkg.TunnelManager) *NotificationsHandler {
	return &NotificationsHandler{notifications: notifications, tunnel: tunnel}
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

func (h *NotificationsHandler) TunnelStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.tunnel.Status(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *NotificationsHandler) GenerateTunnelKey(w http.ResponseWriter, r *http.Request) {
	var cfg notifpkg.TunnelConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, notifpkg.ErrInvalidInput)
		return
	}
	status, err := h.tunnel.GenerateKey(r.Context(), cfg)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *NotificationsHandler) TestTunnelSSH(w http.ResponseWriter, r *http.Request) {
	if err := h.tunnel.TestSSH(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (h *NotificationsHandler) StartTunnel(w http.ResponseWriter, r *http.Request) {
	status, err := h.tunnel.Start(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := h.notifications.SetTelegramProxy(r.Context(), "socks5://127.0.0.1:"+strconv.Itoa(status.Config.LocalPort)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *NotificationsHandler) StopTunnel(w http.ResponseWriter, r *http.Request) {
	status, err := h.tunnel.Stop(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *NotificationsHandler) RestartTunnel(w http.ResponseWriter, r *http.Request) {
	status, err := h.tunnel.Restart(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *NotificationsHandler) DeleteTunnel(w http.ResponseWriter, r *http.Request) {
	status, _ := h.tunnel.Status(r.Context())
	if err := h.tunnel.Delete(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	settings, err := h.notifications.GetSettings(r.Context())
	if err == nil && status.Config.LocalPort > 0 && settings.TelegramHTTPProxy == "socks5://127.0.0.1:"+strconv.Itoa(status.Config.LocalPort) {
		_, _ = h.notifications.SetTelegramProxy(r.Context(), "")
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationsHandler) TunnelLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.tunnel.Logs(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}
