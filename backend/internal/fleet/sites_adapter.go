package fleet

import (
	"context"

	sitesvc "github.com/ebash/dock-pilot/backend/internal/sites"
)

// SitesAppCounter adapts sites.Service to SiteHealthProvider.
type SitesAppCounter struct {
	Sites *sitesvc.Service
}

func (a SitesAppCounter) CountApps(ctx context.Context) (total, running, unhealthy int, err error) {
	if a.Sites == nil {
		return 0, 0, 0, nil
	}
	rows, err := a.Sites.HealthAll(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	total = len(rows)
	for _, h := range rows {
		switch h.Overall {
		case "healthy":
			running++
		case "degraded":
			running++
			unhealthy++
		case "unhealthy":
			unhealthy++
		}
	}
	return total, running, unhealthy, nil
}
