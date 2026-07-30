package sites

import (
	"github.com/ebash/barn/backend/internal/db"
)

func toSiteResponse(site db.Site, domains []db.SiteDomain, envVars []db.SiteEnvVar) SiteResponse {
	resp := SiteResponse{
		ID:              site.ID,
		Name:            site.Name,
		Slug:            site.Slug,
		SiteType:        site.SiteType,
		PrimaryURL:      site.PrimaryUrl,
		GitRepoURL:      site.GitRepoUrl,
		GitBranch:       site.GitBranch,
		DockerfilePath:  site.DockerfilePath,
		BuildContext:    site.BuildContext,
		ContainerPort:   site.ContainerPort,
		NginxSSLEnabled: site.NginxSslEnabled,
		NginxForceHTTPS:    site.NginxForceHttps,
		DockerVolumeMounts: volumeLinesFromText(site.DockerVolumeMounts),
		DockerNamedVolumes: volumeLinesFromText(site.DockerNamedVolumes),
		DockerNetworkHost:  site.DockerNetworkHost,
		HealthCheckPath:    site.HealthCheckPath,
		Status:             site.Status,
		CreatedAt:       site.CreatedAt,
		UpdatedAt:       site.UpdatedAt,
		Domains:         make([]DomainResponse, 0, len(domains)),
		EnvVars:         make([]EnvVarResponse, 0, len(envVars)),
	}

	if site.HostPort.Valid {
		p := site.HostPort.Int32
		resp.HostPort = &p
	}

	for _, d := range domains {
		resp.Domains = append(resp.Domains, DomainResponse{
			ID:        d.ID,
			Domain:    d.Domain,
			IsPrimary: d.IsPrimary,
		})
	}
	for _, e := range envVars {
		resp.EnvVars = append(resp.EnvVars, EnvVarResponse{
			Key:   e.Key,
			Value: e.Value,
		})
	}

	return resp
}

func toListItem(site db.Site) SiteListItem {
	return SiteListItem{
		ID:         site.ID,
		Name:       site.Name,
		Slug:       site.Slug,
		SiteType:   site.SiteType,
		PrimaryURL: site.PrimaryUrl,
		Status:     site.Status,
		UpdatedAt:  site.UpdatedAt,
	}
}
