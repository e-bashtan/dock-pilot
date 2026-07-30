package deployments

import (
	"time"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func toDeploymentResponse(d db.Deployment) DeploymentResponse {
	resp := DeploymentResponse{
		ID:        d.ID,
		SiteID:    d.SiteID,
		Status:    d.Status,
		Message:   d.Message,
		CreatedAt: d.CreatedAt,
	}
	if d.StartedAt.Valid {
		t := d.StartedAt.Time
		resp.StartedAt = &t
	}
	if d.FinishedAt.Valid {
		t := d.FinishedAt.Time
		resp.FinishedAt = &t
	}
	return resp
}

func timestamptzNow() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
