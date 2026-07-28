package deployments

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/docker"
	"github.com/ebash/dock-pilot/backend/internal/git"
	"github.com/ebash/dock-pilot/backend/internal/nginx"
	secretpkg "github.com/ebash/dock-pilot/backend/internal/secrets"
	"github.com/ebash/dock-pilot/backend/internal/sites"
	"github.com/ebash/dock-pilot/backend/internal/ssl"
)

type Worker struct {
	queries *db.Queries
	docker  docker.Client
	nginx   nginx.Manager
	ssl     ssl.Manager
	secrets *secretpkg.Service
	logger  *slog.Logger
	workDir string
	queue   chan deployJob
}

type deployJob struct {
	site db.Site
	dep  db.Deployment
}

func NewWorker(
	queries *db.Queries,
	dockerClient docker.Client,
	nginxMgr nginx.Manager,
	sslMgr ssl.Manager,
	secrets *secretpkg.Service,
	workDir string,
	logger *slog.Logger,
) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	w := &Worker{
		queries: queries,
		docker:  dockerClient,
		nginx:   nginxMgr,
		ssl:     sslMgr,
		secrets: secrets,
		logger:  logger,
		workDir: workDir,
		queue:   make(chan deployJob, 32),
	}
	go w.run()
	return w
}

func (w *Worker) Enqueue(site db.Site, dep db.Deployment) {
	select {
	case w.queue <- deployJob{site: site, dep: dep}:
	default:
		w.logger.Warn("deployment queue full", "deployment_id", dep.ID)
	}
}

func (w *Worker) run() {
	for job := range w.queue {
		w.process(job)
	}
}

func (w *Worker) process(job deployJob) {
	ctx := context.Background()
	depID := job.dep.ID
	site := job.site
	srcDir := siteSourceDir(w.workDir, site.Slug)

	if err := os.MkdirAll(w.workDir, 0o755); err != nil {
		w.fail(ctx, depID, "prepare work dir: "+err.Error())
		return
	}

	w.updateStatus(ctx, depID, "running", "Deployment started")

	for _, step := range w.buildSteps(site, depID, srcDir) {
		w.appendLog(ctx, depID, step.level, step.message)
		var err error
		site, err = step.fn(ctx)
		if err != nil {
			w.appendLog(ctx, depID, "error", err.Error())
			w.finish(ctx, depID, "failed", err.Error())
			return
		}
	}

	_, _ = w.queries.UpdateSiteStatus(ctx, db.UpdateSiteStatusParams{
		ID:     site.ID,
		Status: "active",
	})

	w.appendLog(ctx, depID, "info", "Cleaning unused Docker images and build cache")
	if prune, err := w.docker.Prune(ctx); err != nil {
		w.appendLog(ctx, depID, "warn", "Docker prune skipped: "+err.Error())
	} else if prune.SpaceReclaimed > 0 || prune.ImagesDeleted > 0 || prune.ContainersDeleted > 0 {
		w.appendLog(ctx, depID, "info", fmt.Sprintf(
			"Docker prune: removed %d image(s), %d container(s), reclaimed %s",
			prune.ImagesDeleted, prune.ContainersDeleted, formatBytes(prune.SpaceReclaimed),
		))
	}

	w.finish(ctx, depID, "succeeded", "Deployment completed successfully")
}

type deployStep struct {
	level   string
	message string
	fn      func(context.Context) (db.Site, error)
}

