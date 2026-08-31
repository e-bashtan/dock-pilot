package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ebash/barn/backend/internal/billing"
	deploysvc "github.com/ebash/barn/backend/internal/deployments"
	notifpkg "github.com/ebash/barn/backend/internal/notifications"
	"github.com/ebash/barn/backend/internal/panelbackup"
	"github.com/ebash/barn/backend/internal/pgdb"
	secretpkg "github.com/ebash/barn/backend/internal/secrets"
	"github.com/ebash/barn/backend/internal/servers"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
	syspkg "github.com/ebash/barn/backend/internal/system"
)

type Handlers struct {
	Sites         *SitesHandler
	Secrets       *SecretsHandler
	Deployments   *DeploymentsHandler
	Notifications *NotificationsHandler
	QR            *QRHandler
	System        *SystemHandler
	Databases     *DatabasesHandler
	Backups       *BackupsHandler
	Billing       *BillingHandler
	Servers       *ServersHandler
	ServersSvc    *servers.Service
}

func NewRouter(h Handlers, apiToken string, corsOrigins []string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Token", "X-Barn-Token", "X-Fleet-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/qr/exchange", h.QR.Exchange)

		if h.Servers != nil && h.ServersSvc != nil {
			r.Post("/servers/node/pair", h.Servers.AcceptPair)
			r.Group(func(r chi.Router) {
				r.Use(h.ServersSvc.ServersTokenAuth(servers.ScopeStatusRead))
				r.Get("/servers/node/status", h.Servers.NodeStatus)
			})
			r.Group(func(r chi.Router) {
				r.Use(h.ServersSvc.ServersTokenAuth(servers.ScopeAppsRead))
				r.Get("/servers/node/apps", h.Servers.NodeApps)
			})
			r.Group(func(r chi.Router) {
				r.Use(h.ServersSvc.ServersTokenAuth(servers.ScopeBackupsRead))
				r.Get("/servers/node/backups", h.Servers.NodeBackups)
			})
			r.Group(func(r chi.Router) {
				r.Use(h.ServersSvc.ServersTokenAuth(servers.ScopeVersionRead))
				r.Get("/servers/node/version", h.Servers.NodeVersion)
			})
			r.Group(func(r chi.Router) {
				r.Use(h.ServersSvc.ServersTokenAuth(servers.ScopeHeartbeatWrite))
				r.Post("/servers/ingest/heartbeat", h.Servers.IngestHeartbeat)
			})
			r.Group(func(r chi.Router) {
				r.Use(h.ServersSvc.ServersTokenAuth(servers.ScopeEventsWrite))
				r.Post("/servers/ingest/events", h.Servers.IngestEvent)
				r.Post("/servers/ingest/events/batch", h.Servers.IngestEventBatch)
			})
			r.Post("/servers/agent/register", h.Servers.AgentRegister)
		}

		r.Group(func(r chi.Router) {
			r.Use(BearerTokenAuth(apiToken))

			r.Post("/auth/qr", h.QR.Create)

			if h.Servers != nil {
				r.Route("/servers", func(r chi.Router) {
					r.Get("/settings", h.Servers.GetSettings)
					r.Put("/settings", h.Servers.UpdateSettings)
					r.Get("/overview", h.Servers.Overview)
					r.Get("/nodes", h.Servers.ListNodes)
					r.Post("/nodes/barn", h.Servers.PairBarn)
					r.Post("/nodes/dockpilot", h.Servers.PairDockpilot)
					r.Get("/nodes/{id}", h.Servers.GetNode)
					r.Patch("/nodes/{id}", h.Servers.UpdateNode)
					r.Put("/nodes/{id}/billing", h.Servers.UpdateNodeBilling)
					r.Post("/nodes/{id}/update-agent", h.Servers.StartAgentUpdate)
					r.Delete("/nodes/{id}", h.Servers.DeleteNode)
					r.Get("/events", h.Servers.ListEvents)
					r.Get("/incidents", h.Servers.ListIncidents)
					r.Post("/pairing-code", h.Servers.CreatePairingCode)
					r.Delete("/master", h.Servers.DisconnectMaster)
					r.Post("/installations/agent", h.Servers.StartAgentInstall)
					r.Get("/installations/{id}", h.Servers.GetInstallation)
					r.Post("/installations/{id}/confirm-host-key", h.Servers.ConfirmHostKey)
					r.Delete("/installations/{id}", h.Servers.CancelInstallation)
					r.Get("/installations/{id}/logs", h.Servers.ListInstallationLogs)
				})
			}

			r.Route("/sites", func(r chi.Router) {
				r.Post("/", h.Sites.Create)
				r.Get("/", h.Sites.List)
				r.Get("/health", h.Sites.HealthAll)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.Sites.Get)
					r.Get("/export", h.Sites.Export)
					r.Get("/health", h.Sites.Health)
					r.Get("/logs/stream", h.Sites.StreamContainerLogs)
					r.Patch("/", h.Sites.Update)
					r.Delete("/", h.Sites.Delete)

					r.Post("/deploy", h.Deployments.Deploy)
					r.Post("/container/start", h.Sites.StartContainer)
					r.Post("/container/stop", h.Sites.StopContainer)
					r.Post("/container/restart", h.Sites.RestartContainer)
					r.Get("/deployments", h.Deployments.ListBySite)

					r.Route("/secrets", func(r chi.Router) {
						r.Get("/", h.Secrets.List)
						r.Post("/", h.Secrets.CreateMany)
						r.Put("/{key}", h.Secrets.Upsert)
						r.Delete("/{key}", h.Secrets.Delete)
					})
				})
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/settings", h.Notifications.GetSettings)
				r.Put("/settings", h.Notifications.UpdateSettings)
				r.Post("/test", h.Notifications.SendTest)
			})

			r.Route("/system", func(r chi.Router) {
				r.Get("/status", h.System.Status)
				r.Get("/host", h.System.HostInfo)
				r.Get("/processes", h.System.Processes)
				r.Get("/docker-dirs", h.System.DockerDirs)
				r.Post("/docker/prune", h.System.PruneDocker)
				r.Get("/update", h.System.UpdateInfo)
				r.Post("/update", h.System.StartUpdate)
				r.Get("/update/job", h.System.UpdateJob)
			})

			r.Route("/backups", func(r chi.Router) {
				r.Get("/settings", h.Backups.GetSettings)
				r.Put("/settings", h.Backups.UpdateSettings)
				r.Get("/full", h.Backups.ListFull)
				r.Post("/full", h.Backups.CreateFull)
				r.Post("/full/restore", h.Backups.RestoreFull)
				r.Get("/full/restore/stream", h.Backups.StreamRestoreFull)
				r.Post("/settings/test", h.Backups.TestS3)
				r.Get("/operations", h.Backups.ListOperations)
			})

			r.Route("/billing", func(r chi.Router) {
				r.Get("/accounts", h.Billing.List)
				r.Post("/accounts", h.Billing.Create)
				r.Patch("/accounts/{id}", h.Billing.Update)
				r.Delete("/accounts/{id}", h.Billing.Delete)
				r.Post("/accounts/{id}/refresh", h.Billing.Refresh)
			})

			r.Route("/databases", func(r chi.Router) {
				r.Get("/", h.Databases.ListInstances)
				r.Post("/", h.Databases.CreateInstance)
				r.Get("/health", h.Databases.HealthAll)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.Databases.GetInstance)
					r.Get("/health", h.Databases.Health)
					r.Post("/deploy", h.Databases.DeployInstance)
					r.Get("/deploy/stream", h.Databases.StreamDeploy)
					r.Post("/stop", h.Databases.StopInstance)
					r.Delete("/", h.Databases.DeleteInstance)

					r.Get("/databases", h.Databases.ListDatabases)
					r.Post("/databases", h.Databases.CreateDatabase)
					r.Delete("/databases/{dbId}", h.Databases.DeleteDatabase)
					r.Get("/databases/{dbId}/tables", h.Databases.ListTables)
					r.Post("/databases/{dbId}/select", h.Databases.SelectTable)

					r.Get("/roles", h.Databases.ListRoles)
					r.Post("/roles", h.Databases.CreateRole)
					r.Delete("/roles/{roleId}", h.Databases.DeleteRole)
					r.Post("/roles/{roleId}/grants", h.Databases.GrantRole)

					r.Get("/connection", h.Databases.ConnectionInfo)
					r.Get("/admin-credentials", h.Databases.AdminCredentials)

					r.Get("/schedules", h.Databases.ListSchedules)
					r.Post("/schedules", h.Databases.CreateSchedule)
					r.Patch("/schedules/{scheduleId}", h.Databases.UpdateSchedule)
					r.Delete("/schedules/{scheduleId}", h.Databases.DeleteSchedule)

					r.Get("/backups", h.Databases.ListBackups)
					r.Post("/backups", h.Databases.ManualBackup)
					r.Post("/backups/restore", h.Databases.RestoreBackup)
					r.Get("/backups/restore/stream", h.Databases.StreamRestoreBackup)
					r.Post("/restore-upload", h.Databases.RestoreUpload)
				})
			})

			r.Get("/deployments/{id}/logs/stream", h.Deployments.StreamLogs)
		})
	})

	return r
}

func Mount(logger *slog.Logger, apiToken string, corsOrigins []string, sites *sitesvc.Service, secrets *secretpkg.Service, deployments *deploysvc.Service, notifications *notifpkg.Service, systemSvc *syspkg.Service, databases *pgdb.Service, backups *panelbackup.Service, billingSvc *billing.Service, serversSvc *servers.Service, qr *QRHandler) http.Handler {
	_ = logger
	h := Handlers{
		Sites:         NewSitesHandler(sites, secrets),
		Secrets:       NewSecretsHandler(secrets),
		Deployments:   NewDeploymentsHandler(deployments),
		Notifications: NewNotificationsHandler(notifications),
		QR:            qr,
		System:        NewSystemHandler(systemSvc),
		Databases:     NewDatabasesHandler(databases),
		Backups:       NewBackupsHandler(backups),
		Billing:       NewBillingHandler(billingSvc),
	}
	if serversSvc != nil {
		h.Servers = NewServersHandler(serversSvc)
		h.ServersSvc = serversSvc
	}
	return NewRouter(h, apiToken, corsOrigins)
}
