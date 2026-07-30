package sites

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
)

// ContainerActionResponse is returned after start/stop/restart.
type ContainerActionResponse struct {
	Action    string                 `json:"action"`
	Container docker.ContainerStatus `json:"container"`
}

func (s *Service) StopContainer(ctx context.Context, id uuid.UUID) (ContainerActionResponse, error) {
	site, err := s.queries.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ContainerActionResponse{}, ErrNotFound
		}
		return ContainerActionResponse{}, fmt.Errorf("get site: %w", err)
	}

	names := containerNamesForSite(site)
	if err := s.docker.Stop(ctx, names...); err != nil {
		return ContainerActionResponse{}, fmt.Errorf("stop container: %w", err)
	}

	_, _ = s.queries.UpdateSiteStatus(ctx, db.UpdateSiteStatusParams{
		ID:     id,
		Status: "stopped",
	})

	st, err := inspectSiteContainer(ctx, s.docker, site)
	if err != nil {
		return ContainerActionResponse{}, err
	}
	return ContainerActionResponse{Action: "stop", Container: st}, nil
}

func (s *Service) StartContainer(ctx context.Context, id uuid.UUID) (ContainerActionResponse, error) {
	site, err := s.queries.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ContainerActionResponse{}, ErrNotFound
		}
		return ContainerActionResponse{}, fmt.Errorf("get site: %w", err)
	}

	if site.Status == "draft" {
		return ContainerActionResponse{}, fmt.Errorf("%w: deploy the site before starting the container", ErrInvalidInput)
	}

	env, err := s.loadContainerEnv(ctx, id)
	if err != nil {
		return ContainerActionResponse{}, err
	}

	if err := RunSiteContainer(ctx, s.docker, site, env); err != nil {
		return ContainerActionResponse{}, fmt.Errorf("start container: %w", err)
	}

	_, _ = s.queries.UpdateSiteStatus(ctx, db.UpdateSiteStatusParams{
		ID:     id,
		Status: "active",
	})

	st, err := inspectSiteContainer(ctx, s.docker, site)
	if err != nil {
		return ContainerActionResponse{}, err
	}
	return ContainerActionResponse{Action: "start", Container: st}, nil
}

func (s *Service) RestartContainer(ctx context.Context, id uuid.UUID) (ContainerActionResponse, error) {
	site, err := s.queries.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ContainerActionResponse{}, ErrNotFound
		}
		return ContainerActionResponse{}, fmt.Errorf("get site: %w", err)
	}

	if site.Status == "draft" {
		return ContainerActionResponse{}, fmt.Errorf("%w: deploy the site before restarting the container", ErrInvalidInput)
	}

	names := containerNamesForSite(site)
	if err := s.docker.Stop(ctx, names...); err != nil {
		return ContainerActionResponse{}, fmt.Errorf("stop container: %w", err)
	}

	env, err := s.loadContainerEnv(ctx, id)
	if err != nil {
		return ContainerActionResponse{}, err
	}

	if err := RunSiteContainer(ctx, s.docker, site, env); err != nil {
		return ContainerActionResponse{}, fmt.Errorf("start container: %w", err)
	}

	_, _ = s.queries.UpdateSiteStatus(ctx, db.UpdateSiteStatusParams{
		ID:     id,
		Status: "active",
	})

	st, err := inspectSiteContainer(ctx, s.docker, site)
	if err != nil {
		return ContainerActionResponse{}, err
	}
	return ContainerActionResponse{Action: "restart", Container: st}, nil
}
