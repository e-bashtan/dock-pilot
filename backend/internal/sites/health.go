package sites

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebash/barn/backend/internal/healthcheck"
)

func (s *Service) Health(ctx context.Context, id uuid.UUID) (healthcheck.Result, error) {
	if s.health == nil {
		return healthcheck.Result{}, fmt.Errorf("health check not configured")
	}
	site, err := s.queries.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return healthcheck.Result{}, ErrNotFound
		}
		return healthcheck.Result{}, fmt.Errorf("get site: %w", err)
	}
	return s.health.Check(ctx, site), nil
}

func (s *Service) HealthAll(ctx context.Context) ([]healthcheck.Result, error) {
	if s.health == nil {
		return nil, fmt.Errorf("health check not configured")
	}
	rows, err := s.queries.ListSites(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	out := make([]healthcheck.Result, 0, len(rows))
	for _, site := range rows {
		out = append(out, s.health.Check(ctx, site))
	}
	return out, nil
}
