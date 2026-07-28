package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	deploysvc "github.com/ebash/dock-pilot/backend/internal/deployments"
	notifpkg "github.com/ebash/dock-pilot/backend/internal/notifications"
	"github.com/ebash/dock-pilot/backend/internal/pgdb"
	secretpkg "github.com/ebash/dock-pilot/backend/internal/secrets"
	sitesvc "github.com/ebash/dock-pilot/backend/internal/sites"
	syspkg "github.com/ebash/dock-pilot/backend/internal/system"
)

type Handlers struct {
	Sites         *SitesHandler
	Secrets       *SecretsHandler
	Deployments   *DeploymentsHandler
	Notifications *NotificationsHandler
	QR            *QRHandler
	System        *SystemHandler
	Databases     *DatabasesHandler
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
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/qr/exchange", h.QR.Exchange)

		r.Group(func(r chi.Router) {
			r.Use(BearerTokenAuth(apiToken))

			r.Post("/auth/qr", h.QR.Create)

			r.Route("/sites", func(r chi.Router) {
				r.Post("/", h.Sites.Create)
				r.Get("/", h.Sites.List)
				r.Get("/health", h.Sites.HealthAll)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.Sites.Get)
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
				r.Post("/docker/prune", h.System.PruneDocker)
			})

			r.Route("/databases", func(r chi.Router) {
				r.Get("/", h.Databases.ListInstances)
				r.Post("/", h.Databases.CreateInstance)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.Databases.GetInstance)
					r.Post("/deploy", h.Databases.DeployInstance)
					r.Post("/stop", h.Databases.StopInstance)
					r.Delete("/", h.Databases.DeleteInstance)

					r.Get("/databases", h.Databases.ListDatabases)
					r.Post("/databases", h.Databases.CreateDatabase)
					r.Delete("/databases/{dbId}", h.Databases.DeleteDatabase)

					r.Get("/roles", h.Databases.ListRoles)
					r.Post("/roles", h.Databases.CreateRole)
					r.Delete("/roles/{roleId}", h.Databases.DeleteRole)
					r.Post("/roles/{roleId}/grants", h.Databases.GrantRole)

					r.Get("/connection", h.Databases.ConnectionInfo)

					r.Get("/schedules", h.Databases.ListSchedules)
					r.Post("/schedules", h.Databases.CreateSchedule)
					r.Patch("/schedules/{scheduleId}", h.Databases.UpdateSchedule)
					r.Delete("/schedules/{scheduleId}", h.Databases.DeleteSchedule)

					r.Get("/backups", h.Databases.ListBackups)
					r.Post("/backups", h.Databases.ManualBackup)
					r.Post("/backups/{backupId}/restore", h.Databases.RestoreBackup)
				})
			})

			r.Get("/deployments/{id}/logs/stream", h.Deployments.StreamLogs)
		})
	})

	return r
}

func Mount(logger *slog.Logger, apiToken string, corsOrigins []string, sites *sitesvc.Service, secrets *secretpkg.Service, deployments *deploysvc.Service, notifications *notifpkg.Service, systemSvc *syspkg.Service, databases *pgdb.Service, qr *QRHandler) http.Handler {
	_ = logger
	return NewRouter(Handlers{
		Sites:         NewSitesHandler(sites),
		Secrets:       NewSecretsHandler(secrets),
		Deployments:   NewDeploymentsHandler(deployments),
		Notifications: NewNotificationsHandler(notifications),
		QR:            qr,
		System:        NewSystemHandler(systemSvc),
		Databases:     NewDatabasesHandler(databases),
	}, apiToken, corsOrigins)
}
