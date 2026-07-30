package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ebash/dock-pilot/backend/internal/db"
)

type PollingWorker struct {
	svc    *Service
	logger interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
	}
}

func NewPollingWorker(svc *Service) *PollingWorker {
	return &PollingWorker{svc: svc, logger: svc.logger}
}

func (w *PollingWorker) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *PollingWorker) tick(ctx context.Context) {
	mode, err := w.svc.Mode(ctx)
	if err != nil || mode != ModeMaster {
		return
	}
	nodes, err := w.svc.q.ListDockpilotNodes(ctx)
	if err != nil {
		return
	}
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, n := range nodes {
		n := n
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			w.pollNode(ctx, n)
		}()
	}
	wg.Wait()
	w.recomputeStatuses(ctx)
}

func (w *PollingWorker) pollNode(ctx context.Context, node db.FleetNode) {
	cred, err := w.svc.q.GetOutboundCredential(ctx, node.ID)
	if err != nil || len(cred.EncryptedToken) == 0 {
		return
	}
	token, err := w.svc.cipher.Decrypt(cred.EncryptedToken)
	if err != nil {
		return
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pctx, http.MethodGet, stringsTrimRight(node.BaseUrl)+"/api/fleet/node/status", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != 200 {
		return
	}
	var status NodeStatusPayload
	if json.Unmarshal(body, &status) != nil {
		return
	}
	now := time.Now().UTC()
	payload := w.svc.mergeNodeSnapshotPayload(ctx, node.ID, map[string]any{
		"metrics":      status.Metrics,
		"applications": status.Apps,
		"host_ip":      status.HostIP,
		"billing":      status.Billing,
	})
	_, _ = w.svc.q.InsertFleetSnapshot(ctx, db.InsertFleetSnapshotParams{
		NodeID:           node.ID,
		CollectedAt:      now,
		CpuPercent:       pgFloat8(status.Metrics.CPUPercent),
		MemoryUsedBytes:  pgInt8(int64(status.Metrics.MemoryUsed)),
		MemoryTotalBytes: pgInt8(int64(status.Metrics.MemoryTotal)),
		DiskUsedPercent:  pgFloat8(status.Metrics.DiskUsedPct),
		UptimeSeconds:    pgInt8(status.Metrics.UptimeSeconds),
		AppsTotal:        pgInt4(status.Apps.Total),
		AppsRunning:      pgInt4(status.Apps.Running),
		AppsUnhealthy:    pgInt4(status.Apps.Unhealthy),
		Payload:          payload,
	})
	caps, _ := json.Marshal(DockpilotCapabilities())
	prev := node.Status
	_, _ = w.svc.q.UpdateFleetNodePoll(ctx, db.UpdateFleetNodePollParams{
		ID:           node.ID,
		Status:       StatusOnline,
		LastSeenAt:   pgTimestamptz(now),
		Version:      status.Version,
		Capabilities: caps,
	})
	if prev == StatusOffline {
		_, _ = w.svc.RecordEvent(ctx, node.ID, IngestEvent{
			EventID: NewEventID(), EventType: "node.online", Severity: "info",
			Title: "Node online", Message: node.Name + " is online", OccurredAt: now,
			NodeUID: node.NodeUid.String(),
		})
	}
}

func (w *PollingWorker) recomputeStatuses(ctx context.Context) {
	nodes, err := w.svc.q.ListFleetNodes(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, n := range nodes {
		if n.ConnectionType == ConnLocal {
			continue
		}
		var last *time.Time
		if n.LastSeenAt.Valid {
			t := n.LastSeenAt.Time
			last = &t
		}
		st := ComputeStatus(last, now)
		if st != n.Status {
			_ = w.svc.q.UpdateFleetNodeStatus(ctx, db.UpdateFleetNodeStatusParams{ID: n.ID, Status: st})
			if st == StatusOffline && n.Status != StatusOffline {
				_, _ = w.svc.RecordEvent(ctx, n.ID, IngestEvent{
					EventID: NewEventID(), EventType: "node.offline", Severity: "critical",
					Title: "Node offline", Message: n.Name + " is offline", OccurredAt: now,
					NodeUID: n.NodeUid.String(),
				})
			}
		}
	}
}

type OutboxWorker struct {
	svc *Service
}

func NewOutboxWorker(svc *Service) *OutboxWorker {
	return &OutboxWorker{svc: svc}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *OutboxWorker) tick(ctx context.Context) {
	settings, err := w.svc.ensureSettings(ctx)
	if err != nil || settings.Mode != ModeManagedNode || len(settings.EncryptedMasterToken) == 0 {
		return
	}
	token, err := w.svc.cipher.Decrypt(settings.EncryptedMasterToken)
	if err != nil {
		return
	}
	rows, err := w.svc.q.ListPendingOutbox(ctx, 20)
	if err != nil {
		return
	}
	for _, row := range rows {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, stringsTrimRight(settings.MasterUrl)+"/api/fleet/ingest/events", bytes.NewReader(row.Payload))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode >= 300 {
			if res != nil {
				res.Body.Close()
			}
			delay := outboxBackoff(int(row.Attempts + 1))
			_ = w.svc.q.BumpOutboxAttempt(ctx, db.BumpOutboxAttemptParams{
				ID:            row.ID,
				NextAttemptAt: time.Now().UTC().Add(delay),
				LastError:     "delivery failed",
			})
			continue
		}
		res.Body.Close()
		_ = w.svc.q.MarkOutboxDelivered(ctx, row.ID)
	}
	_ = w.svc.q.DeleteOldDeliveredOutbox(ctx, pgTimestamptz(time.Now().UTC().Add(-7*24*time.Hour)))
}

func outboxBackoff(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 10 * time.Second
	case attempt == 2:
		return 30 * time.Second
	case attempt == 3:
		return time.Minute
	case attempt == 4:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func stringsTrimRight(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

type HeartbeatWorker struct {
	svc *Service
}

func NewHeartbeatWorker(svc *Service) *HeartbeatWorker {
	return &HeartbeatWorker{svc: svc}
}

func (w *HeartbeatWorker) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *HeartbeatWorker) tick(ctx context.Context) {
	settings, err := w.svc.ensureSettings(ctx)
	if err != nil || settings.Mode != ModeManagedNode || len(settings.EncryptedMasterToken) == 0 {
		return
	}
	token, err := w.svc.cipher.Decrypt(settings.EncryptedMasterToken)
	if err != nil {
		return
	}
	status, err := w.svc.LocalNodeStatus(ctx)
	if err != nil {
		return
	}
	body, _ := json.Marshal(HeartbeatRequest{
		NodeUID: status.NodeUID,
		Version: status.Version,
		Metrics: status.Metrics,
		Apps:    &status.Apps,
		HostIP:  status.HostIP,
		Billing: status.Billing,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stringsTrimRight(settings.MasterUrl)+"/api/fleet/ingest/heartbeat", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	res.Body.Close()
}

type RetentionWorker struct {
	svc *Service
}

func NewRetentionWorker(svc *Service) *RetentionWorker {
	return &RetentionWorker{svc: svc}
}

func (w *RetentionWorker) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = w.svc.q.DeleteOldFleetSnapshots(ctx, time.Now().UTC().Add(-30*24*time.Hour))
			}
		}
	}()
}
