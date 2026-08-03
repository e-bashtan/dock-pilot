package nginx

import (
	"context"
	"log/slog"
)

// SiteConfig holds reverse-proxy settings for a site.
type SiteConfig struct {
	PrimaryDomain string
	Aliases       []string
	UpstreamHost  string
	UpstreamPort  int
	SSLEnabled    bool // use HTTPS vhost when certificate files exist on host
	ForceHTTPS    bool // HTTP → HTTPS redirect when SSL is active
}

// Manager generates and applies nginx configuration.
type Manager interface {
	WriteConfig(ctx context.Context, siteKey string, cfg SiteConfig) error
	RemoveConfig(ctx context.Context, siteKey string) error
	TestConfig(ctx context.Context) error
	Reload(ctx context.Context) error
	// EnsureHostDefaults applies http-level settings needed by the panel (upload limits, etc.).
	EnsureHostDefaults(ctx context.Context) error
}

// StubManager logs nginx operations.
type StubManager struct {
	logger *slog.Logger
}

func NewStubManager(logger *slog.Logger) *StubManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &StubManager{logger: logger}
}

func (s *StubManager) WriteConfig(ctx context.Context, siteKey string, cfg SiteConfig) error {
	s.logger.InfoContext(ctx, "stub nginx write config",
		"site_key", siteKey,
		"primary", cfg.PrimaryDomain,
		"aliases", cfg.Aliases,
		"upstream", cfg.UpstreamHost,
		"port", cfg.UpstreamPort,
		"ssl_enabled", cfg.SSLEnabled,
		"force_https", cfg.ForceHTTPS,
	)
	return nil
}

func (s *StubManager) RemoveConfig(ctx context.Context, siteKey string) error {
	s.logger.InfoContext(ctx, "stub nginx remove config", "site_key", siteKey)
	return nil
}

func (s *StubManager) EnsureHostDefaults(ctx context.Context) error {
	s.logger.InfoContext(ctx, "stub nginx ensure host defaults")
	return nil
}

func (s *StubManager) TestConfig(ctx context.Context) error {
	s.logger.InfoContext(ctx, "stub nginx config test")
	return nil
}

func (s *StubManager) Reload(ctx context.Context) error {
	s.logger.InfoContext(ctx, "stub nginx reload")
	return nil
}
