package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ebash/barn/backend/internal/hostexec"
)

const (
	tunnelUnitName   = "telegram-socks-tunnel.service"
	tunnelKeyPath    = "/root/.ssh/telegram_foreign_tunnel"
	tunnelConfigPath = "/etc/barn/telegram-socks-tunnel.json"
)

var (
	tunnelHostPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
	tunnelUserPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]*[$]?$`)
)

type TunnelConfig struct {
	Host      string `json:"host"`
	SSHPort   int    `json:"ssh_port"`
	SSHUser   string `json:"ssh_user"`
	LocalPort int    `json:"local_port"`
}

type TunnelStatus struct {
	Configured  bool         `json:"configured"`
	KeyCreated  bool         `json:"key_created"`
	PublicKey   string       `json:"public_key,omitempty"`
	Config      TunnelConfig `json:"config"`
	Service     string       `json:"service"`
	SOCKSReady  bool         `json:"socks_ready"`
	InstallHint string       `json:"install_hint,omitempty"`
	Logs        string       `json:"logs,omitempty"`
}

type TunnelManager struct {
	host *hostexec.Runner
}

func NewTunnelManager(hostRoot string) *TunnelManager {
	return &TunnelManager{host: hostexec.New(hostRoot)}
}

func (m *TunnelManager) Status(ctx context.Context) (TunnelStatus, error) {
	cfg, configured, err := m.readConfig()
	if err != nil {
		return TunnelStatus{}, err
	}
	pub, _ := os.ReadFile(m.host.ChrootPath(tunnelKeyPath + ".pub"))
	status := TunnelStatus{
		Configured: configured,
		KeyCreated: len(pub) > 0,
		PublicKey:  strings.TrimSpace(string(pub)),
		Config:     cfg,
		Service:    "not-installed",
	}
	if status.PublicKey != "" {
		status.InstallHint = "mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '" + status.PublicKey + "' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
	}
	if configured {
		out, _ := m.systemctl(ctx, "is-active", tunnelUnitName)
		state := strings.TrimSpace(out)
		if state != "" {
			status.Service = state
		}
		status.SOCKSReady = socks5Ready(cfg.LocalPort, 700*time.Millisecond)
	}
	return status, nil
}

func (m *TunnelManager) GenerateKey(ctx context.Context, cfg TunnelConfig) (TunnelStatus, error) {
	cfg = normalizeTunnelConfig(cfg)
	if err := validateTunnelConfig(cfg); err != nil {
		return TunnelStatus{}, err
	}
	keyHostPath := m.host.ChrootPath(tunnelKeyPath)
	if err := os.MkdirAll(filepath.Dir(keyHostPath), 0o700); err != nil {
		return TunnelStatus{}, fmt.Errorf("create SSH directory: %w", err)
	}
	if _, err := os.Stat(keyHostPath); errors.Is(err, os.ErrNotExist) {
		if _, err := m.host.RunCombined(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-f", tunnelKeyPath, "-N", "", "-C", "telegram-socks-tunnel"); err != nil {
			return TunnelStatus{}, fmt.Errorf("generate SSH key: %w", err)
		}
	}
	if err := m.writeConfig(cfg); err != nil {
		return TunnelStatus{}, err
	}
	return m.Status(ctx)
}

func (m *TunnelManager) TestSSH(ctx context.Context) error {
	cfg, ok, err := m.readConfig()
	if err != nil || !ok {
		return fmt.Errorf("%w: tunnel is not configured", ErrInvalidInput)
	}
	args := []string{
		"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=accept-new",
		"-o", "ExitOnForwardFailure=yes", "-i", tunnelKeyPath, "-p", strconv.Itoa(cfg.SSHPort),
		cfg.SSHUser + "@" + cfg.Host, "true",
	}
	if _, err := m.host.RunCombined(ctx, "ssh", args...); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	return nil
}

func (m *TunnelManager) Start(ctx context.Context) (TunnelStatus, error) {
	cfg, ok, err := m.readConfig()
	if err != nil || !ok {
		return TunnelStatus{}, fmt.Errorf("%w: tunnel is not configured", ErrInvalidInput)
	}
	if err := m.TestSSH(ctx); err != nil {
		return TunnelStatus{}, err
	}
	unitPath := m.host.ChrootPath("/etc/systemd/system/" + tunnelUnitName)
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return TunnelStatus{}, err
	}
	if err := os.WriteFile(unitPath, []byte(renderTunnelUnit(cfg)), 0o644); err != nil {
		return TunnelStatus{}, fmt.Errorf("write systemd unit: %w", err)
	}
	if _, err := m.systemctl(ctx, "daemon-reload"); err != nil {
		return TunnelStatus{}, err
	}
	if _, err := m.systemctl(ctx, "enable", "--now", tunnelUnitName); err != nil {
		return TunnelStatus{}, err
	}
	if !waitSOCKS5(cfg.LocalPort, 5*time.Second) {
		return TunnelStatus{}, fmt.Errorf("Telegram SOCKS5 tunnel did not become ready; check the service log")
	}
	return m.Status(ctx)
}

func (m *TunnelManager) Stop(ctx context.Context) (TunnelStatus, error) {
	if _, err := m.systemctl(ctx, "stop", tunnelUnitName); err != nil {
		return TunnelStatus{}, err
	}
	return m.Status(ctx)
}

func (m *TunnelManager) Restart(ctx context.Context) (TunnelStatus, error) {
	if _, err := m.systemctl(ctx, "restart", tunnelUnitName); err != nil {
		return TunnelStatus{}, err
	}
	cfg, ok, err := m.readConfig()
	if err != nil || !ok {
		return TunnelStatus{}, fmt.Errorf("%w: tunnel is not configured", ErrInvalidInput)
	}
	if !waitSOCKS5(cfg.LocalPort, 5*time.Second) {
		return TunnelStatus{}, fmt.Errorf("Telegram SOCKS5 tunnel did not become ready; check the service log")
	}
	return m.Status(ctx)
}

func (m *TunnelManager) Delete(ctx context.Context) error {
	_, _ = m.systemctl(ctx, "disable", "--now", tunnelUnitName)
	for _, path := range []string{
		"/etc/systemd/system/" + tunnelUnitName,
		tunnelConfigPath,
		tunnelKeyPath,
		tunnelKeyPath + ".pub",
	} {
		if err := os.Remove(m.host.ChrootPath(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	_, _ = m.systemctl(ctx, "daemon-reload")
	return nil
}

func (m *TunnelManager) Logs(ctx context.Context) (string, error) {
	out, err := m.host.RunHostCombined(ctx, "nsenter", "-t", "1", "-m", "-n", "-p", "--", "journalctl", "-u", tunnelUnitName, "-n", "80", "--no-pager")
	if err != nil {
		return out, err
	}
	return strings.TrimSpace(out), nil
}

func (m *TunnelManager) systemctl(ctx context.Context, args ...string) (string, error) {
	nsArgs := []string{"-t", "1", "-m", "-n", "-p", "--", "systemctl"}
	nsArgs = append(nsArgs, args...)
	out, err := m.host.RunHostCombined(ctx, "nsenter", nsArgs...)
	if err != nil {
		return out, fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (m *TunnelManager) readConfig() (TunnelConfig, bool, error) {
	raw, err := os.ReadFile(m.host.ChrootPath(tunnelConfigPath))
	if errors.Is(err, os.ErrNotExist) {
		return normalizeTunnelConfig(TunnelConfig{}), false, nil
	}
	if err != nil {
		return TunnelConfig{}, false, err
	}
	var cfg TunnelConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return TunnelConfig{}, false, fmt.Errorf("read tunnel config: %w", err)
	}
	return normalizeTunnelConfig(cfg), true, nil
}

func (m *TunnelManager) writeConfig(cfg TunnelConfig) error {
	path := m.host.ChrootPath(tunnelConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func normalizeTunnelConfig(cfg TunnelConfig) TunnelConfig {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.SSHUser = strings.TrimSpace(cfg.SSHUser)
	if cfg.SSHPort == 0 {
		cfg.SSHPort = 22
	}
	if cfg.LocalPort == 0 {
		cfg.LocalPort = 1080
	}
	return cfg
}

func validateTunnelConfig(cfg TunnelConfig) error {
	if !tunnelHostPattern.MatchString(cfg.Host) || !tunnelUserPattern.MatchString(cfg.SSHUser) {
		return fmt.Errorf("%w: invalid SSH host or user", ErrInvalidInput)
	}
	if cfg.SSHPort < 1 || cfg.SSHPort > 65535 || cfg.LocalPort < 1024 || cfg.LocalPort > 65535 {
		return fmt.Errorf("%w: invalid SSH or local port", ErrInvalidInput)
	}
	return nil
}

func renderTunnelUnit(cfg TunnelConfig) string {
	return fmt.Sprintf(`[Unit]
Description=SOCKS tunnel to foreign VPS for Telegram
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/bin/ssh -N -o BatchMode=yes -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -i %s -p %d -D 127.0.0.1:%d %s@%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, tunnelKeyPath, cfg.SSHPort, cfg.LocalPort, cfg.SSHUser, cfg.Host)
}

func socks5Ready(port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return false
	}
	reply := make([]byte, 2)
	if _, err := conn.Read(reply); err != nil {
		return false
	}
	return reply[0] == 0x05 && reply[1] == 0x00
}

func waitSOCKS5(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if socks5Ready(port, 700*time.Millisecond) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}
