package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/pgdb"
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

func (h *DatabasesHandler) HealthAll(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.HealthAll(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.HealthResult{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) Health(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.Health(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
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

func (h *DatabasesHandler) StreamDeploy(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
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

	row, err := h.svc.DeployInstanceWithLog(r.Context(), id, writeLog)
	if err != nil {
		writeLog("error", err.Error())
		_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"failed\"}\n\n")
		flusher.Flush()
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"status":    row.Status,
		"host_port": row.HostPort,
	})
	_, _ = fmt.Fprintf(w, "event: done\ndata: %s\n\n", payload)
	flusher.Flush()
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

func (h *DatabasesHandler) ListTables(w http.ResponseWriter, r *http.Request) {
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
	rows, err := h.svc.ListPublicTables(r.Context(), id, dbID)
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []pgdb.TableInfo{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *DatabasesHandler) SelectTable(w http.ResponseWriter, r *http.Request) {
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
	var req pgdb.SelectTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.SelectFromTable(r.Context(), id, dbID, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
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

func (h *DatabasesHandler) AdminCredentials(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.AdminCredentials(r.Context(), id)
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
	var scheduleID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("schedule_id")); raw != "" {
		sid, err := parseUUID(raw)
		if err != nil {
			writeError(w, pgdb.ErrInvalidInput)
			return
		}
		scheduleID = &sid
	}
	rows, err := h.svc.ListBackups(r.Context(), id, scheduleID, 100)
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
	var req pgdb.RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	row, err := h.svc.RestoreBackup(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *DatabasesHandler) StreamRestoreBackup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}

	scheduleID, err := parseUUID(r.URL.Query().Get("schedule_id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}
	s3Key := strings.TrimSpace(r.URL.Query().Get("s3_key"))
	if s3Key == "" {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}

	req := pgdb.RestoreRequest{
		ScheduleID:         scheduleID,
		S3Key:              s3Key,
		TargetDatabaseName: strings.TrimSpace(r.URL.Query().Get("target_database_name")),
		CreateDatabase:     r.URL.Query().Get("create_database") != "false",
		DropExisting:       r.URL.Query().Get("drop_existing") == "true",
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

	_, err = h.svc.RestoreBackupWithLog(r.Context(), id, req, writeLog)
	if err != nil {
		writeLog("error", err.Error())
		_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"failed\"}\n\n")
		flusher.Flush()
		return
	}
	_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"succeeded\"}\n\n")
	flusher.Flush()
}

const maxRestoreUploadBytes = 512 << 20 // 512 MiB

func (h *DatabasesHandler) RestoreUpload(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, pgdb.ErrInvalidInput)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRestoreUploadBytes+1024)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, fmt.Errorf("%w: upload too large or invalid multipart form", pgdb.ErrInvalidInput))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, fmt.Errorf("%w: file is required", pgdb.ErrInvalidInput))
		return
	}
	defer file.Close()

	name := strings.ToLower(header.Filename)
	if !strings.HasSuffix(name, ".sql") && !strings.HasSuffix(name, ".sql.gz") && !strings.HasSuffix(name, ".gz") {
		writeError(w, fmt.Errorf("%w: expected .sql or .sql.gz file", pgdb.ErrInvalidInput))
		return
	}

	target := strings.TrimSpace(r.FormValue("target_database_name"))
	if target == "" {
		writeError(w, fmt.Errorf("%w: target_database_name is required", pgdb.ErrInvalidInput))
		return
	}

	opts := pgdb.RestoreUploadOptions{
		TargetDatabaseName: target,
		CreateDatabase:     formBool(r.FormValue("create_database"), true),
		DropExisting:       formBool(r.FormValue("drop_existing"), false),
	}

	wantStream := r.URL.Query().Get("stream") == "1" ||
		strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	if !wantStream {
		row, err := h.svc.RestoreFromUpload(r.Context(), id, opts, file)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, row)
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

	writeLog("info", fmt.Sprintf("Uploading %s (%d bytes)…", header.Filename, header.Size))
	_, err = h.svc.RestoreFromUploadWithLog(r.Context(), id, opts, file, writeLog)
	if err != nil {
		writeLog("error", err.Error())
		_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"failed\"}\n\n")
		flusher.Flush()
		return
	}
	_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"succeeded\"}\n\n")
	flusher.Flush()
}

func formBool(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