func (w *Worker) buildSteps(site db.Site, depID uuid.UUID, srcDir string) []deployStep {
	steps := []deployStep{
		{"info", fmt.Sprintf("Cloning %s (branch %s)", site.GitRepoUrl, site.GitBranch), func(ctx context.Context) (db.Site, error) {
			if site.GitRepoUrl == "" {
				return site, fmt.Errorf("git repository url is empty")
			}
			secrets, err := w.secrets.DecryptForDeploy(ctx, site.ID)
			if err != nil {
				return site, fmt.Errorf("load git secrets: %w", err)
			}
			envVars, err := w.queries.ListSiteEnvVars(ctx, site.ID)
			if err != nil {
				return site, fmt.Errorf("list env vars: %w", err)
			}
			envMap := make(map[string]string, len(envVars))
			for _, ev := range envVars {
				envMap[ev.Key] = ev.Value
			}
			token := git.TokenFromSecrets(secrets)
			if token == "" {
				token = git.TokenFromEnv(envMap)
			}
			sshKey := git.SSHKeyFromSecrets(secrets)
			cloneOpts := git.CloneOptions{
				RepoURL:   site.GitRepoUrl,
				Branch:    site.GitBranch,
				Dest:      srcDir,
				GitToken:  token,
				GitSSHKey: sshKey,
			}
			w.appendLog(ctx, depID, "info", "Git auth: "+git.AuthMode(cloneOpts))
			if err := git.Clone(ctx, cloneOpts); err != nil {
				return site, err
			}
			return site, nil
		}},
		{"info", "Building Docker image " + docker.ImageTagForSlug(site.Slug), func(ctx context.Context) (db.Site, error) {
			if err := w.stepBuild(site, srcDir, depID)(ctx); err != nil {
				return site, err
			}
			return site, nil
		}},
	}

	if sites.IsTelegramBot(site.SiteType) {
		steps = append(steps, deployStep{
			"info", "Starting bot container",
			func(ctx context.Context) (db.Site, error) {
				if err := w.stepRun(site)(ctx); err != nil {
					return site, err
				}
				return site, nil
			},
		})
		return steps
	}

	if !sites.UsesHostNetwork(site) {
		steps = append(steps, deployStep{"info", "Allocating host port", func(ctx context.Context) (db.Site, error) {
			if err := w.stepAllocatePort(&site)(ctx); err != nil {
				return site, err
			}
			return w.reloadSite(ctx, site.ID)
		}})
	} else {
		steps = append(steps, deployStep{"info", "Using host network (network_mode: host)", func(ctx context.Context) (db.Site, error) {
			return site, nil
		}})
	}

	steps = append(steps,
		deployStep{"info", "Starting container" + containerNetworkNote(site), func(ctx context.Context) (db.Site, error) {
			if err := w.stepRun(site)(ctx); err != nil {
				return site, err
			}
			return site, nil
		}},
		deployStep{"info", "Writing nginx config", func(ctx context.Context) (db.Site, error) {
			if err := w.stepNginx(site)(ctx); err != nil {
				return site, err
			}
			return site, nil
		}},
		deployStep{"info", "Testing nginx config", func(ctx context.Context) (db.Site, error) {
			return site, w.stepNginxTest(ctx)
		}},
		deployStep{"info", "Reloading nginx", func(ctx context.Context) (db.Site, error) {
			return site, w.stepNginxReload(ctx)
		}},
		deployStep{"info", "Issuing SSL certificate", func(ctx context.Context) (db.Site, error) {
			if err := w.stepSSL(site, depID)(ctx); err != nil {
				return site, err
			}
			return site, nil
		}},
		deployStep{"info", "Updating nginx config for HTTPS", func(ctx context.Context) (db.Site, error) {
			if !site.NginxSslEnabled {
				return site, nil
			}
			if err := w.stepNginx(site)(ctx); err != nil {
				return site, err
			}
			return site, nil
		}},
		deployStep{"info", "Testing nginx config (HTTPS)", func(ctx context.Context) (db.Site, error) {
			if !site.NginxSslEnabled {
				return site, nil
			}
			return site, w.stepNginxTest(ctx)
		}},
		deployStep{"info", "Reloading nginx (HTTPS)", func(ctx context.Context) (db.Site, error) {
			if !site.NginxSslEnabled {
				return site, nil
			}
			return site, w.stepNginxReload(ctx)
		}},
	)
	return steps
}

func (w *Worker) fail(ctx context.Context, depID uuid.UUID, msg string) {
	w.appendLog(ctx, depID, "error", msg)
	w.finish(ctx, depID, "failed", msg)
}

