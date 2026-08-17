package sites

import (
	"time"

	"github.com/google/uuid"
)

type DomainInput struct {
	Domain    string `json:"domain"`
	IsPrimary bool   `json:"is_primary"`
}

type EnvVarInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CreateSiteRequest struct {
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	SiteType        string          `json:"site_type"`
	PrimaryURL      string          `json:"primary_url"`
	GitRepoURL      string          `json:"git_repo_url"`
	GitBranch       string          `json:"git_branch"`
	DockerfilePath  string          `json:"dockerfile_path"`
	BuildContext    string          `json:"build_context"`
	ContainerPort   int32           `json:"container_port"`
	NginxSSLEnabled bool            `json:"nginx_ssl_enabled"`
	NginxForceHTTPS    bool          `json:"nginx_force_https"`
	DockerVolumeMounts []string      `json:"docker_volume_mounts"`
	DockerNamedVolumes []string      `json:"docker_named_volumes"`
	DockerNetworkHost  bool          `json:"docker_network_host"`
	HealthCheckPath    string        `json:"health_check_path"`
	Domains            []DomainInput `json:"domains"`
	EnvVars            []EnvVarInput `json:"env_vars"`
}

type UpdateSiteRequest struct {
	Name            *string         `json:"name,omitempty"`
	PrimaryURL      *string         `json:"primary_url,omitempty"`
	GitRepoURL      *string         `json:"git_repo_url,omitempty"`
	GitBranch       *string         `json:"git_branch,omitempty"`
	DockerfilePath  *string         `json:"dockerfile_path,omitempty"`
	BuildContext    *string         `json:"build_context,omitempty"`
	ContainerPort   *int32          `json:"container_port,omitempty"`
	HostPort        *int32          `json:"host_port,omitempty"`
	NginxSSLEnabled *bool           `json:"nginx_ssl_enabled,omitempty"`
	NginxForceHTTPS *bool           `json:"nginx_force_https,omitempty"`
	Status             *string       `json:"status,omitempty"`
	DockerVolumeMounts *[]string     `json:"docker_volume_mounts,omitempty"`
	DockerNamedVolumes *[]string     `json:"docker_named_volumes,omitempty"`
	DockerNetworkHost  *bool         `json:"docker_network_host,omitempty"`
	HealthCheckPath    *string       `json:"health_check_path,omitempty"`
	Domains            []DomainInput `json:"domains,omitempty"`
	EnvVars            []EnvVarInput `json:"env_vars,omitempty"`
}

type DomainResponse struct {
	ID        uuid.UUID `json:"id"`
	Domain    string    `json:"domain"`
	IsPrimary bool      `json:"is_primary"`
}

type EnvVarResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SiteResponse struct {
	ID              uuid.UUID        `json:"id"`
	Name            string           `json:"name"`
	Slug            string           `json:"slug"`
	SiteType        string           `json:"site_type"`
	PrimaryURL      string           `json:"primary_url"`
	GitRepoURL      string           `json:"git_repo_url"`
	GitBranch       string           `json:"git_branch"`
	DockerfilePath  string           `json:"dockerfile_path"`
	BuildContext    string           `json:"build_context"`
	ContainerPort   int32            `json:"container_port"`
	HostPort        *int32           `json:"host_port,omitempty"`
	NginxSSLEnabled bool             `json:"nginx_ssl_enabled"`
	NginxForceHTTPS    bool             `json:"nginx_force_https"`
	DockerVolumeMounts []string         `json:"docker_volume_mounts"`
	DockerNamedVolumes []string         `json:"docker_named_volumes"`
	DockerNetworkHost  bool             `json:"docker_network_host"`
	HealthCheckPath    string           `json:"health_check_path"`
	Status             string           `json:"status"`
	Domains         []DomainResponse `json:"domains"`
	EnvVars         []EnvVarResponse `json:"env_vars"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type SiteListItem struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Slug           string     `json:"slug"`
	SiteType       string     `json:"site_type"`
	PrimaryURL     string     `json:"primary_url"`
	Status         string     `json:"status"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LastDeployedAt *time.Time `json:"last_deployed_at,omitempty"`
}
