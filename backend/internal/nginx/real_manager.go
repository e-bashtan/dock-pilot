package nginx

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ebash/barn/backend/internal/hostexec"
)

type RealManager struct {
	logger     *slog.Logger
	available  string
	enabled    string
	host       *hostexec.Runner
	configTmpl *template.Template
}

type RealConfig struct {
	SitesAvailable string
	SitesEnabled   string
	HostRoot       string
}

func NewRealManager(cfg RealConfig, logger *slog.Logger) (*RealManager, error) {
	tmpl, err := template.New("site").Parse(siteConfigTemplate)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RealManager{
		logger:     logger,
		available:  cfg.SitesAvailable,
		enabled:    cfg.SitesEnabled,
		host:       hostexec.New(cfg.HostRoot),
		configTmpl: tmpl,
	}, nil
}

func (m *RealManager) WriteConfig(ctx context.Context, siteKey string, cfg SiteConfig) error {
	allDomains := append([]string{cfg.PrimaryDomain}, cfg.Aliases...)
	allDomains = uniqueDomains(allDomains)
	if len(allDomains) == 0 {
		return fmt.Errorf("no domains configured")
	}

	if err := m.ensureGlobalTuning(ctx, allDomains); err != nil {
		return err
	}

	useSSL := cfg.SSLEnabled && m.certificateExists(cfg.PrimaryDomain)

	var buf bytes.Buffer
	if err := m.configTmpl.Execute(&buf, map[string]any{
		"Primary":      cfg.PrimaryDomain,
		"Domains":      strings.Join(allDomains, " "),
		"Upstream":     fmt.Sprintf("http://%s:%d", cfg.UpstreamHost, cfg.UpstreamPort),
		"SSL":          useSSL,
		"ForceHTTPS":   cfg.ForceHTTPS && useSSL,
		"AcmeLocation": acmeChallengeLocation,
	}); err != nil {
		return fmt.Errorf("render nginx config: %w", err)
	}

	safeName := safeFilename(siteKey)
	availablePath := filepath.Join(m.available, "barn-"+safeName+".conf")
	enabledPath := filepath.Join(m.enabled, "barn-"+safeName+".conf")

	if err := m.host.MkdirAll(m.available, 0o755); err != nil {
		return fmt.Errorf("mkdir sites-available: %w", err)
	}
	if err := m.host.MkdirAll(m.enabled, 0o755); err != nil {
		return fmt.Errorf("mkdir sites-enabled: %w", err)
	}
	if err := m.host.MkdirAll(m.host.ChrootPath(AcmeWebroot), 0o755); err != nil {
		return fmt.Errorf("mkdir acme webroot: %w", err)
	}
	if err := m.cleanupConflictingConfigs(cfg.PrimaryDomain, availablePath); err != nil {
		return fmt.Errorf("cleanup conflicting nginx configs: %w", err)
	}
	if err := m.host.WriteFile(availablePath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write nginx config: %w", err)
	}
	linkTarget, err := filepath.Rel(filepath.Dir(enabledPath), availablePath)
	if err != nil {
		return fmt.Errorf("symlink target: %w", err)
	}
	if err := m.host.Symlink(linkTarget, enabledPath); err != nil {
		return fmt.Errorf("enable site: %w", err)
	}

	m.logger.InfoContext(ctx, "nginx config written",
		"site_key", siteKey,
		"path", availablePath,
		"domains", allDomains,
		"ssl", useSSL,
	)
	return nil
}

// cleanupConflictingConfigs removes other vhosts (manual or legacy barn/dockpilot) for the same domain.
func (m *RealManager) cleanupConflictingConfigs(primaryDomain, keepAvailablePath string) error {
	primaryDomain = strings.TrimSpace(primaryDomain)
	if primaryDomain == "" {
		return nil
	}

	entries, err := os.ReadDir(m.available)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	keepBase := filepath.Base(keepAvailablePath)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == keepBase || !strings.HasSuffix(name, ".conf") {
			continue
		}
		availablePath := filepath.Join(m.available, name)
		b, err := os.ReadFile(availablePath)
		if err != nil {
			continue
		}
		if !configDeclaresDomain(string(b), primaryDomain) {
			continue
		}

		enabledPath := filepath.Join(m.enabled, name)
		_ = m.host.Remove(enabledPath)
		_ = m.host.Remove(availablePath)
		m.logger.Info("removed conflicting nginx config",
			"domain", primaryDomain,
			"file", name,
		)
	}
	return nil
}

func (m *RealManager) certificateExists(primaryDomain string) bool {
	primaryDomain = strings.TrimSpace(primaryDomain)
	if primaryDomain == "" {
		return false
	}
	certPath := m.host.ChrootPath(filepath.Join("/etc/letsencrypt/live", primaryDomain, "fullchain.pem"))
	_, err := os.Stat(certPath)
	return err == nil
}