func (w *Worker) stepBuild(site db.Site, srcDir string, depID uuid.UUID) func(context.Context) error {
	return func(ctx context.Context) error {
		tag := docker.ImageTagForSlug(site.Slug)
		return w.docker.Build(ctx, docker.BuildOptions{
			ContextDir:     srcDir,
			RepoURL:        site.GitRepoUrl,
			Branch:         site.GitBranch,
			DockerfilePath: site.DockerfilePath,
			BuildContext:   site.BuildContext,
			ImageTag:       tag,
			OnOutput: func(line string) {
				if len(line) > 4000 {
					line = line[:4000] + "…"
				}
				w.appendLog(ctx, depID, "info", line)
			},
		})
	}
}

func (w *Worker) stepAllocatePort(site *db.Site) func(context.Context) error {
	return func(ctx context.Context) error {
		if site.HostPort.Valid {
			if portFree, err := w.hostPortAvailable(ctx, int(site.HostPort.Int32)); err != nil {
				return err
			} else if portFree {
				return nil
			}
			// Port belongs to another site/container — pick a new one, do not stop other containers.
			w.logger.InfoContext(ctx, "host port in use by another site, allocating new port",
				"site", site.Slug, "old_port", site.HostPort.Int32)
		}
		port, err := w.docker.AllocatePort(ctx)
		if err != nil {
			return err
		}
		updated, err := w.queries.UpdateSiteHostPort(ctx, db.UpdateSiteHostPortParams{
			ID:       site.ID,
			HostPort: pgtype.Int4{Int32: int32(port), Valid: true},
		})
		if err != nil {
			return err
		}
		*site = updated
		w.logger.InfoContext(ctx, "allocated port", "site", site.Slug, "port", port)
		return nil
	}
}

func (w *Worker) reloadSite(ctx context.Context, id uuid.UUID) (db.Site, error) {
	return w.queries.GetSite(ctx, id)
}

func (w *Worker) hostPortAvailable(ctx context.Context, port int) (bool, error) {
	type checker interface {
		HostPortAvailable(context.Context, int) bool
	}
	if c, ok := w.docker.(checker); ok {
		return c.HostPortAvailable(ctx, port), nil
	}
	return true, nil
}

func containerNetworkNote(site db.Site) string {
	if sites.UsesHostNetwork(site) {
		return " (network_mode: host)"
	}
	return ""
}

func (w *Worker) stepRun(site db.Site) func(context.Context) error {
	return func(ctx context.Context) error {
		hostNetwork := sites.UsesHostNetwork(site)
		publishPorts := sites.IsWebSite(site.SiteType) && !hostNetwork
		if publishPorts && !site.HostPort.Valid {
			return fmt.Errorf("host port not allocated")
		}

		env, err := w.loadContainerEnv(ctx, site.ID)
		if err != nil {
			return err
		}
		if publishPorts {
			if _, ok := env["PORT"]; !ok {
				env["PORT"] = strconv.Itoa(int(site.ContainerPort))
			}
		}

		mountLines := sites.VolumeLinesFromSite(site)
		namedLines := sites.NamedVolumeLinesFromSite(site)
		volMounts, ensureVolumes, err := docker.ParseVolumeConfig(site.Slug, mountLines, namedLines)
		if err != nil {
			return err
		}

		tag := docker.ImageTagForSlug(site.Slug)
		containerName, stopNames := docker.NamesForSite(site.Slug, site.PrimaryUrl)
		runOpts := docker.RunOptions{
			ImageTag:      tag,
			ContainerName: containerName,
			StopNames:     stopNames,
			Env:           env,
			PublishPorts:  publishPorts,
			NetworkHost:   hostNetwork,
			Mounts:        volMounts,
			EnsureVolumes: ensureVolumes,
		}
		if publishPorts {
			runOpts.HostPort = int(site.HostPort.Int32)
			runOpts.ContainerPort = int(site.ContainerPort)
		}
		containerID, err := w.docker.Run(ctx, runOpts)
		if err != nil {
			return err
		}
		w.logger.InfoContext(ctx, "container started",
			"id", containerID,
			"name", containerName,
			"stopped", stopNames,
		)
		return nil
	}
}

