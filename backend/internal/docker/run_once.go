package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// RunOnceOptions describes a one-shot container (like `docker run --rm`).
type RunOnceOptions struct {
	Image       string
	Cmd         []string
	Env         []string
	NetworkHost bool
}

// RunOnce creates a temporary container, streams I/O, waits for exit, and removes it.
func (c *RealClient) RunOnce(ctx context.Context, opts RunOnceOptions, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if opts.Image == "" {
		return -1, fmt.Errorf("run once: image is empty")
	}
	if len(opts.Cmd) == 0 {
		return -1, fmt.Errorf("run once: command is empty")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cfg := &container.Config{
		Image:        opts.Image,
		Cmd:          opts.Cmd,
		Env:          opts.Env,
		AttachStdin:  stdin != nil,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    stdin != nil,
		StdinOnce:    stdin != nil,
	}
	hostCfg := &container.HostConfig{
		AutoRemove: false, // remove explicitly after wait so we can read exit code
	}
	if opts.NetworkHost {
		hostCfg.NetworkMode = "host"
	}

	created, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "")
	if err != nil {
		return -1, fmt.Errorf("run once create: %w", err)
	}
	id := created.ID
	defer func() {
		_ = c.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	}()

	attach, err := c.cli.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  stdin != nil,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return -1, fmt.Errorf("run once attach: %w", err)
	}
	defer attach.Close()

	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return -1, fmt.Errorf("run once start: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		if stdin == nil {
			errCh <- nil
			return
		}
		_, copyErr := io.Copy(attach.Conn, stdin)
		_ = attach.CloseWrite()
		errCh <- copyErr
	}()

	if _, demuxErr := stdcopy.StdCopy(stdout, stderr, attach.Reader); demuxErr != nil && demuxErr != io.EOF {
		return -1, fmt.Errorf("run once stream: %w", demuxErr)
	}
	if copyErr := <-errCh; copyErr != nil && copyErr != io.EOF {
		return -1, fmt.Errorf("run once stdin: %w", copyErr)
	}

	statusCh, waitErrCh := c.cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-waitErrCh:
		if err != nil {
			return -1, fmt.Errorf("run once wait: %w", err)
		}
		return -1, fmt.Errorf("run once wait: channel closed")
	case st := <-statusCh:
		if st.Error != nil {
			return int(st.StatusCode), fmt.Errorf("run once: %s", st.Error.Message)
		}
		return int(st.StatusCode), nil
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

// ContainerImage returns the image ref for a running/stopped container.
func (c *RealClient) ContainerImage(ctx context.Context, name string) (string, error) {
	name = SanitizeContainerName(name)
	if name == "" {
		return "", fmt.Errorf("container name is empty")
	}
	info, err := c.cli.ContainerInspect(ctx, name)
	if err != nil {
		return "", err
	}
	return info.Config.Image, nil
}

// ImageExists reports whether a local image ref exists.
func (c *RealClient) ImageExists(ctx context.Context, ref string) bool {
	ref = normalizeImageRef(ref)
	if ref == "" {
		return false
	}
	_, _, err := c.cli.ImageInspectWithRaw(ctx, ref)
	return err == nil
}

func (s *StubClient) RunOnce(ctx context.Context, opts RunOnceOptions, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	s.logger.InfoContext(ctx, "stub docker run once", "image", opts.Image, "cmd", opts.Cmd)
	if stdout != nil {
		_, _ = io.WriteString(stdout, "-- stub dump\n")
	}
	return 0, nil
}

func (s *StubClient) ContainerImage(ctx context.Context, name string) (string, error) {
	return "postgres:16", nil
}

func (s *StubClient) ImageExists(ctx context.Context, ref string) bool {
	return true
}