func (m *RealManager) TestConfig(ctx context.Context) error {
	m.pruneDuplicateHashTuning(ctx)
	m.logger.InfoContext(ctx, "nginx config test")
	out, err := m.host.RunCombined(ctx, "nginx", "-t")
	if err == nil {
		return nil
	}

	combined := out + err.Error()
	if strings.Contains(combined, "server_names_hash_bucket_size") {
		m.logger.WarnContext(ctx, "nginx -t failed due to server_names_hash; re-applying hash tuning", "error", err)
		// Use a conservative large bucket so long domains pass without knowing all vhosts.
		if tuneErr := m.host.RunShell(ctx, applyNginxHashTuningScript(128, 2048)); tuneErr != nil {
			return fmt.Errorf("nginx -t: %w (hash retune failed: %v)", err, tuneErr)
		}
		if _, retryErr := m.host.RunCombined(ctx, "nginx", "-t"); retryErr == nil {
			return nil
		} else {
			return fmt.Errorf("nginx -t: %w", retryErr)
		}
	}
	return fmt.Errorf("nginx -t: %w", err)
}

func (m *RealManager) Reload(ctx context.Context) error {
	m.logger.InfoContext(ctx, "nginx reload")
	if m.host.UsesChroot() {
		if err := m.reloadOnHost(ctx); err == nil {
			m.logger.InfoContext(ctx, "nginx reloaded", "method", "nsenter+systemctl")
			return nil
		} else {
			m.logger.WarnContext(ctx, "nginx reload via nsenter failed, trying chroot", "error", err)
		}
	}
	return m.reloadChroot(ctx)
}

// reloadOnHost runs systemctl in the host namespace (same approach as certbot).
func (m *RealManager) reloadOnHost(ctx context.Context) error {
	const script = `export DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket
for f in /run/nginx.pid /var/run/nginx.pid; do
  if [ -s "$f" ] && ! kill -0 "$(tr -d ' \n' < "$f")" 2>/dev/null; then
    rm -f "$f"
  fi
done
if systemctl is-active --quiet nginx 2>/dev/null; then
  systemctl reload nginx
else
  systemctl start nginx
fi`
	_, err := m.host.RunHostCombined(ctx, "nsenter", "-t", "1", "-m", "-n", "-u", "-i", "-p", "--", "sh", "-c", script)
	return err
}

func (m *RealManager) reloadChroot(ctx context.Context) error {
	const dbus = "DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket"
	const cleanPID = `for f in /run/nginx.pid /var/run/nginx.pid; do
  if [ -s "$f" ] && ! kill -0 "$(tr -d ' \n' < "$f")" 2>/dev/null; then rm -f "$f"; fi
done`
	attempts := []struct {
		name string
		cmd  string
	}{
		{"systemctl reload", dbus + "; " + cleanPID + "; systemctl reload nginx"},
		{"systemctl try-reload", dbus + "; " + cleanPID + "; systemctl try-reload nginx"},
		{"nginx -s reload", cleanPID + "; nginx -s reload"},
	}
	var lastErr error
	for _, a := range attempts {
		if err := m.host.RunShell(ctx, a.cmd); err == nil {
			m.logger.InfoContext(ctx, "nginx reloaded", "method", a.name)
			return nil
		} else {
			m.logger.WarnContext(ctx, "nginx reload attempt failed", "method", a.name, "error", err)
			lastErr = err
		}
	}
	return fmt.Errorf("nginx reload failed: %w", lastErr)
}

var _ Manager = (*RealManager)(nil)

// AcmeWebroot is where certbot --webroot writes challenge files (must match nginx location).
const AcmeWebroot = "/var/www/certbot"

const acmeChallengeLocation = `    location ^~ /.well-known/acme-challenge/ {
        root /var/www/certbot;
        default_type "text/plain";
        try_files $uri =404;
    }`

const siteConfigTemplate = `server {
    listen 80;
    server_name {{ .Domains }};

    {{ .AcmeLocation }}
{{ if .ForceHTTPS }}
    location / {
        return 301 https://$host$request_uri;
    }
{{ else }}
    location / {
        proxy_pass {{ .Upstream }};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
{{ end }}
}
{{ if .SSL }}
server {
    listen 443 ssl;
    server_name {{ .Domains }};

    ssl_certificate /etc/letsencrypt/live/{{ .Primary }}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/{{ .Primary }}/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    location / {
        proxy_pass {{ .Upstream }};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
{{ end }}`

func uniqueDomains(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range in {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func safeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, s)
	if s == "" {
		return "site"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}