func (w *Worker) loadContainerEnv(ctx context.Context, siteID uuid.UUID) (map[string]string, error) {
	envVars, err := w.queries.ListSiteEnvVars(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}
	secretMap, err := w.secrets.DecryptForDeploy(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("load secrets: %w", err)
	}

	out := make(map[string]string, len(envVars)+len(secretMap))
	for _, ev := range envVars {
		out[ev.Key] = ev.Value
	}
	for k, v := range secretMap {
		out[k] = v
	}
	return out, nil
}

func (w *Worker) stepNginx(site db.Site) func(context.Context) error {
	return func(ctx context.Context) error {
		domains, err := w.queries.ListSiteDomains(ctx, site.ID)
		if err != nil {
			return err
		}
		aliases := make([]string, 0)
		primary := extractHost(site.PrimaryUrl)
		for _, d := range domains {
			if d.Domain != primary {
				aliases = append(aliases, d.Domain)
			}
		}
		port := sites.UpstreamPort(site)
		configKey := primary
		if configKey == "" {
			configKey = site.Slug
		}
		return w.nginx.WriteConfig(ctx, configKey, nginx.SiteConfig{
			PrimaryDomain: primary,
			Aliases:       aliases,
			UpstreamHost:  "127.0.0.1",
			UpstreamPort:  port,
			SSLEnabled:    site.NginxSslEnabled,
			ForceHTTPS:    site.NginxForceHttps,
		})
	}
}

func (w *Worker) stepNginxTest(ctx context.Context) error {
	return w.nginx.TestConfig(ctx)
}

func (w *Worker) stepNginxReload(ctx context.Context) error {
	return w.nginx.Reload(ctx)
}

func (w *Worker) stepSSL(site db.Site, depID uuid.UUID) func(context.Context) error {
	return func(ctx context.Context) error {
		if !site.NginxSslEnabled {
			w.appendLog(ctx, depID, "info", "SSL disabled, skipping certbot")
			return nil
		}
		domains, err := w.queries.ListSiteDomains(ctx, site.ID)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(domains))
		for _, d := range domains {
			names = append(names, d.Domain)
		}
		names = mergeCertDomains(extractHost(site.PrimaryUrl), names)
		return w.ssl.IssueCertificate(ctx, names)
	}
}

func (w *Worker) appendLog(ctx context.Context, depID uuid.UUID, level, message string) {
	_, err := w.queries.AppendDeploymentLog(ctx, db.AppendDeploymentLogParams{
		DeploymentID: depID,
		Level:        level,
		Message:      message,
	})
	if err != nil {
		w.logger.Error("append deployment log", "error", err, "deployment_id", depID)
	}
}

func (w *Worker) updateStatus(ctx context.Context, depID uuid.UUID, status, message string) {
	_, err := w.queries.UpdateDeployment(ctx, db.UpdateDeploymentParams{
		ID:      depID,
		Status:  textVal(status),
		Message: textVal(message),
	})
	if err != nil {
		w.logger.Error("update deployment status", "error", err, "deployment_id", depID)
	}
}

func (w *Worker) finish(ctx context.Context, depID uuid.UUID, status, message string) {
	now := time.Now().UTC()
	_, err := w.queries.UpdateDeployment(ctx, db.UpdateDeploymentParams{
		ID:         depID,
		Status:     textVal(status),
		Message:    textVal(message),
		FinishedAt: timestamptz(now),
	})
	if err != nil {
		w.logger.Error("finish deployment", "error", err, "deployment_id", depID)
	}
	w.appendLog(ctx, depID, "info", message)
}

// mergeCertDomains puts primary first (Let's Encrypt cert directory name).
func mergeCertDomains(primary string, domains []string) []string {
	seen := map[string]bool{}
	var out []string
	if p := strings.TrimSpace(primary); p != "" {
		out = append(out, p)
		seen[p] = true
	}
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d != "" && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func extractHost(url string) string {
	u := strings.TrimSpace(url)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}
