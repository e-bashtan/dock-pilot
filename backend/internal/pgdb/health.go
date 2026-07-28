package pgdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/docker"
	"github.com/ebash/dock-pilot/backend/internal/healthcheck"
)

// HealthResult is a live probe of a managed Postgres instance.
type HealthResult struct {
	InstanceID uuid.UUID                 `json:"instance_id"`
	Name       string                    `json:"name"`
	Overall    string                    `json:"overall"` // healthy, degraded, unhealthy, unknown
	Message    string                    `json:"message"`
	Container  *healthcheck.ContainerInfo `json:"container,omitempty"`
	Ready      *bool                     `json:"ready,omitempty"`
	CheckedAt  time.Time                 `json:"checked_at"`
}

func (s *Service) Health(ctx context.Context, id uuid.UUID) (HealthResult, error) {
	inst, err := s.requireInstance(ctx, id)
	if err != nil {
		return HealthResult{}, err
	}
	return s.checkInstance(ctx, inst), nil
}

func (s *Service) HealthAll(ctx context.Context) ([]HealthResult, error) {
	rows, err := s.queries.ListPgInstances(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HealthResult, 0, len(rows))
	for _, inst := range rows {
		out = append(out, s.checkInstance(ctx, inst))
	}
	return out, nil
}

func (s *Service) checkInstance(ctx context.Context, inst db.PdbInstance) HealthResult {
	now := time.Now().UTC()
	res := HealthResult{
		InstanceID: inst.ID,
		Name:       inst.Name,
		Overall:    "unknown",
		Message:    "Not checked",
		CheckedAt:  now,
	}

	cname := s.containerName(inst)
	st, err := s.docker.InspectContainer(ctx, cname)
	if err != nil {
		res.Overall = "unknown"
		res.Message = "Docker inspect failed: " + err.Error()
		return res
	}

	res.Container = &healthcheck.ContainerInfo{
		Found:     st.Found,
		Running:   st.Running,
		State:     st.State,
		Health:    st.Health,
		Container: st.Container,
	}
	if res.Container.Container == "" && st.Found {
		res.Container.Container = cname
	}

	if !st.Found {
		if inst.Status == "draft" {
			res.Overall = "unhealthy"
			res.Message = "Postgres not deployed yet"
			return res
		}
		res.Overall = "unhealthy"
		res.Message = "Container not found — deploy Postgres"
		return res
	}

	if !docker.IsContainerRunning(st) {
		res.Overall = "unhealthy"
		res.Message = fmt.Sprintf("Container not running (state: %s)", st.State)
		return res
	}

	h := strings.ToLower(st.Health)
	if h == "unhealthy" {
		res.Overall = "unhealthy"
		res.Message = "Docker HEALTHCHECK reports unhealthy"
		return res
	}

	ready := s.waitReady(ctx, inst) == nil
	res.Ready = &ready
	if !ready {
		res.Overall = "degraded"
		res.Message = "Container running but pg_isready failed"
		return res
	}

	res.Overall = "healthy"
	res.Message = "Postgres is ready"
	return res
}
