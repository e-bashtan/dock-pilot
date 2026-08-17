package sites

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
	"github.com/ebash/barn/backend/internal/healthcheck"
	"github.com/ebash/barn/backend/internal/nginx"
	"github.com/ebash/barn/backend/internal/ssl"
)

// DeploySecrets loads decrypted secrets for running a site container.
type DeploySecrets interface {
	DecryptForDeploy(ctx context.Context, siteID uuid.UUID) (map[string]string, error)
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	health  *healthcheck.Checker
	docker  docker.Client
	secrets DeploySecrets
	nginx   nginx.Manager
	ssl     ssl.Manager
	logger  *slog.Logger
}

func NewService(
	pool *pgxpool.Pool,
	queries *db.Queries,
	checker *healthcheck.Checker,
	dockerClient docker.Client,
	secretsSvc DeploySecrets,
	nginxMgr nginx.Manager,
	sslMgr ssl.Manager,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		pool:    pool,
		queries: queries,
		health:  checker,
		docker:  dockerClient,
		secrets: secretsSvc,
		nginx:   nginxMgr,
		ssl:     sslMgr,
		logger:  logger,
	}
}

func (s *Service) Create(ctx context.Context, req CreateSiteRequest) (SiteResponse, error) {
	if err := validateCreate(req); err != nil {
		return SiteResponse{}, err
	}

	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = slugify(req.Name)
	}

	siteType := NormalizeSiteType(req.SiteType)
	primaryURL := strings.TrimSpace(req.PrimaryURL)
	nginxSSL := req.NginxSSLEnabled
	nginxForce := req.NginxForceHTTPS
	containerPort := defaultInt32(req.ContainerPort, 3000)
	healthPath := ""
	if IsWebSite(siteType) {
		healthPath = NormalizeHealthCheckPath(req.HealthCheckPath)
	}

	if IsTelegramBot(siteType) {
		if primaryURL == "" {
			primaryURL = "telegram://" + slug
		}
		nginxSSL = false
		nginxForce = false
	} else if primaryURL == "" {
		return SiteResponse{}, fmt.Errorf("%w: primary_url is required", ErrInvalidInput)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SiteResponse{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	site, err := qtx.CreateSite(ctx, db.CreateSiteParams{
		Name:            strings.TrimSpace(req.Name),
		Slug:            slug,
		PrimaryUrl:      primaryURL,
		GitRepoUrl:      strings.TrimSpace(req.GitRepoURL),
		GitBranch:       defaultStr(req.GitBranch, "main"),
		DockerfilePath:  defaultStr(req.DockerfilePath, "Dockerfile"),
		BuildContext:    defaultStr(req.BuildContext, "."),
		ContainerPort:   containerPort,
		HostPort:        pgtype.Int4{Valid: false},
		NginxSslEnabled: nginxSSL,
		NginxForceHttps: nginxForce,
		SiteType:           siteType,
		DockerVolumeMounts: volumeLinesToText(req.DockerVolumeMounts),
		DockerNamedVolumes: volumeLinesToText(req.DockerNamedVolumes),
		DockerNetworkHost:  req.DockerNetworkHost,
		HealthCheckPath:    healthPath,
		Status:             "draft",
	})
	if err != nil {
		if isUniqueViolation(err) {
			return SiteResponse{}, ErrSlugConflict
		}
		return SiteResponse{}, fmt.Errorf("create site: %w", err)
	}

	var domains []db.SiteDomain
	if IsWebSite(siteType) {
		domains, err = syncDomains(ctx, qtx, site.ID, primaryURL, req.Domains)
		if err != nil {
			return SiteResponse{}, err
		}
	}

	envVars, err := syncEnvVars(ctx, qtx, site.ID, req.EnvVars)
	if err != nil {
		return SiteResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SiteResponse{}, fmt.Errorf("commit tx: %w", err)
	}

	return toSiteResponse(site, domains, envVars), nil
}

func (s *Service) List(ctx context.Context) ([]SiteListItem, error) {
	rows, err := s.queries.ListSites(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	deployTimes, err := s.queries.ListLatestDeployTimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list latest deploys: %w", err)
	}
	lastDeploy := make(map[uuid.UUID]time.Time, len(deployTimes))
	for _, row := range deployTimes {
		lastDeploy[row.SiteID] = row.CreatedAt
	}
	items := make([]SiteListItem, 0, len(rows))
	for _, row := range rows {
		item := toListItem(row)
		if t, ok := lastDeploy[row.ID]; ok {
			item.LastDeployedAt = &t
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (SiteResponse, error) {
	site, err := s.queries.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiteResponse{}, ErrNotFound
		}
		return SiteResponse{}, fmt.Errorf("get site: %w", err)
	}
	return s.loadFull(ctx, site)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateSiteRequest) (SiteResponse, error) {
	if req.HealthCheckPath != nil {
		if err := validateHealthCheckPath(*req.HealthCheckPath); err != nil {
			return SiteResponse{}, err
		}
	}

	if _, err := s.queries.GetSite(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiteResponse{}, ErrNotFound
		}
		return SiteResponse{}, fmt.Errorf("get site: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SiteResponse{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	site, err := qtx.UpdateSite(ctx, db.UpdateSiteParams{
		ID:              id,
		Name:            textPtr(req.Name),
		PrimaryUrl:      textPtr(req.PrimaryURL),
		GitRepoUrl:      textPtr(req.GitRepoURL),
		GitBranch:       textPtr(req.GitBranch),
		DockerfilePath:  textPtr(req.DockerfilePath),
		BuildContext:    textPtr(req.BuildContext),
		ContainerPort:   int4Opt(req.ContainerPort),
		HostPort:        int4Opt(req.HostPort),
		NginxSslEnabled: boolOpt(req.NginxSSLEnabled),
		NginxForceHttps: boolOpt(req.NginxForceHTTPS),
		Status:             textPtr(req.Status),
		DockerVolumeMounts: volumeTextPtr(req.DockerVolumeMounts),
		DockerNamedVolumes: volumeTextPtr(req.DockerNamedVolumes),
		DockerNetworkHost:  boolOpt(req.DockerNetworkHost),
		HealthCheckPath:    healthCheckPathPtr(req.HealthCheckPath),
	})
	if err != nil {
		return SiteResponse{}, fmt.Errorf("update site: %w", err)
	}

	var domains []db.SiteDomain
	if req.Domains != nil {
		primary := site.PrimaryUrl
		if req.PrimaryURL != nil {
			primary = *req.PrimaryURL
		}
		domains, err = syncDomains(ctx, qtx, id, primary, req.Domains)
		if err != nil {
			return SiteResponse{}, err
		}
	}

	var envVars []db.SiteEnvVar
	if req.EnvVars != nil {
		envVars, err = syncEnvVars(ctx, qtx, id, req.EnvVars)
		if err != nil {
			return SiteResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return SiteResponse{}, fmt.Errorf("commit tx: %w", err)
	}

	if req.Domains == nil || req.EnvVars == nil {
		return s.loadFull(ctx, site)
	}
	return toSiteResponse(site, domains, envVars), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	site, err := s.queries.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get site: %w", err)
	}

	names := containerNamesForSite(site)
	if err := s.docker.Stop(ctx, names...); err != nil {
		s.logger.WarnContext(ctx, "stop container on site delete", "site_id", id, "error", err)
	}

	primary := extractHost(site.PrimaryUrl)
	if IsWebSite(site.SiteType) {
		configKey := primary
		if configKey == "" {
			configKey = site.Slug
		}
		if s.nginx != nil {
			if err := s.nginx.RemoveConfig(ctx, configKey); err != nil {
				s.logger.WarnContext(ctx, "remove nginx config on site delete", "site_id", id, "error", err)
			} else if err := s.nginx.TestConfig(ctx); err != nil {
				s.logger.WarnContext(ctx, "nginx -t after site delete", "site_id", id, "error", err)
			} else if err := s.nginx.Reload(ctx); err != nil {
				s.logger.WarnContext(ctx, "nginx reload after site delete", "site_id", id, "error", err)
			}
		}
		if s.ssl != nil && primary != "" {
			if err := s.ssl.DeleteCertificate(ctx, primary); err != nil {
				s.logger.WarnContext(ctx, "delete certificate on site delete", "site_id", id, "domain", primary, "error", err)
			}
		}
	}

	if err := s.queries.DeleteSite(ctx, id); err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	return nil
}

func (s *Service) loadFull(ctx context.Context, site db.Site) (SiteResponse, error) {
	domains, err := s.queries.ListSiteDomains(ctx, site.ID)
	if err != nil {
		return SiteResponse{}, fmt.Errorf("list domains: %w", err)
	}
	envVars, err := s.queries.ListSiteEnvVars(ctx, site.ID)
	if err != nil {
		return SiteResponse{}, fmt.Errorf("list env vars: %w", err)
	}
	return toSiteResponse(site, domains, envVars), nil
}

func syncDomains(ctx context.Context, q *db.Queries, siteID uuid.UUID, primaryURL string, inputs []DomainInput) ([]db.SiteDomain, error) {
	if err := q.DeleteSiteDomains(ctx, siteID); err != nil {
		return nil, fmt.Errorf("clear domains: %w", err)
	}

	seen := map[string]bool{}
	var result []db.SiteDomain

	if primaryURL != "" {
		d, err := q.UpsertSiteDomain(ctx, db.UpsertSiteDomainParams{
			SiteID:    siteID,
			Domain:    extractHost(primaryURL),
			IsPrimary: true,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert primary domain: %w", err)
		}
		result = append(result, d)
		seen[d.Domain] = true
	}

	for _, in := range inputs {
		domain := strings.TrimSpace(in.Domain)
		if domain == "" || seen[domain] {
			continue
		}
		d, err := q.UpsertSiteDomain(ctx, db.UpsertSiteDomainParams{
			SiteID:    siteID,
			Domain:    domain,
			IsPrimary: in.IsPrimary,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert domain: %w", err)
		}
		result = append(result, d)
		seen[domain] = true
	}

	return result, nil
}

func syncEnvVars(ctx context.Context, q *db.Queries, siteID uuid.UUID, inputs []EnvVarInput) ([]db.SiteEnvVar, error) {
	existing, err := q.ListSiteEnvVars(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}

	incoming := make(map[string]string, len(inputs))
	for _, in := range inputs {
		key := strings.TrimSpace(in.Key)
		if key == "" {
			if strings.TrimSpace(in.Value) != "" {
				return nil, fmt.Errorf("%w: env var name is required when value is set", ErrInvalidInput)
			}
			continue
		}
		incoming[key] = in.Value
	}

	var result []db.SiteEnvVar
	for key, value := range incoming {
		ev, err := q.UpsertSiteEnvVar(ctx, db.UpsertSiteEnvVarParams{
			SiteID: siteID,
			Key:    key,
			Value:  value,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert env var %s: %w", key, err)
		}
		result = append(result, ev)
	}

	for _, ex := range existing {
		if _, ok := incoming[ex.Key]; ok {
			continue
		}
		if err := q.DeleteSiteEnvVar(ctx, db.DeleteSiteEnvVarParams{
			SiteID: siteID,
			Key:    ex.Key,
		}); err != nil {
			return nil, fmt.Errorf("delete env var %s: %w", ex.Key, err)
		}
	}

	return result, nil
}

func validateCreate(req CreateSiteRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	siteType := NormalizeSiteType(req.SiteType)
	if IsWebSite(siteType) && strings.TrimSpace(req.PrimaryURL) == "" {
		return fmt.Errorf("%w: primary_url is required", ErrInvalidInput)
	}
	if err := validateHealthCheckPath(req.HealthCheckPath); err != nil {
		return err
	}
	if req.Slug != "" && !slugPattern.MatchString(strings.ToLower(req.Slug)) {
		return fmt.Errorf("%w: invalid slug", ErrInvalidInput)
	}
	return nil
}

func slugify(name string) string {
	s := strings.ToLower(name)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "site"
	}
	if len(s) > 63 {
		s = s[:63]
	}
	return s
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

func defaultStr(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func defaultInt32(v, d int32) int32 {
	if v == 0 {
		return d
	}
	return v
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
