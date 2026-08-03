package ssl

import (
	"context"
	"log/slog"
)

// Manager issues and renews TLS certificates.
type Manager interface {
	IssueCertificate(ctx context.Context, domains []string) error
	DeleteCertificate(ctx context.Context, primaryDomain string) error
}

// StubManager logs certbot operations.
type StubManager struct {
	logger *slog.Logger
}

func NewStubManager(logger *slog.Logger) *StubManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &StubManager{logger: logger}
}

func (s *StubManager) IssueCertificate(ctx context.Context, domains []string) error {
	s.logger.InfoContext(ctx, "stub certbot issue", "domains", domains)
	return nil
}

func (s *StubManager) DeleteCertificate(ctx context.Context, primaryDomain string) error {
	s.logger.InfoContext(ctx, "stub certbot delete", "domain", primaryDomain)
	return nil
}
