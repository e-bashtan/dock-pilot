package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultConfigPath = "/etc/dockpilot-agent/config.json"

// Config is the on-disk agent configuration (mode 0600).
type Config struct {
	MasterURL                string   `json:"master_url"`
	NodeUID                  string   `json:"node_uid"`
	NodeToken                string   `json:"node_token"`
	HeartbeatIntervalSeconds int      `json:"heartbeat_interval_seconds"`
	MonitoredUnits           []string `json:"monitored_units,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		HeartbeatIntervalSeconds: 30,
		MonitoredUnits:           []string{},
	}
}

// Load reads and parses the agent config file.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.HeartbeatIntervalSeconds <= 0 {
		cfg.HeartbeatIntervalSeconds = 30
	}
	if cfg.MonitoredUnits == nil {
		cfg.MonitoredUnits = []string{}
	}
	return cfg, nil
}

// Save writes config atomically with mode 0600.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	b = append(b, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod config: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func (c Config) ValidateRuntime() error {
	if c.MasterURL == "" {
		return fmt.Errorf("master_url is required")
	}
	if c.NodeUID == "" {
		return fmt.Errorf("node_uid is required")
	}
	if c.NodeToken == "" {
		return fmt.Errorf("node_token is required")
	}
	return nil
}
