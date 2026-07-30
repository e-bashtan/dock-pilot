package deployments

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebash/barn/backend/internal/db"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
)

type Service struct {
	queries *db.Queries
	worker  *Worker
}

func NewService(queries *db.Queries, worker *Worker) *Service {
	return &Service{queries: queries, worker: worker}
}

func (s *Service) StartDeploy(ctx context.Context, siteID uuid.UUID) (DeploymentResponse, error) {
	site, err := s.queries.GetSite(ctx, siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeploymentResponse{}, sitesvc.ErrNotFound
		}
		return DeploymentResponse{}, fmt.Errorf("get site: %w", err)
	}

	now := time.Now().UTC()
	dep, err := s.queries.CreateDeployment(ctx, db.CreateDeploymentParams{
		SiteID:    siteID,
		Status:    "pending",
		Message:   "Deployment queued",
		StartedAt: timestamptz(now),
	})
	if err != nil {
		return DeploymentResponse{}, fmt.Errorf("create deployment: %w", err)
	}

	s.worker.Enqueue(site, dep)

	return toDeploymentResponse(dep), nil
}

func (s *Service) ListBySite(ctx context.Context, siteID uuid.UUID) ([]DeploymentResponse, error) {
	if _, err := s.queries.GetSite(ctx, siteID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sitesvc.ErrNotFound
		}
		return nil, fmt.Errorf("get site: %w", err)
	}

	rows, err := s.queries.ListDeploymentsBySite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	out := make([]DeploymentResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDeploymentResponse(row))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (DeploymentResponse, error) {
	dep, err := s.queries.GetDeployment(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeploymentResponse{}, ErrNotFound
		}
		return DeploymentResponse{}, fmt.Errorf("get deployment: %w", err)
	}
	return toDeploymentResponse(dep), nil
}

func (s *Service) StreamLogs(ctx context.Context, deploymentID uuid.UUID, w io.Writer, flusher interface{ Flush() }) error {
	if _, err := s.queries.GetDeployment(ctx, deploymentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get deployment: %w", err)
	}

	writeEvent := func(id int64, level, message string, createdAt time.Time) error {
		_, err := fmt.Fprintf(w, "id: %d\nevent: log\ndata: {\"id\":%d,\"level\":%q,\"message\":%q,\"created_at\":%q}\n\n",
			id, id, level, message, createdAt.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	logs, err := s.queries.ListDeploymentLogs(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("list logs: %w", err)
	}

	var lastID int64
	for _, log := range logs {
		if err := writeEvent(log.ID, log.Level, log.Message, log.CreatedAt); err != nil {
			return err
		}
		lastID = log.ID
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			dep, err := s.queries.GetDeployment(ctx, deploymentID)
			if err != nil {
				return fmt.Errorf("poll deployment: %w", err)
			}

			newLogs, err := s.queries.ListDeploymentLogsAfter(ctx, db.ListDeploymentLogsAfterParams{
				DeploymentID: deploymentID,
				ID:           lastID,
			})
			if err != nil {
				return fmt.Errorf("poll logs: %w", err)
			}

			for _, log := range newLogs {
				if err := writeEvent(log.ID, log.Level, log.Message, log.CreatedAt); err != nil {
					return err
				}
				lastID = log.ID
			}

			if isTerminal(dep.Status) {
				_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":%q}\n\n", dep.Status)
				if flusher != nil {
					flusher.Flush()
				}
				return nil
			}
		}
	}
}

func isTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}
