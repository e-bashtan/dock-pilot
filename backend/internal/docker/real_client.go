package docker

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
)

type RealClient struct {
	cli          *client.Client
	logger       *slog.Logger
	portStart    int
	portEnd      int
}

type RealConfig struct {
	Host       string
	PortStart  int
	PortEnd    int
}

func NewRealClient(cfg RealConfig, logger *slog.Logger) (*RealClient, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.Host != "" {
		opts = []client.Opt{client.WithHost(cfg.Host), client.WithAPIVersionNegotiation()}
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	start, end := cfg.PortStart, cfg.PortEnd
	if start == 0 {
		start = 18080
	}
	if end == 0 {
		end = 18999
	}
	return &RealClient{cli: cli, logger: logger, portStart: start, portEnd: end}, nil
}

func (c *RealClient) Build(ctx context.Context, opts BuildOptions) error {
	if opts.ContextDir == "" {
		return fmt.Errorf("build context dir is empty")
	}
	dockerfile := opts.DockerfilePath
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	contextPath := opts.ContextDir
	if opts.BuildContext != "" && opts.BuildContext != "." {
		contextPath = fmt.Sprintf("%s/%s", opts.ContextDir, trimSlash(opts.BuildContext))
	}

	tar, err := archive.TarWithOptions(contextPath, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("tar context: %w", err)
	}
	defer tar.Close()

	imageTag := normalizeImageRef(opts.ImageTag)

	c.logger.InfoContext(ctx, "docker build",
		"repo", opts.RepoURL,
		"branch", opts.Branch,
		"context", contextPath,
		"dockerfile", dockerfile,
		"tag", imageTag,
	)

	resp, err := c.cli.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: dockerfile,
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("image build: %w", err)
	}
	defer resp.Body.Close()

	buildLog, buildErr := consumeBuildOutput(resp.Body, opts.OnOutput)
	if buildErr != nil {
		return buildError(imageTag, buildErr, buildLog)
	}

	if _, _, err := c.cli.ImageInspectWithRaw(ctx, imageTag); err != nil {
		return buildError(imageTag, fmt.Errorf("image not created (inspect failed): %w", err), buildLog)
	}

	c.logger.InfoContext(ctx, "docker build finished", "tag", imageTag)
	return nil
}

func (c *RealClient) Run(ctx context.Context, opts RunOptions) (string, error) {
	containerName := opts.ContainerName
	if containerName == "" {
		return "", fmt.Errorf("container name is required")
	}

	for _, name := range uniqueContainerNames(append(opts.StopNames, containerName)) {
		_ = c.stopContainer(ctx, name)
	}

	if err := c.ensureNamedVolumes(ctx, opts.EnsureVolumes); err != nil {
		return "", err
	}

	config := &container.Config{
		Image: opts.ImageTag,
		Env:   envMapToSlice(opts.Env),
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		Mounts:        dockerMounts(opts.Mounts),
	}

	if opts.NetworkHost {
		hostConfig.NetworkMode = "host"
		c.logger.InfoContext(ctx, "docker run (network_mode=host)",
			"image", opts.ImageTag,
			"name", containerName,
		)
	} else if opts.PublishPorts {
		hostPort := strconv.Itoa(opts.HostPort)
		containerPort := strconv.Itoa(opts.ContainerPort)
		portKey := nat.Port(containerPort + "/tcp")
		config.ExposedPorts = nat.PortSet{portKey: struct{}{}}
		hostConfig.PortBindings = nat.PortMap{
			portKey: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}},
		}
		c.logger.InfoContext(ctx, "docker run",
			"image", opts.ImageTag,
			"name", containerName,
			"host_port", opts.HostPort,
			"container_port", opts.ContainerPort,
		)
	} else {
		c.logger.InfoContext(ctx, "docker run (no ports)",
			"image", opts.ImageTag,
			"name", containerName,
		)
	}

	created, err := c.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		_ = c.cli.ContainerRemove(ctx, created.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("container start: %w", err)
	}
	return created.ID, nil
}

func (c *RealClient) Stop(ctx context.Context, containerNames ...string) error {
	for _, name := range uniqueContainerNames(containerNames) {
		if err := c.stopContainer(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func uniqueContainerNames(names []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, n := range names {
		n = SanitizeContainerName(n)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func (c *RealClient) ensureNamedVolumes(ctx context.Context, names []string) error {
	for _, name := range names {
		if name == "" {
			continue
		}
		_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
		if err == nil {
			c.logger.InfoContext(ctx, "docker volume created", "name", name)
			continue
		}
		if _, inspectErr := c.cli.VolumeInspect(ctx, name); inspectErr != nil {
			return fmt.Errorf("create volume %s: %w", name, err)
		}
		c.logger.InfoContext(ctx, "docker volume exists", "name", name)
	}
	return nil
}

func dockerMounts(mounts []Mount) []mount.Mount {
	if len(mounts) == 0 {
		return nil
	}
	out := make([]mount.Mount, 0, len(mounts))
	for _, m := range mounts {
		t := mount.TypeBind
		if m.Type == "volume" {
			t = mount.TypeVolume
		}
		out = append(out, mount.Mount{
			Type:     t,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}
	return out
}

func (c *RealClient) stopContainer(ctx context.Context, name string) error {
	_, err := c.cli.ContainerInspect(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	timeout := 10
	_ = c.cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
	if err := c.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *RealClient) AllocatePort(ctx context.Context) (int, error) {
	for port := c.portStart; port <= c.portEnd; port++ {
		if !c.hostPortAvailable(ctx, port) {
			continue
		}
		c.logger.InfoContext(ctx, "allocated host port", "port", port)
		return port, nil
	}
	return 0, fmt.Errorf("no free port in range %d-%d", c.portStart, c.portEnd)
}

// HostPortAvailable reports whether the host port can be bound (not held by Docker).
func (c *RealClient) HostPortAvailable(ctx context.Context, port int) bool {
	return c.hostPortAvailable(ctx, port)
}

func (c *RealClient) hostPortAvailable(ctx context.Context, port int) bool {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for _, ctr := range containers {
			for _, p := range ctr.Ports {
				if int(p.PublicPort) == port {
					return false
				}
			}
		}
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func (c *RealClient) Close() error {
	return c.cli.Close()
}

func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func trimSlash(s string) string {
	s = trimPrefixSlash(s)
	return trimSuffixSlash(s)
}

func trimPrefixSlash(s string) string {
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return s
}

func trimSuffixSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// Ensure RealClient implements Client at compile time.
var _ Client = (*RealClient)(nil)

// Ping verifies Docker daemon connectivity.
func (c *RealClient) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}
