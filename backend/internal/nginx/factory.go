package nginx

import (
	"log/slog"

	"github.com/ebash/barn/backend/internal/config"
)

func NewFromConfig(cfg config.DeployConfig, logger *slog.Logger) (Manager, error) {
	if cfg.Mode == "stub" {
		if logger == nil {
			logger = slog.Default()
		}
		return NewStubManager(logger), nil
	}
	return NewRealManager(RealConfig{
		SitesAvailable: cfg.NginxSitesAvailable,
		SitesEnabled:   cfg.NginxSitesEnabled,
		HostRoot:       cfg.HostRoot,
	}, logger)
}
