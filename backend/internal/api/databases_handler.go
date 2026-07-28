package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ebash/dock-pilot/backend/internal/pgdb"
)

type DatabasesHandler struct {
	svc *pgdb.Service
}

func NewDatabasesHandler(svc *pgdb.Service) *DatabasesHandler {
	return &DatabasesHandler{svc: svc}
}

func (h *DatabasesHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListInstances(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.InstanceResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) CreateInstance(w http.ResponseWriter, r *http.Request) {
	var req pgdb.CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.CreateInstance(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *DatabasesHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) DeployInstance(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.DeployInstance(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) StopInstance(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.StopInstance(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	if err := h.svc.DeleteInstance(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DatabasesHandler) ListDatabases(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	rows, err := h.svc.ListDatabases(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.DatabaseResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) CreateDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.CreateDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.CreateDatabase(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *DatabasesHandler) DeleteDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	dbID, err := parseUUID(chi.URLParam(r, "dbId"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	if err := h.svc.DeleteDatabase(r.Context(), id, dbID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DatabasesHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	rows, err := h.svc.ListRoles(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.RoleResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.CreateRole(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *DatabasesHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	roleID, err := parseUUID(chi.URLParam(r, "roleId"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	if err := h.svc.DeleteRole(r.Context(), id, roleID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DatabasesHandler) GrantRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	roleID, err := parseUUID(chi.URLParam(r, "roleId"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.GrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.GrantRole(r.Context(), id, roleID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) ConnectionInfo(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	dbID, err := parseUUID(r.URL.Query().Get("database_id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	roleID, err := parseUUID(r.URL.Query().Get("role_id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.ConnectionInfo(r.Context(), id, dbID, roleID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	rows, err := h.svc.ListSchedules(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.ScheduleResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.CreateSchedule(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *DatabasesHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	scheduleID, err := parseUUID(chi.URLParam(r, "scheduleId"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.UpdateSchedule(r.Context(), id, scheduleID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	scheduleID, err := parseUUID(chi.URLParam(r, "scheduleId"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	if err := h.svc.DeleteSchedule(r.Context(), id, scheduleID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DatabasesHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	rows, err := h.svc.ListBackups(r.Context(), id, 50)
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.BackupResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) ManualBackup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.ManualBackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.ManualBackup(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *DatabasesHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	backupID, err := parseUUID(chi.URLParam(r, "backupId"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	var req pgdb.RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.RestoreBackup(r.Context(), id, backupID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}
