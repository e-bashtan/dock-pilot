package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ebash/dock-pilot/backend/internal/fleet"
)

type FleetHandler struct {
	svc *fleet.Service
}

func NewFleetHandler(svc *fleet.Service) *FleetHandler {
	return &FleetHandler{svc: svc}
}

func (h *FleetHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req fleet.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.UpdateSettings(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) Overview(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Overview(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListNodes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.GetNode(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	if err := h.svc.DeleteNode(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FleetHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	var req fleet.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.UpdateNode(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) UpdateNodeBilling(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	var req fleet.UpdateNodeBillingRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.UpdateNodeBilling(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) PairDockpilot(w http.ResponseWriter, r *http.Request) {
	var req fleet.PairDockpilotRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.PairRemoteDockpilot(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *FleetHandler) CreatePairingCode(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.CreatePairingCode(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *FleetHandler) DisconnectMaster(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DisconnectMaster(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FleetHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListEvents(r.Context(), 50, 0)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListIncidents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) AcceptPair(w http.ResponseWriter, r *http.Request) {
	var req fleet.PairNodeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.AcceptPair(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) NodeStatus(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.LocalNodeStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) NodeApps(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.LocalNodeStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out.Apps)
}

func (h *FleetHandler) NodeVersion(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.LocalNodeStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": out.Version})
}

func (h *FleetHandler) IngestHeartbeat(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := fleet.NodeIDFromContext(r.Context())
	if !ok {
		writeError(w, fleet.ErrUnauthorized)
		return
	}
	var req fleet.HeartbeatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	if err := h.svc.IngestHeartbeat(r.Context(), nodeID, req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FleetHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := fleet.NodeIDFromContext(r.Context())
	if !ok {
		writeError(w, fleet.ErrUnauthorized)
		return
	}
	var req fleet.IngestEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	_, err := h.svc.RecordEvent(r.Context(), nodeID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FleetHandler) StartAgentInstall(w http.ResponseWriter, r *http.Request) {
	var req fleet.CreateAgentInstallRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
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

func (h *FleetHandler) ConfirmHostKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.ConfirmHostKey(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) GetInstallation(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.GetInstallation(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) CancelInstallation(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	if err := h.svc.CancelInstallation(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FleetHandler) AgentRegister(w http.ResponseWriter, r *http.Request) {
	var req fleet.AgentRegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
		return
	}
	out, err := h.svc.RegisterAgent(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *FleetHandler) NodeBackups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

func (h *FleetHandler) IngestEventBatch(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := fleet.NodeIDFromContext(r.Context())
	if !ok {
		writeError(w, fleet.ErrUnauthorized)
		return
	}
	var req struct {
		Events []fleet.IngestEvent `json:"events"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		writeError(w, fleet.ErrInvalidInput)
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

func (h *FleetHandler) ListInstallationLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fleet.ErrInvalidInput)
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
