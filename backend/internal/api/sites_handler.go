package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/healthcheck"
	secretpkg "github.com/ebash/barn/backend/internal/secrets"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
)

type SitesHandler struct {
	sites   *sitesvc.Service
	secrets *secretpkg.Service
}

func NewSitesHandler(sites *sitesvc.Service, secrets *secretpkg.Service) *SitesHandler {
	return &SitesHandler{sites: sites, secrets: secrets}
}

type siteExportSecret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type siteExportDocument struct {
	Format             string                `json:"format"`
	FormatVersion      int                   `json:"format_version"`
	SiteType           string                `json:"site_type"`
	Name               string                `json:"name"`
	Slug               string                `json:"slug"`
	PrimaryURL         string                `json:"primary_url"`
	GitRepoURL         string                `json:"git_repo_url"`
	GitBranch          string                `json:"git_branch"`
	DockerfilePath     string                `json:"dockerfile_path"`
	BuildContext       string                `json:"build_context"`
	ContainerPort      int32                 `json:"container_port,omitempty"`
	NginxSSLEnabled    bool                  `json:"nginx_ssl_enabled"`
	NginxForceHTTPS    bool                  `json:"nginx_force_https"`
	DockerVolumeMounts []string              `json:"docker_volume_mounts"`
	DockerNamedVolumes []string              `json:"docker_named_volumes"`
	DockerNetworkHost  bool                  `json:"docker_network_host"`
	HealthCheckPath    string                `json:"health_check_path,omitempty"`
	Domains            []sitesvc.DomainInput `json:"domains"`
	EnvVars            []sitesvc.EnvVarInput `json:"env_vars"`
	Secrets            []siteExportSecret    `json:"secrets"`
	Deploy             bool                  `json:"deploy"`
}

func (h *SitesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req sitesvc.CreateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	site, err := h.sites.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, site)
}

func (h *SitesHandler) List(w http.ResponseWriter, r *http.Request) {
	sites, err := h.sites.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if sites == nil {
		sites = []sitesvc.SiteListItem{}
	}
	writeJSON(w, http.StatusOK, sites)
}

func (h *SitesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	site, err := h.sites.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, site)
}

func (h *SitesHandler) Export(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}
	site, err := h.sites.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	plainSecrets, err := h.secrets.DecryptForDeploy(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	doc := siteExportDocument{
		Format: "barn.site", FormatVersion: 1, SiteType: site.SiteType,
		Name: site.Name, Slug: site.Slug, PrimaryURL: site.PrimaryURL,
		GitRepoURL: site.GitRepoURL, GitBranch: site.GitBranch,
		DockerfilePath: site.DockerfilePath, BuildContext: site.BuildContext,
		ContainerPort: site.ContainerPort, NginxSSLEnabled: site.NginxSSLEnabled,
		NginxForceHTTPS: site.NginxForceHTTPS, DockerVolumeMounts: site.DockerVolumeMounts,
		DockerNamedVolumes: site.DockerNamedVolumes, DockerNetworkHost: site.DockerNetworkHost,
		HealthCheckPath: site.HealthCheckPath, Deploy: false,
		Domains: make([]sitesvc.DomainInput, 0, len(site.Domains)),
		EnvVars: make([]sitesvc.EnvVarInput, 0, len(site.EnvVars)),
		Secrets: make([]siteExportSecret, 0, len(plainSecrets)),
	}
	for _, domain := range site.Domains {
		doc.Domains = append(doc.Domains, sitesvc.DomainInput{Domain: domain.Domain, IsPrimary: domain.IsPrimary})
	}
	for _, env := range site.EnvVars {
		doc.EnvVars = append(doc.EnvVars, sitesvc.EnvVarInput{Key: env.Key, Value: env.Value})
	}
	keys := make([]string, 0, len(plainSecrets))
	for key := range plainSecrets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		doc.Secrets = append(doc.Secrets, siteExportSecret{Key: key, Value: plainSecrets[key]})
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="barn-%s.json"`, site.Slug))
	writeJSON(w, http.StatusOK, doc)
}

func (h *SitesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	var req sitesvc.UpdateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	site, err := h.sites.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, site)
}

func (h *SitesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	if err := h.sites.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SitesHandler) Health(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}
	result, err := h.sites.Health(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SitesHandler) HealthAll(w http.ResponseWriter, r *http.Request) {
	results, err := h.sites.HealthAll(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if results == nil {
		results = []healthcheck.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *SitesHandler) StreamContainerLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming not supported"))
		return
	}

	tail := sitesvc.ParseLogTail(r.URL.Query().Get("tail"), 300)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_ = h.sites.StreamContainerLogs(r.Context(), id, tail, w, flusher)
}

func (h *SitesHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	h.containerAction(w, r, h.sites.StartContainer)
}

func (h *SitesHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	h.containerAction(w, r, h.sites.StopContainer)
}

func (h *SitesHandler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	h.containerAction(w, r, h.sites.RestartContainer)
}

func (h *SitesHandler) containerAction(
	w http.ResponseWriter,
	r *http.Request,
	fn func(context.Context, uuid.UUID) (sitesvc.ContainerActionResponse, error),
) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, sitesvc.ErrInvalidInput)
		return
	}
	resp, err := fn(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
