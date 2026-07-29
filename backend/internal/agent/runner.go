package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/ebash/dock-pilot/backend/internal/metrics"
)

// Runner is the agent main loop: metrics, systemd checks, heartbeat, outbox flush.
type Runner struct {
	Config  Config
	Client  *Client
	Metrics *metrics.Collector
	Outbox  *Outbox
	Version string
	Logger  *slog.Logger

	prevServices map[string]string
}

func NewRunner(cfg Config, version string, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		Config:       cfg,
		Client:       NewClient(cfg.MasterURL, cfg.NodeToken),
		Metrics:      metrics.New(""),
		Outbox:       NewOutbox(DefaultOutboxDir),
		Version:      version,
		Logger:       logger,
		prevServices: map[string]string{},
	}
}

// Run blocks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.Outbox.Ensure(); err != nil {
		r.Logger.Warn("outbox init", "error", err)
	}
	_ = r.tick(ctx)

	for {
		interval := r.jitteredInterval()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			if err := r.tick(ctx); err != nil {
				r.Logger.Warn("tick failed", "error", err)
			}
		}
	}
}

func (r *Runner) jitteredInterval() time.Duration {
	base := time.Duration(r.Config.HeartbeatIntervalSeconds) * time.Second
	if base <= 0 {
		base = 30 * time.Second
	}
	// ±10% jitter
	j := float64(base) * 0.10
	delta := (rand.Float64()*2 - 1) * j
	d := time.Duration(float64(base) + delta)
	if d < 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func (r *Runner) tick(ctx context.Context) error {
	snap, err := r.Metrics.Collect()
	if err != nil {
		r.Logger.Warn("metrics collect", "error", err)
	}

	units := r.Config.MonitoredUnits
	services := CheckUnits(ctx, units)
	r.emitServiceTransitions(services)

	hb := HeartbeatRequest{
		NodeUID:      r.Config.NodeUID,
		Version:      r.Version,
		AgentVersion: r.Version,
		Metrics:      snap,
		Services:     services,
	}
	if err := r.Client.Heartbeat(ctx, hb); err != nil {
		r.Logger.Warn("heartbeat", "error", err)
	} else {
		r.Logger.Debug("heartbeat ok", "node_uid", r.Config.NodeUID)
	}

	r.flushOutbox(ctx)
	return nil
}

func (r *Runner) emitServiceTransitions(services []ServiceStatus) {
	now := time.Now().UTC()
	for _, s := range services {
		prev, ok := r.prevServices[s.UnitName]
		r.prevServices[s.UnitName] = s.State
		if !ok {
			continue
		}
		if prev == s.State {
			continue
		}
		var eventType, severity, title string
		switch {
		case s.State == "failed" || s.State == "not-found":
			eventType = "systemd.service_failed"
			severity = "warning"
			title = "systemd unit failed"
		case (prev == "failed" || prev == "not-found") && s.State == "active":
			eventType = "systemd.service_recovered"
			severity = "info"
			title = "systemd unit recovered"
		default:
			continue
		}
		payload, _ := json.Marshal(map[string]string{
			"unit_name": s.UnitName,
			"from":      prev,
			"to":        s.State,
		})
		hostname, _ := os.Hostname()
		ev := Event{
			EventID:      newEventID(),
			EventType:    eventType,
			Severity:     severity,
			ResourceType: "systemd_unit",
			ResourceID:   s.UnitName,
			Title:        title,
			Message:      hostname + ": " + s.UnitName + " " + prev + " → " + s.State,
			Payload:      payload,
			OccurredAt:   now,
			NodeUID:      r.Config.NodeUID,
		}
		if err := r.Outbox.Enqueue(ev); err != nil {
			r.Logger.Warn("outbox enqueue", "error", err)
		}
	}
}

func (r *Runner) flushOutbox(ctx context.Context) {
	items, err := r.Outbox.List(50)
	if err != nil || len(items) == 0 {
		return
	}
	events := make([]Event, 0, len(items))
	ids := make([]string, 0, len(items))
	for _, it := range items {
		var ev Event
		if err := json.Unmarshal(it.Payload, &ev); err != nil {
			r.Logger.Warn("outbox decode", "id", it.ID, "error", err)
			ids = append(ids, it.ID) // drop corrupt
			continue
		}
		if ev.NodeUID == "" {
			ev.NodeUID = r.Config.NodeUID
		}
		events = append(events, ev)
		ids = append(ids, it.ID)
	}
	if len(events) == 0 {
		_ = r.Outbox.Ack(ids...)
		return
	}
	if err := r.Client.PostEventsBatch(ctx, events); err != nil {
		r.Logger.Warn("events batch", "error", err, "count", len(events))
		return
	}
	_ = r.Outbox.Ack(ids...)
}
