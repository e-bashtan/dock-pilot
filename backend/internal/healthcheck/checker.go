package healthcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
)

const (
	httpProbeTimeout    = 20 * time.Second
	httpProbeAttempts   = 3
	httpProbeRetryDelay = 2 * time.Second
)

// ContainerInfo is Docker runtime state for a site.
type ContainerInfo struct {
	Found     bool   `json:"found"`
	Running   bool   `json:"running"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Container string `json:"container,omitempty"`
}

// HTTPInfo is an HTTP probe result (websites only).
type HTTPInfo struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

// Result is the health snapshot for one site.
type Result struct {
	SiteID    uuid.UUID      `json:"site_id"`
	SiteType  string         `json:"site_type"`
	Overall   string         `json:"overall"` // healthy, degraded, unhealthy, unknown
	Message   string         `json:"message"`
	Container *ContainerInfo `json:"container,omitempty"`
	HTTP      *HTTPInfo      `json:"http,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
}

type Checker struct {
	docker docker.Client
	http   *http.Client
}

func NewChecker(dockerClient docker.Client) *Checker {
	return &Checker{
		docker: dockerClient,
		http: &http.Client{
			Timeout: httpProbeTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (c *Checker) Check(ctx context.Context, site db.Site) Result {
	now := time.Now().UTC()
	res := Result{
		SiteID:    site.ID,
		SiteType:  site.SiteType,
		Overall:   "unknown",
		Message:   "Not checked",
		CheckedAt: now,
	}

	names := docker.ContainerNamesForSite(site.Slug, site.PrimaryUrl)
	st, err := c.docker.InspectContainer(ctx, names...)
	if err != nil {
		res.Overall = "unknown"
		res.Message = "Docker inspect failed: " + err.Error()
		return res
	}

	res.Container = &ContainerInfo{
		Found:     st.Found,
		Running:   st.Running,
		State:     st.State,
		Health:    st.Health,
		Container: st.Container,
	}

	if isTelegramBot(site.SiteType) {
		res.Overall, res.Message = telegramOverall(*res.Container)
		return res
	}

	httpInfo := c.probeHTTP(ctx, site)
	if httpInfo != nil {
		res.HTTP = httpInfo
	}
	res.Overall, res.Message = webOverall(*res.Container, res.HTTP)
	return res
}

func telegramOverall(c ContainerInfo) (overall, message string) {
	if !c.Found {
		return "unhealthy", "Container not found — deploy the bot"
	}
	if !docker.IsContainerRunning(docker.ContainerStatus{
		Found: c.Found, Running: c.Running, State: c.State, Health: c.Health,
	}) {
		return "unhealthy", fmt.Sprintf("Container not running (state: %s)", c.State)
	}
	h := strings.ToLower(c.Health)
	if h == "unhealthy" {
		return "unhealthy", "Docker HEALTHCHECK: unhealthy"
	}
	if h == "starting" {
		return "degraded", "Docker HEALTHCHECK: starting"
	}
	if h == "healthy" {
		return "healthy", "Container running, HEALTHCHECK healthy"
	}
	return "healthy", "Container running"
}

func webOverall(c ContainerInfo, httpInfo *HTTPInfo) (overall, message string) {
	if !c.Found {
		return "unhealthy", "Container not found — deploy the site"
	}
	if !docker.IsContainerRunning(docker.ContainerStatus{
		Found: c.Found, Running: c.Running, State: c.State, Health: c.Health,
	}) {
		return "unhealthy", fmt.Sprintf("Container not running (state: %s)", c.State)
	}
	if !docker.DockerHealthOK(docker.ContainerStatus{Health: c.Health}) {
		return "unhealthy", "Docker HEALTHCHECK: " + c.Health
	}
	if httpInfo == nil {
		return "degraded", "Container running; HTTP check skipped"
	}
	if httpInfo.OK {
		if httpInfo.StatusCode > 0 {
			return "healthy", fmt.Sprintf("Container running, HTTP %d", httpInfo.StatusCode)
		}
		return "healthy", "Container running, HTTP OK"
	}
	if httpInfo.Error != "" {
		return "degraded", "Container running; HTTP: " + httpInfo.Error
	}
	return "degraded", fmt.Sprintf("Container running; HTTP %d", httpInfo.StatusCode)
}

func (c *Checker) probeHTTP(ctx context.Context, site db.Site) *HTTPInfo {
	paths := healthCheckPaths(site.HealthCheckPath)
	var last *HTTPInfo
	for i, path := range paths {
		for _, target := range probeTargets(site, path) {
			info := c.doHTTPWithRetries(ctx, target.URL, target.Host)
			if info.OK {
				return info
			}
			if info.Error == "" && info.StatusCode >= 200 && info.StatusCode < 400 {
				info.OK = true
				return info
			}
			last = info
		}
		if i == len(paths)-1 {
			return last
		}
	}
	return last
}

type httpProbe struct {
	URL  string
	Host string
}

func probeTargets(site db.Site, path string) []httpProbe {
	var out []httpProbe
	host := primaryHost(site.PrimaryUrl)
	port := upstreamPort(site)

	if port > 0 {
		out = append(out, httpProbe{
			URL:  fmt.Sprintf("http://127.0.0.1:%d%s", port, path),
			Host: host,
		})
	}
	if host != "" {
		out = append(out, httpProbe{
			URL:  "http://127.0.0.1" + path,
			Host: host,
		})
	}
	if base := siteURL(site.PrimaryUrl); base != "" {
		out = append(out, httpProbe{
			URL: strings.TrimSuffix(base, "/") + path,
		})
	}
	return out
}

func upstreamPort(site db.Site) int {
	if site.DockerNetworkHost {
		return int(site.ContainerPort)
	}
	if site.HostPort.Valid && site.HostPort.Int32 > 0 {
		return int(site.HostPort.Int32)
	}
	if site.ContainerPort > 0 {
		return int(site.ContainerPort)
	}
	return 0
}

func primaryHost(raw string) string {
	u := strings.TrimSpace(raw)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	if i := strings.Index(u, ":"); i >= 0 {
		u = u[:i]
	}
	return u
}

func healthCheckPaths(custom string) []string {
	custom = strings.TrimSpace(custom)
	if custom != "" {
		if !strings.HasPrefix(custom, "/") {
			custom = "/" + custom
		}
		return []string{custom}
	}
	return []string{"/health", "/"}
}

func (c *Checker) doHTTPWithRetries(ctx context.Context, url, host string) *HTTPInfo {
	var last *HTTPInfo
	for attempt := 1; attempt <= httpProbeAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return &HTTPInfo{URL: url, Error: err.Error()}
		}

		info := c.doHTTP(ctx, url, host)
		if info.OK {
			return info
		}
		if info.Error == "" && info.StatusCode >= 200 && info.StatusCode < 400 {
			info.OK = true
			return info
		}
		if !shouldRetryHTTP(info) || attempt == httpProbeAttempts {
			return info
		}
		last = info

		select {
		case <-ctx.Done():
			return last
		case <-time.After(httpProbeRetryDelay):
		}
	}
	return last
}

func shouldRetryHTTP(info *HTTPInfo) bool {
	if info.Error != "" {
		return true
	}
	return info.StatusCode >= 500
}

func (c *Checker) doHTTP(ctx context.Context, url, host string) *HTTPInfo {
	info := &HTTPInfo{URL: url}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	req.Header.Set("User-Agent", "Barn-HealthCheck/1.0")
	if host != "" {
		req.Host = host
		req.Header.Set("Host", host)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	info.StatusCode = resp.StatusCode
	info.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
	return info
}

func isTelegramBot(siteType string) bool {
	return strings.TrimSpace(strings.ToLower(siteType)) == "telegram_bot"
}

func siteURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" || strings.HasPrefix(u, "telegram://") || strings.HasPrefix(u, "bot://") {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	return u
}
