package sites

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
)

// RunSiteContainer starts (or recreates) the site container from the last built image.
func RunSiteContainer(ctx context.Context, dockerClient docker.Client, site db.Site, env map[string]string) error {
	hostNetwork := UsesHostNetwork(site)
	publishPorts := IsWebSite(site.SiteType) && !hostNetwork
	if publishPorts && !site.HostPort.Valid {
		return fmt.Errorf("host port not allocated — deploy the site first")
	}

	if publishPorts {
		if _, ok := env["PORT"]; !ok {
			env = copyEnv(env)
			env["PORT"] = strconv.Itoa(int(site.ContainerPort))
		}
	}

	mountLines := VolumeLinesFromSite(site)
	namedLines := NamedVolumeLinesFromSite(site)
	volMounts, ensureVolumes, err := docker.ParseVolumeConfig(site.Slug, mountLines, namedLines)
	if err != nil {
		return err
	}

	tag := docker.ImageTagForSlug(site.Slug)
	resolved, err := dockerClient.ResolveLocalImage(ctx, tag, docker.ImageTagsForSlug(site.Slug)...)
	if err != nil {
		return err
	}
	containerName, stopNames := docker.NamesForSite(site.Slug, site.PrimaryUrl)
	runOpts := docker.RunOptions{
		ImageTag:      resolved,
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

	if _, err := dockerClient.Run(ctx, runOpts); err != nil {
		return err
	}
	return nil
}

func copyEnv(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func containerNamesForSite(site db.Site) []string {
	return docker.ContainerNamesForSite(site.Slug, site.PrimaryUrl)
}

func inspectSiteContainer(ctx context.Context, dockerClient docker.Client, site db.Site) (docker.ContainerStatus, error) {
	return dockerClient.InspectContainer(ctx, containerNamesForSite(site)...)
}

func (s *Service) loadContainerEnv(ctx context.Context, siteID uuid.UUID) (map[string]string, error) {
	if s.secrets == nil {
		return nil, fmt.Errorf("secrets not configured")
	}
	envVars, err := s.queries.ListSiteEnvVars(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}
	secretMap, err := s.secrets.DecryptForDeploy(ctx, siteID)
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
