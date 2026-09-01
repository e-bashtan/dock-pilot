package notifications

import (
	"strings"
	"testing"
)

func TestValidateTunnelConfig(t *testing.T) {
	t.Parallel()
	valid := TunnelConfig{Host: "foreign-vps.example.com", SSHPort: 22, SSHUser: "tunnel", LocalPort: 1080}
	if err := validateTunnelConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for _, cfg := range []TunnelConfig{
		{Host: "host;reboot", SSHPort: 22, SSHUser: "tunnel", LocalPort: 1080},
		{Host: "example.com", SSHPort: 22, SSHUser: "root -o ProxyCommand=x", LocalPort: 1080},
		{Host: "example.com", SSHPort: 0, SSHUser: "tunnel", LocalPort: 1080},
		{Host: "example.com", SSHPort: 22, SSHUser: "tunnel", LocalPort: 80},
	} {
		if err := validateTunnelConfig(cfg); err == nil {
			t.Fatalf("unsafe config accepted: %+v", cfg)
		}
	}
}

func TestRenderTunnelUnit(t *testing.T) {
	t.Parallel()
	unit := renderTunnelUnit(TunnelConfig{Host: "203.0.113.10", SSHPort: 2222, SSHUser: "tunnel", LocalPort: 1080})
	for _, expected := range []string{
		"BatchMode=yes",
		"ExitOnForwardFailure=yes",
		"-p 2222",
		"-D 127.0.0.1:1080",
		"tunnel@203.0.113.10",
		"Restart=always",
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("unit does not contain %q", expected)
		}
	}
}
