package servers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/metrics"
)

var allowedEventTypes = map[string]string{
	"node.offline":              "critical",
	"node.online":               "info",
	"container.unhealthy":       "warning",
	"container.recovered":       "info",
	"deployment.failed":         "critical",
	"backup.failed":             "critical",
	"disk.warning":              "warning",
	"disk.critical":             "critical",
	"systemd.service_failed":    "critical",
	"systemd.service_recovered": "info",
	"billing.expiring":          "warning",
}

func (s *Service) IngestHeartbeat(ctx context.Context, nodeID uuid.UUID, req HeartbeatRequest) error {
	now := time.Now().UTC()
	appsTotal, appsRunning, appsUnhealthy := 0, 0, 0
	if req.Apps != nil {
		appsTotal, appsRunning, appsUnhealthy = req.Apps.Total, req.Apps.Running, req.Apps.Unhealthy
	}
	payload := s.mergeNodeSnapshotPayload(ctx, nodeID, map[string]any{
		"metrics":      req.Metrics,
		"applications": req.Apps,
		"services":     req.Services,
		"host_ip":      req.HostIP,
		"billing":      req.Billing,
	})
	_, _ = s.q.InsertServersSnapshot(ctx, db.InsertServersSnapshotParams{
		NodeID:           nodeID,
		CollectedAt:      now,
		CpuPercent:       pgFloat8(req.Metrics.CPUPercent),
		MemoryUsedBytes:  pgInt8(int64(req.Metrics.MemoryUsed)),
		MemoryTotalBytes: pgInt8(int64(req.Metrics.MemoryTotal)),
		DiskUsedPercent:  pgFloat8(req.Metrics.DiskUsedPct),
		UptimeSeconds:    pgInt8(req.Metrics.UptimeSeconds),
		AppsTotal:        pgInt4(appsTotal),
		AppsRunning:      pgInt4(appsRunning),
		AppsUnhealthy:    pgInt4(appsUnhealthy),
		Payload:          payload,
	})
	node, err := s.q.GetServersNode(ctx, nodeID)
	if err != nil {
		return mapErr(err)
	}
	prev := node.Status
	_, err = s.q.UpdateServersNodeHeartbeat(ctx, db.UpdateServersNodeHeartbeatParams{
		ID:           nodeID,
		Status:       StatusOnline,
		LastSeenAt:   pgTimestamptz(now),
		Version:      req.Version,
		AgentVersion: req.AgentVersion,
	})
	if err != nil {
		return mapErr(err)
	}
	if prev == StatusOffline || prev == StatusWarning {
		_, _ = s.RecordEvent(ctx, nodeID, IngestEvent{
			EventID:    NewEventID(),
			EventType:  "node.online",
			Severity:   "info",
			Title:      "Node online",
			Message:    fmt.Sprintf("%s is online", node.Name),
			OccurredAt: now,
			NodeUID:    node.NodeUid.String(),
		})
	}
	return nil
}

func (s *Service) RecordEvent(ctx context.Context, nodeID uuid.UUID, ev IngestEvent) (bool, error) {
	if ev.EventID == uuid.Nil {
		ev.EventID = NewEventID()
	}
	if ev.NotifyOnly {
		s.notifyIncident(ctx, ev.Title, ev.Message)
		return true, nil
	}
	if _, ok := allowedEventTypes[ev.EventType]; !ok {
		return false, fmt.Errorf("%w: unknown event type", ErrInvalidInput)
	}
	if ev.Severity == "" {
		ev.Severity = allowedEventTypes[ev.EventType]
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}
	payload := ev.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	nodeUID := pgtype.UUID{}
	if ev.NodeUID != "" {
		if id, err := uuid.Parse(ev.NodeUID); err == nil {
			nodeUID = pgUUID(id)
		}
	}
	inserted, err := s.q.InsertServersEvent(ctx, db.InsertServersEventParams{
		EventID:      ev.EventID,
		NodeID:       pgUUID(nodeID),
		NodeUid:      nodeUID,
		EventType:    ev.EventType,
		Severity:     ev.Severity,
		ResourceType: ev.ResourceType,
		ResourceID:   ev.ResourceID,
		Title:        ev.Title,
		Message:      ev.Message,
		Payload:      payload,
		OccurredAt:   ev.OccurredAt,
	})
	if err != nil {
		if isNoRows(err) {
			return false, nil // duplicate event_id
		}
		return false, mapErr(err)
	}
	_ = inserted

	isRecovery := strings.HasSuffix(ev.EventType, ".online") ||
		strings.HasSuffix(ev.EventType, ".recovered") ||
		ev.EventType == "node.online" ||
		ev.EventType == "container.recovered" ||
		ev.EventType == "systemd.service_recovered"

	dedup := DedupKey(nodeID, recoveryBaseType(ev.EventType), ev.ResourceType, ev.ResourceID)
	if isRecovery {
		if open, err := s.q.GetOpenIncidentByDedup(ctx, dedup); err == nil {
			_, _ = s.q.ResolveServersIncident(ctx, db.ResolveServersIncidentParams{
				ID:          open.ID,
				LastEventID: pgUUID(ev.EventID),
			})
			s.notifyIncident(ctx, open.Title+" recovered", ev.Message)
		}
		return true, nil
	}

	if open, err := s.q.GetOpenIncidentByDedup(ctx, dedup); err == nil {
		_, _ = s.q.TouchServersIncident(ctx, db.TouchServersIncidentParams{
			ID:          open.ID,
			LastEventID: pgUUID(ev.EventID),
		})
		return true, nil
	}
	title := ev.Title
	if title == "" {
		title = ev.EventType
	}
	_, err = s.q.CreateServersIncident(ctx, db.CreateServersIncidentParams{
		NodeID:       pgUUID(nodeID),
		DedupKey:     dedup,
		EventType:    recoveryBaseType(ev.EventType),
		ResourceType: ev.ResourceType,
		ResourceID:   ev.ResourceID,
		Title:        title,
		LastEventID:  pgUUID(ev.EventID),
	})
	if err != nil {
		return true, nil // race on unique
	}
	s.notifyIncident(ctx, title, ev.Message)
	return true, nil
}

