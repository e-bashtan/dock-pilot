package pgdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
	"github.com/ebash/barn/backend/internal/healthcheck"
)

// HealthResult is a live probe of a managed Postgres instance.
type HealthResult struct {
	InstanceID uuid.UUID                  `json:"instance_id"`
	Name       string                     `json:"name"`
	Overall    string                     `json:"overall"` // healthy, degraded, unhealthy, unknown
	Message    string                     `json:"message"`
	Container  *healthcheck.ContainerInfo `json:"container,omitempty"`
	Ready      *bool                      `json:"ready,omitempty"`
	CheckedAt  time.Time                  `json:"checked_at"`
	Databases  []DatabaseHealth           `json:"databases,omitempty"`
}

type DatabaseHealth struct {
	Name      string     `json:"name"`
	Overall   string     `json:"overall"`
	Message   string     `json:"message,omitempty"`
	LastDMLAt *time.Time `json:"last_dml_at,omitempty"`
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
	present := s.presentManagedContainers(ctx)
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

	s.logger.InfoContext(ctx, "managed postgres health",
		"instance_id", inst.ID.String(),
		"resolved_container", cname,
		"volume", volumeForManagedContainer(cname),
		"admin_user_setting", inst.AdminUser,
		"present_containers", present,
		"inspect_found", st.Found,
		"inspect_running", st.Running,
		"inspect_health", st.Health,
	)

	if !st.Found {
		if inst.Status == "draft" {
			res.Overall = "unhealthy"
			res.Message = "Postgres not deployed yet"
			return res
		}
		res.Overall = "unhealthy"
		res.Message = fmt.Sprintf("Container %s not found — present: %v", cname, present)
		return res
	}

	if !docker.IsContainerRunning(st) {
		res.Overall = "unhealthy"
		res.Message = fmt.Sprintf("Container not running (state: %s) · resolved=%s · present=%v", st.State, cname, present)
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
		res.Message = fmt.Sprintf("Container running but pg_isready failed · resolved=%s · present=%v", cname, present)
		return res
	}

	dbs := []string(nil)
	if creds, cerr := s.resolveExecCreds(ctx, inst); cerr == nil {
		var listErr error
		dbs, listErr = s.clusterDatabaseNames(ctx, creds)
		if listErr != nil {
			res.Overall = "degraded"
			res.Message = "Postgres is ready, but database check failed: " + truncateDiag(listErr.Error(), 300)
			return res
		}
		failed := make([]string, 0)
		for _, database := range dbs {
			dbHealth := DatabaseHealth{Name: database, Overall: "healthy"}
			stats, err := s.databaseDMLStats(ctx, creds, database)
			if err != nil {
				failed = append(failed, database)
				dbHealth.Overall = "unhealthy"
				dbHealth.Message = truncateDiag(err.Error(), 300)
				s.logger.WarnContext(ctx, "managed postgres database health failed",
					"instance_id", inst.ID.String(),
					"database", database,
					"error", truncateDiag(err.Error(), 300),
				)
			} else if activity, saveErr := s.queries.UpsertPdbDatabaseActivity(ctx, db.UpsertPdbDatabaseActivityParams{
				InstanceID: inst.ID, DatabaseName: database,
				Inserts: stats.inserts, Updates: stats.updates, Deletes: stats.deletes, CheckedAt: now,
			}); saveErr != nil {
				s.logger.WarnContext(ctx, "save postgres database activity failed", "database", database, "error", saveErr)
			} else if activity.LastDmlAt.Valid {
				last := activity.LastDmlAt.Time
				dbHealth.LastDMLAt = &last
			}
			res.Databases = append(res.Databases, dbHealth)
		}
		if len(failed) > 0 {
			res.Overall = "degraded"
			res.Message = fmt.Sprintf("Postgres is ready, but database check failed: %s", strings.Join(failed, ", "))
			return res
		}
		s.logger.InfoContext(ctx, "managed postgres health cluster databases",
			"instance_id", inst.ID.String(),
			"container", creds.container,
			"user", creds.user,
			"mode", creds.mode.String(),
			"databases", dbs,
		)
	} else {
		s.logger.WarnContext(ctx, "managed postgres health could not resolve creds",
			"instance_id", inst.ID.String(),
			"container", cname,
			"error", cerr.Error(),
		)
		res.Overall = "degraded"
		res.Message = "Postgres is ready, but database credentials could not be checked"
		return res
	}

	res.Overall = "healthy"
	if len(dbs) > 0 {
		res.Message = fmt.Sprintf("Postgres is ready · exec→%s · dbs=[%s] · present=%v",
			cname, strings.Join(dbs, ","), present)
	} else {
		res.Message = fmt.Sprintf("Postgres is ready · exec→%s · present=%v", cname, present)
	}
	return res
}

type databaseDMLCounters struct {
	inserts int64
	updates int64
	deletes int64
}

func (s *Service) databaseDMLStats(ctx context.Context, creds execCreds, database string) (databaseDMLCounters, error) {
	var stdout, stderr strings.Builder
	opts := s.execOpts(creds, []string{
		"psql",
		"-v", "ON_ERROR_STOP=1",
		"-U", creds.user,
		"-d", database,
		"-tA", "-F", "|",
		"-c", "SELECT COALESCE(sum(n_tup_ins),0), COALESCE(sum(n_tup_upd),0), COALESCE(sum(n_tup_del),0) FROM pg_stat_user_tables",
	})
	code, err := s.docker.Exec(ctx, opts, nil, &stdout, &stderr)
	if err != nil {
		return databaseDMLCounters{}, err
	}
	if code != 0 {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = fmt.Sprintf("psql exit %d", code)
		}
		return databaseDMLCounters{}, fmt.Errorf("%s", message)
	}
	parts := strings.Split(strings.TrimSpace(stdout.String()), "|")
	if len(parts) != 3 {
		return databaseDMLCounters{}, fmt.Errorf("unexpected statistics response")
	}
	values := make([]int64, 3)
	for i, raw := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return databaseDMLCounters{}, fmt.Errorf("parse database statistics: %w", err)
		}
		values[i] = value
	}
	return databaseDMLCounters{inserts: values[0], updates: values[1], deletes: values[2]}, nil
}
