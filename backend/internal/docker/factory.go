package docker

import (
	"context"
	"log/slog"

	"github.com/ebash/barn/backend/internal/config"
)

func NewFromConfig(cfg config.DeployConfig, logger *slog.Logger) (Client, error) {
	if cfg.Mode == "stub" {
		if logger == nil {
			logger = slog.Default()
		}
		return NewStubClient(logger), nil
	}

	client, err := NewRealClient(RealConfig{
		Host:      cfg.DockerHost,
		PortStart: cfg.PortRangeStart,
		PortEnd:   cfg.PortRangeEnd,
	}, logger)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