func recoveryBaseType(t string) string {
	switch t {
	case "node.online":
		return "node.offline"
	case "container.recovered":
		return "container.unhealthy"
	case "systemd.service_recovered":
		return "systemd.service_failed"
	default:
		return t
	}
}

func (s *Service) notifyIncident(ctx context.Context, title, message string) {
	if s.notify == nil {
		return
	}
	text := fmt.Sprintf("<b>Barn</b>\n%s\n%s", title, message)
	if err := s.notify.SendText(ctx, text); err != nil {
		s.logger.Warn("fleet telegram notify", "error", err)
	}
}

func (s *Service) ListEvents(ctx context.Context, limit, offset int32) ([]EventResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q.ListServersEvents(ctx, db.ListServersEventsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]EventResponse, 0, len(rows))
	for _, r := range rows {
		ev := EventResponse{
			ID:           r.ID,
			EventID:      r.EventID,
			EventType:    r.EventType,
			Severity:     r.Severity,
			ResourceType: r.ResourceType,
			ResourceID:   r.ResourceID,
			Title:        r.Title,
			Message:      r.Message,
			OccurredAt:   r.OccurredAt,
		}
		if r.NodeID.Valid {
			id := uuid.UUID(r.NodeID.Bytes)
			ev.NodeID = &id
		}
		if r.NodeUid.Valid {
			ev.NodeUID = uuid.UUID(r.NodeUid.Bytes).String()
		}
		out = append(out, ev)
	}
	return out, nil
}

func (s *Service) ListIncidents(ctx context.Context) ([]IncidentResponse, error) {
	rows, err := s.q.ListOpenServersIncidents(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]IncidentResponse, 0, len(rows))
	for _, r := range rows {
		inc := IncidentResponse{
			ID:          r.ID,
			EventType:   r.EventType,
			Title:       r.Title,
			Status:      r.Status,
			Count:       int(r.Count),
			FirstSeenAt: r.FirstSeenAt,
			LastSeenAt:  r.LastSeenAt,
		}
		if r.NodeID.Valid {
			id := uuid.UUID(r.NodeID.Bytes)
			inc.NodeID = &id
		}
		if r.ResolvedAt.Valid {
			t := r.ResolvedAt.Time
			inc.ResolvedAt = &t
		}
		out = append(out, inc)
	}
	return out, nil
}

func (s *Service) EnqueueLocalEvent(ctx context.Context, ev IngestEvent) error {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return err
	}
	if settings.Mode == ModeManagedNode && settings.NotificationMode == NotifyMaster {
		if ev.EventID == uuid.Nil {
			ev.EventID = NewEventID()
		}
		ev.NodeUID = settings.NodeUid.String()
		payload, _ := json.Marshal(ev)
		_, err := s.q.InsertServersOutbox(ctx, db.InsertServersOutboxParams{
			EventID: ev.EventID,
			Payload: payload,
		})
		return mapErr(err)
	}
	if settings.Mode == ModeMaster {
		local, err := s.q.GetLocalServersNode(ctx)
		if err != nil {
			return mapErr(err)
		}
		_, err = s.RecordEvent(ctx, local.ID, ev)
		return err
	}
	return nil
}

// OnLocalIncident implements notifications.ServersEventSink.
func (s *Service) OnLocalIncident(ctx context.Context, kind, resourceID, name, overall, message string) error {
	eventType := "container.unhealthy"
	severity := "warning"
	title := name + " unhealthy"
	if overall == "healthy" {
		eventType = "container.recovered"
		severity = "info"
		title = name + " recovered"
	}
	return s.EnqueueLocalEvent(ctx, IngestEvent{
		EventID:      NewEventID(),
		EventType:    eventType,
		Severity:     severity,
		ResourceType: kind,
		ResourceID:   resourceID,
		Title:        title,
		Message:      message,
		OccurredAt:   time.Now().UTC(),
	})
}

// OnLocalMessage forwards an arbitrary user notification through the master.
func (s *Service) OnLocalMessage(ctx context.Context, message string) error {
	return s.EnqueueLocalEvent(ctx, IngestEvent{
		EventID:    NewEventID(),
		Title:      "Notification",
		Message:    message,
		OccurredAt: time.Now().UTC(),
		NotifyOnly: true,
	})
}

func SnapshotFromMetrics(m metrics.Snapshot) MetricsDTO {
	return MetricsDTO{
		CPUPercent:       m.CPUPercent,
		MemoryUsedBytes:  m.MemoryUsed,
		MemoryTotalBytes: m.MemoryTotal,
		DiskUsedPercent:  m.DiskUsedPct,
		UptimeSeconds:    m.UptimeSeconds,
		Load1:            m.Load1,
		Load5:            m.Load5,
		Load15:           m.Load15,
	}
}
