package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

func (c *RealClient) Exec(ctx context.Context, opts ExecOptions, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	name := SanitizeContainerName(opts.ContainerName)
	if name == "" {
		return -1, fmt.Errorf("container name is empty")
	}
	if len(opts.Cmd) == 0 {
		return -1, fmt.Errorf("exec command is empty")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	attachStdin := stdin != nil
	createResp, err := c.cli.ContainerExecCreate(ctx, name, types.ExecConfig{
		AttachStdin:  attachStdin,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          opts.Cmd,
		Env:          opts.Env,
		User:         opts.User,
	})
	if err != nil {
		return -1, fmt.Errorf("exec create: %w", err)
	}

	hijacked, err := c.cli.ContainerExecAttach(ctx, createResp.ID, types.ExecStartCheck{})
	if err != nil {
		return -1, fmt.Errorf("exec attach: %w", err)
	}
	defer hijacked.Close()

	errCh := make(chan error, 1)
	go func() {
		if !attachStdin {
			errCh <- nil
			return
		}
		_, copyErr := io.Copy(hijacked.Conn, stdin)
		_ = hijacked.CloseWrite()
		errCh <- copyErr
	}()

	if _, demuxErr := stdcopy.StdCopy(stdout, stderr, hijacked.Reader); demuxErr != nil && demuxErr != io.EOF {
		return -1, fmt.Errorf("exec stream: %w", demuxErr)
	}
	if copyErr := <-errCh; copyErr != nil && copyErr != io.EOF {
		return -1, fmt.Errorf("exec stdin: %w", copyErr)
	}

	inspect, err := c.cli.ContainerExecInspect(ctx, createResp.ID)
	if err != nil {
		return -1, fmt.Errorf("exec inspect: %w", err)
	}
	return inspect.ExitCode, nil
}
