package hostexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes commands on the host (optionally via chroot for containerized API).
type Runner struct {
	HostRoot string
}

func New(hostRoot string) *Runner {
	return &Runner{HostRoot: strings.TrimSpace(hostRoot)}
}

// UsesChroot reports whether commands run in the host filesystem root (containerized API).
func (r *Runner) UsesChroot() bool {
	return r.HostRoot != ""
}

// ChrootPath returns path as seen from API container (HostRoot + absolute host path).
func (r *Runner) ChrootPath(hostAbsPath string) string {
	if r.HostRoot == "" {
		return hostAbsPath
	}
	return filepath.Join(r.HostRoot, strings.TrimPrefix(hostAbsPath, "/"))
}

func (r *Runner) Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	if r.HostRoot == "" {
		return exec.CommandContext(ctx, name, args...)
	}
	chrootArgs := append([]string{r.HostRoot, name}, args...)
	return exec.CommandContext(ctx, "chroot", chrootArgs...)
}

func (r *Runner) Run(ctx context.Context, name string, args ...string) error {
	cmd := r.Command(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (r *Runner) RunCombined(ctx context.Context, name string, args ...string) (string, error) {
	cmd := r.Command(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// RunShell runs a shell script on the host (via chroot when HostRoot is set).
func (r *Runner) RunShell(ctx context.Context, script string) error {
	var cmd *exec.Cmd
	if r.HostRoot == "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", script)
	} else {
		cmd = exec.CommandContext(ctx, "chroot", r.HostRoot, "sh", "-c", script)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(stderr.String())
		if out == "" {
			return fmt.Errorf("sh -c %q: %w", script, err)
		}
		return fmt.Errorf("sh -c %q: %w: %s", script, err, out)
	}
	return nil
}

func (r *Runner) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (r *Runner) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (r *Runner) Symlink(oldname, newname string) error {
	if _, err := os.Lstat(newname); err == nil {
		if err := os.Remove(newname); err != nil {
			return err
		}
	}
	return os.Symlink(oldname, newname)
}

func (r *Runner) Remove(path string) error {
	return os.Remove(path)
}

// WithChrootDNS replaces $HostRoot/etc/resolv.conf for the duration of fn.
// Host systemd-resolved (127.0.0.53) does not work inside chroot from a container; certbot then cannot reach Let's Encrypt.
func (r *Runner) WithChrootDNS(ctx context.Context, fn func(context.Context) error) error {
	if r.HostRoot == "" {
		return fn(ctx)
	}

	resolvPath := filepath.Join(r.HostRoot, "etc/resolv.conf")
	backupPath := resolvPath + ".dock-pilot.bak"

	orig, hadOrig := os.ReadFile(resolvPath)
	if hadOrig == nil {
		if err := os.WriteFile(backupPath, orig, 0o644); err != nil {
			return fmt.Errorf("backup resolv.conf: %w", err)
		}
		defer func() {
			_ = os.WriteFile(resolvPath, orig, 0o644)
			_ = os.Remove(backupPath)
		}()
	}

	_ = os.Remove(resolvPath)
	if err := os.MkdirAll(filepath.Dir(resolvPath), 0o755); err != nil {
		return fmt.Errorf("mkdir etc: %w", err)
	}
	if err := os.WriteFile(resolvPath, publicResolverConfig(), 0o644); err != nil {
		return fmt.Errorf("write chroot resolv.conf: %w", err)
	}

	return fn(ctx)
}

func publicResolverConfig() []byte {
	return []byte("nameserver 8.8.8.8\nnameserver 1.1.1.1\n")
}

// RunHostCombined runs a command in the API container (never chroot). Used for nsenter.
func (r *Runner) RunHostCombined(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// NsenterSh runs a shell script in the host init namespaces.
// Avoids -i (IPC): many hosts deny /proc/1/ns/ipc even with SYS_ADMIN.
func (r *Runner) NsenterSh(ctx context.Context, script string) (string, error) {
	attempts := [][]string{
		{"-t", "1", "-m", "-u", "-n", "-p", "--", "sh", "-c", script},
		{"-t", "1", "-m", "-n", "-p", "--", "sh", "-c", script},
		{"-t", "1", "-m", "-u", "-n", "--", "sh", "-c", script},
		{"-t", "1", "-m", "--", "sh", "-c", script},
	}
	var lastErr error
	var lastOut string
	for _, args := range attempts {
		out, err := r.RunHostCombined(ctx, "nsenter", args...)
		if err == nil {
			return out, nil
		}
		lastErr = err
		lastOut = out
		// Permission on a specific ns → try fewer namespaces.
		if !strings.Contains(err.Error(), "Permission denied") &&
			!strings.Contains(err.Error(), "Operation not permitted") {
			break
		}
	}
	if lastOut != "" {
		return lastOut, lastErr
	}
	return "", lastErr
}

// RunShellCombined runs a shell script via chroot when HostRoot is set.
func (r *Runner) RunShellCombined(ctx context.Context, script string) (string, error) {
	var cmd *exec.Cmd
	if r.HostRoot == "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", script)
	} else {
		cmd = exec.CommandContext(ctx, "chroot", r.HostRoot, "sh", "-c", script)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("sh -c: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
