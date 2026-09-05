package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/servers"
)

type ServersHandler struct {
	svc *servers.Service
}

func NewServersHandler(svc *servers.Service) *ServersHandler {
	return &ServersHandler{svc: svc}
}

func (h *ServersHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req servers.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.UpdateSettings(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) Overview(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Overview(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListNodes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.GetNode(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	var req servers.DeleteNodeRequest
	if r.Body != nil {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, servers.ErrInvalidInput)
			return
		}
	}
	if err := h.svc.DeleteNode(r.Context(), id, req); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServersHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	var req servers.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.UpdateNode(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) UpdateNodeBilling(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	var req servers.UpdateNodeBillingRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.UpdateNodeBilling(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) PairBarn(w http.ResponseWriter, r *http.Request) {
	var req servers.PairBarnRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.PairRemoteBarn(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *ServersHandler) PairDockpilot(w http.ResponseWriter, r *http.Request) {
	h.PairBarn(w, r)
}

func (h *ServersHandler) CreatePairingCode(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.CreatePairingCode(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *ServersHandler) DisconnectMaster(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DisconnectMaster(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServersHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListEvents(r.Context(), 50, 0)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListIncidents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) AcceptPair(w http.ResponseWriter, r *http.Request) {
	var req servers.PairNodeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.AcceptPair(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) NodeStatus(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.LocalNodeStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) NodeApps(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.LocalNodeStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out.Apps)
}

func (h *ServersHandler) NodeVersion(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.LocalNodeStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": out.Version})
}

func (h *ServersHandler) IngestHeartbeat(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := servers.NodeIDFromContext(r.Context())
	if !ok {
		writeError(w, servers.ErrUnauthorized)
		return
	}
	var req servers.HeartbeatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	if err := h.svc.IngestHeartbeat(r.Context(), nodeID, req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ServersHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := servers.NodeIDFromContext(r.Context())
	if !ok {
		writeError(w, servers.ErrUnauthorized)
		return
	}
	var req servers.IngestEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	_, err := h.svc.RecordEvent(r.Context(), nodeID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ServersHandler) StartAgentInstall(w http.ResponseWriter, r *http.Request) {
	var req servers.CreateAgentInstallRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.StartAgentInstall(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	req.Password = ""
	writeJSON(w, http.StatusAccepted, out)
}

func (h *ServersHandler) StartAgentUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	var req servers.UpdateAgentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.StartAgentUpdate(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	req.Password = ""
	writeJSON(w, http.StatusAccepted, out)
}

func (h *ServersHandler) ConfirmHostKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.ConfirmHostKey(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) GetInstallation(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.GetInstallation(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) CancelInstallation(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	if err := h.svc.CancelInstallation(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServersHandler) AgentRegister(w http.ResponseWriter, r *http.Request) {
	var req servers.AgentRegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.RegisterAgent(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServersHandler) NodeBackups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

func (h *ServersHandler) IngestEventBatch(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := servers.NodeIDFromContext(r.Context())
	if !ok {
		writeError(w, servers.ErrUnauthorized)
		return
	}
	var req struct {
		Events []servers.IngestEvent `json:"events"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	for _, ev := range req.Events {
		if _, err := h.svc.RecordEvent(r.Context(), nodeID, ev); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ServersHandler) ListInstallationLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, servers.ErrInvalidInput)
		return
	}
	out, err := h.svc.ListInstallationLogs(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ensure uuid import used
var _ = uuid.Nil
