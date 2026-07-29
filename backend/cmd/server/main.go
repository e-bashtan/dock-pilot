package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ebash/dock-pilot/backend/internal/api"
	"github.com/ebash/dock-pilot/backend/internal/auth"
	"github.com/ebash/dock-pilot/backend/internal/billing"
	"github.com/ebash/dock-pilot/backend/internal/config"
	"github.com/ebash/dock-pilot/backend/internal/deployments"
	"github.com/ebash/dock-pilot/backend/internal/docker"
	"github.com/ebash/dock-pilot/backend/internal/healthcheck"
	"github.com/ebash/dock-pilot/backend/internal/nginx"
	"github.com/ebash/dock-pilot/backend/internal/notifications"
	"github.com/ebash/dock-pilot/backend/internal/panelbackup"
	"github.com/ebash/dock-pilot/backend/internal/pgdb"
	"github.com/ebash/dock-pilot/backend/internal/secrets"
	"github.com/ebash/dock-pilot/backend/internal/sites"
	"github.com/ebash/dock-pilot/backend/internal/ssl"
	"github.com/ebash/dock-pilot/backend/internal/storage"
	"github.com/ebash/dock-pilot/backend/internal/system"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := storage.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := storage.NewQueries(pool)

	cipher, err := secrets.NewCipher(cfg.SecretsEncryptionKey)
	if err != nil {
		logger.Error("init secrets cipher", "error", err)
		os.Exit(1)
	}

	dockerClient, err := docker.NewFromConfig(cfg.Deploy, logger)
	if err != nil {
		logger.Error("init docker", "error", err)
		os.Exit(1)
	}
	if closer, ok := dockerClient.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}

	nginxMgr, err := nginx.NewFromConfig(cfg.Deploy, logger)
	if err != nil {
		logger.Error("init nginx", "error", err)
		os.Exit(1)
	}

	sslMgr, err := ssl.NewFromConfig(cfg.Deploy, logger)
	if err != nil {
		logger.Error("init ssl", "error", err)
		os.Exit(1)
	}

	logger.Info("deploy mode", "mode", cfg.Deploy.Mode, "work_dir", cfg.Deploy.WorkDir)

	healthChecker := healthcheck.NewChecker(dockerClient)
	secretsSvc := secrets.NewService(queries, cipher)
	sitesSvc := sites.NewService(pool, queries, healthChecker, dockerClient, secretsSvc)
	pgdbSvc := pgdb.NewService(queries, dockerClient, cipher, logger)
	panelBackupSvc := panelbackup.NewService(queries, dockerClient, cipher, pgdbSvc, cfg.DatabaseURL, logger)
	notifSvc := notifications.NewService(queries, cipher, sitesSvc, pgdbSvc)
	billingSvc := billing.NewService(queries, cipher, notifSvc, logger)
	worker := deployments.NewWorker(queries, dockerClient, nginxMgr, sslMgr, secretsSvc, cfg.Deploy.WorkDir, logger)
	deploySvc := deployments.NewService(queries, worker)
	notifWorker := notifications.NewWorker(notifSvc, logger)
	pgBackupWorker := pgdb.NewWorker(pgdbSvc, logger)
	panelBackupWorker := panelbackup.NewWorker(panelBackupSvc, logger)
	billingWorker := billing.NewWorker(billingSvc, logger)
	systemSvc := system.NewService(cfg.Deploy.HostRoot, dockerClient)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	notifWorker.Start(workerCtx)
	pgBackupWorker.Start(workerCtx)
	panelBackupWorker.Start(workerCtx)
	billingWorker.Start(workerCtx)

	logger.Info("cors allowed origins", "origins", cfg.CORSAllowedOrigins)
	qrSvc := auth.NewQRService(pool, cfg.APIToken)
	handler := api.Mount(logger, cfg.APIToken, cfg.CORSAllowedOrigins, sitesSvc, secretsSvc, deploySvc, notifSvc, systemSvc, pgdbSvc, panelBackupSvc, billingSvc, api.NewQRHandler(qrSvc))
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       30 * time.Minute, // large SQL dump uploads
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	workerCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
	logger.Info("server stopped")
}
