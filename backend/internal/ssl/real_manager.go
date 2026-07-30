package ssl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ebash/barn/backend/internal/hostexec"
	"github.com/ebash/barn/backend/internal/nginx"
)

type RealManager struct {
	logger *slog.Logger
	email  string
	host   *hostexec.Runner
}

type RealConfig struct {
	Email    string
	HostRoot string
}

func NewRealManager(cfg RealConfig, logger *slog.Logger) *RealManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &RealManager{
		logger: logger,
		email:  cfg.Email,
		host:   hostexec.New(cfg.HostRoot),
	}
}

func (m *RealManager) IssueCertificate(ctx context.Context, domains []string) error {
	names := uniqueDomains(domains)
	if len(names) == 0 {
		return fmt.Errorf("no domains for certificate")
	}

	if strings.TrimSpace(m.email) == "" {
		return fmt.Errorf("CERTBOT_EMAIL is not set — add it to .env before issuing SSL certificates")
	}

	if err := m.ensureAcmeWebroot(); err != nil {
		return fmt.Errorf("acme webroot: %w", err)
	}

	certbotArgs := certbotArgs(m.email, names)
	m.logger.InfoContext(ctx, "certbot issue", "domains", names, "webroot", nginx.AcmeWebroot)

	var out string
	var err error

	if m.host.UsesChroot() {
		out, err = m.issueOnHost(ctx, certbotArgs)
		if err != nil {
			m.logger.WarnContext(ctx, "certbot via nsenter failed, trying chroot", "error", err)
			out, err = m.issueInChroot(ctx, certbotArgs)
		}
	} else {
		out, err = m.host.RunHostCombined(ctx, "certbot", certbotArgs...)
	}

	if err != nil {
		return fmt.Errorf("certbot: %w", err)
	}
	m.logger.InfoContext(ctx, "certbot finished", "output", strings.TrimSpace(out))
	return nil
}

// issueOnHost runs certbot in the host mount/network namespace (requires pid: host + SYS_ADMIN).
func (m *RealManager) issueOnHost(ctx context.Context, certbotArgs []string) (string, error) {
	args := []string{"-t", "1", "-m", "-n", "-u", "-i", "-p", "--", "certbot"}
	args = append(args, certbotArgs...)
	return m.host.RunHostCombined(ctx, "nsenter", args...)
}

// issueInChroot runs certbot inside HOST_ROOT with a public resolver written inside the chroot.
func (m *RealManager) issueInChroot(ctx context.Context, certbotArgs []string) (string, error) {
	var script strings.Builder
	script.WriteString("set -e\n")
	script.WriteString("mkdir -p /var/www/certbot\n")
	script.WriteString("rm -f /etc/resolv.conf\n")
	script.WriteString("printf 'nameserver 8.8.8.8\\nnameserver 1.1.1.1\\n' > /etc/resolv.conf\n")
	script.WriteString("exec certbot")
	for _, arg := range certbotArgs {
		script.WriteByte(' ')
		script.WriteString(shellQuote(arg))
	}
	script.WriteByte('\n')

	var out string
	err := m.host.WithChrootDNS(ctx, func(ctx context.Context) error {
		var runErr error
		out, runErr = m.host.RunShellCombined(ctx, script.String())
		return runErr
	})
	return out, err
}

func (m *RealManager) ensureAcmeWebroot() error {
	return m.host.MkdirAll(m.host.ChrootPath(nginx.AcmeWebroot), 0o755)
}

func certbotArgs(email string, domains []string) []string {
	args := []string{
		"certonly",
		"--webroot", "-w", nginx.AcmeWebroot,
		"--non-interactive",
		"--agree-tos",
		"-m", email,
	}
	for _, d := range domains {
		args = append(args, "-d", d)
	}
	return args
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

var _ Manager = (*RealManager)(nil)

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
