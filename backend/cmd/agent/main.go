package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/agent"
	"github.com/ebash/barn/backend/internal/metrics"
)

// Set via: go build -ldflags "-X main.version=v1.2.3"
var version = "dev"

func main() {
	configPath := flag.String("config", agent.DefaultConfigPath, "path to agent config.json")
	registerMode := flag.Bool("register", false, "register with master using a one-time registration token")
	regToken := flag.String("registration-token", "", "one-time registration token (with -register)")
	masterURL := flag.String("master-url", "", "master base URL (with -register)")
	nodeUID := flag.String("node-uid", "", "node UID to register (optional; generated if empty)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *registerMode {
		if err := runRegister(logger, *configPath, *masterURL, *regToken, *nodeUID); err != nil {
			logger.Error("register failed", "error", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := agent.Load(*configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", *configPath)
		os.Exit(1)
	}
	if err := cfg.ValidateRuntime(); err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := agent.NewRunner(cfg, version, logger)
	logger.Info("barn-agent starting",
		"version", version,
		"master_url", cfg.MasterURL,
		"node_uid", cfg.NodeUID,
		"heartbeat_interval_seconds", cfg.HeartbeatIntervalSeconds,
	)
	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("runner stopped", "error", err)
		os.Exit(1)
	}
	logger.Info("barn-agent stopped")
}

func runRegister(logger *slog.Logger, configPath, masterURL, regToken, nodeUID string) error {
	if masterURL == "" {
		return fmt.Errorf("-master-url is required with -register")
	}
	if regToken == "" {
		return fmt.Errorf("-registration-token is required with -register")
	}
	if nodeUID == "" {
		nodeUID = uuid.New().String()
	}

	hostname, _ := os.Hostname()
	snap, err := metrics.New("").Collect()
	if err != nil {
		logger.Warn("metrics collect during register", "error", err)
	}
	if snap.Hostname == "" {
		snap.Hostname = hostname
	}

	client := agent.NewClient(masterURL, "")
	client.UserAgent = "barn-agent/" + version

	ctx := context.Background()
	resp, err := client.Register(ctx, agent.RegisterRequest{
		RegistrationToken: regToken,
		NodeUID:           nodeUID,
		Hostname:          snap.Hostname,
		AgentVersion:      version,
		Metrics:           snap,
	})
	if err != nil {
		return err
	}

	cfg := agent.DefaultConfig()
	cfg.MasterURL = resp.MasterURL
	if cfg.MasterURL == "" {
		cfg.MasterURL = masterURL
	}
	cfg.NodeUID = nodeUID
	cfg.NodeToken = resp.NodeToken
	if resp.HeartbeatSeconds > 0 {
		cfg.HeartbeatIntervalSeconds = resp.HeartbeatSeconds
	}

	if err := agent.Save(configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	logger.Info("registered with master",
		"config", configPath,
		"master_url", cfg.MasterURL,
		"node_uid", cfg.NodeUID,
		"heartbeat_interval_seconds", cfg.HeartbeatIntervalSeconds,
	)
	return nil
}
