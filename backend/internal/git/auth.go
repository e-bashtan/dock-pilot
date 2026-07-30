package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneOptions configures a shallow clone.
type CloneOptions struct {
	RepoURL   string
	Branch    string
	Dest      string
	GitToken  string // HTTPS: PAT / fine-grained token (secret GIT_TOKEN, GITHUB_TOKEN, …)
	GitSSHKey string // SSH deploy key PEM (secret GIT_SSH_KEY)
}

// Clone shallow-clones a repository branch into dest (dest is created).
func Clone(ctx context.Context, opts CloneOptions) error {
	repoURL := strings.TrimSpace(opts.RepoURL)
	if repoURL == "" {
		return fmt.Errorf("git repo url is empty")
	}
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		branch = "main"
	}
	dest := opts.Dest

	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("clean dest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	cloneURL := normalizeRepoURL(repoURL)
	var sshKeyFile string
	var cleanup []func()
	defer func() {
		for _, fn := range cleanup {
			fn()
		}
	}()

	cmdEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if key := strings.TrimSpace(opts.GitSSHKey); key != "" && isSSHRepoURL(repoURL) {
		f, err := writeTempSSHKey(key)
		if err != nil {
			return err
		}
		sshKeyFile = f
		cleanup = append(cleanup, func() { _ = os.Remove(f) })
		cmdEnv = append(cmdEnv,
			"GIT_SSH_COMMAND=ssh -i "+quoteShell(f)+
				" -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes",
		)
	} else if token := strings.TrimSpace(opts.GitToken); token != "" {
		if !isSSHRepoURL(repoURL) {
			askpass, err := writeGitAskpass(token)
			if err != nil {
				return err
			}
			cleanup = append(cleanup, func() { _ = os.Remove(askpass) })
			cmdEnv = append(cmdEnv, "GIT_ASKPASS="+askpass, "SSH_ASKPASS="+askpass)
		} else {
			injected, err := injectHTTPToken(cloneURL, token)
			if err != nil {
				return fmt.Errorf("git auth url: %w", err)
			}
			cloneURL = injected
		}
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, cloneURL, dest)
	cmd.Env = cmdEnv
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", formatCloneError(err, opts), sanitizeOutput(string(out), opts.GitToken, sshKeyFile))
	}
	return nil
}

func normalizeRepoURL(raw string) string {
	return strings.TrimSuffix(strings.TrimSpace(raw), "/")
}

func isGitHubHTTPS(raw string) bool {
	u := strings.ToLower(normalizeRepoURL(raw))
	return strings.HasPrefix(u, "https://github.com/") || strings.HasPrefix(u, "http://github.com/")
}

// writeGitAskpass creates a GIT_ASKPASS helper (clean URL clone, no token in logs).
func writeGitAskpass(token string) (string, error) {
	script := "#!/bin/sh\ncase \"$1\" in\n*Username*) printf '%s' 'x-access-token';;\n*Password*) printf '%s' '" +
		strings.ReplaceAll(token, "'", "'\\''") + "';;\nesac\n"
	f, err := os.CreateTemp("", "barn-git-askpass-*")
	if err != nil {
		return "", fmt.Errorf("temp askpass: %w", err)
	}
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Chmod(0o700); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func formatCloneError(err error, opts CloneOptions) error {
	msg := err.Error()
	out := msg
	if strings.Contains(out, "403") || strings.Contains(out, "Write access to repository not granted") {
		if opts.GitToken != "" {
			return fmt.Errorf("git clone auth failed (403): check PAT has access to this repo — fine-grained: add repo + Contents read; classic: repo scope")
		}
		return fmt.Errorf("git clone auth failed (403): add secret GIT_TOKEN with a GitHub PAT that can read this repository")
	}
	if strings.Contains(out, "401") || strings.Contains(out, "Authentication failed") {
		return fmt.Errorf("git clone auth failed: invalid or expired GIT_TOKEN")
	}
	return fmt.Errorf("git clone: %w", err)
}

func isSSHRepoURL(raw string) bool {
	u := strings.TrimSpace(strings.ToLower(raw))
	return strings.HasPrefix(u, "git@") || strings.HasPrefix(u, "ssh://")
}

func injectHTTPToken(rawURL, token string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return rawURL, fmt.Errorf("HTTPS token auth requires http(s) URL, got %q", u.Scheme)
	}
	// GitHub PAT / fine-grained token
	u.User = url.UserPassword("x-access-token", token)
	return u.String(), nil
}

func writeTempSSHKey(pem string) (string, error) {
	f, err := os.CreateTemp("", "barn-git-key-*")
	if err != nil {
		return "", fmt.Errorf("temp ssh key: %w", err)
	}
	if _, err := f.WriteString(strings.TrimSpace(pem) + "\n"); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func quoteShell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func sanitizeOutput(out, token, sshKeyPath string) string {
	s := out
	if token != "" {
		s = strings.ReplaceAll(s, token, "***")
	}
	if sshKeyPath != "" {
		s = strings.ReplaceAll(s, sshKeyPath, "***")
	}
	return strings.TrimSpace(s)
}

// TokenFromSecrets picks a GitHub/Git HTTPS token from decrypted site secrets.
func TokenFromSecrets(secrets map[string]string) string {
	return tokenFromMap(secrets)
}

// TokenFromEnv picks git token from site environment variables.
func TokenFromEnv(env map[string]string) string {
	return tokenFromMap(env)
}

func tokenFromMap(m map[string]string) string {
	for _, key := range []string{"GIT_TOKEN", "GITHUB_TOKEN", "GITHUB_PAT"} {
		if v := strings.TrimSpace(m[key]); v != "" {
			return v
		}
	}
	return ""
}

// AuthMode describes how git clone will authenticate.
func AuthMode(opts CloneOptions) string {
	if strings.TrimSpace(opts.GitSSHKey) != "" && isSSHRepoURL(opts.RepoURL) {
		return "SSH deploy key (GIT_SSH_KEY)"
	}
	if strings.TrimSpace(opts.GitToken) != "" {
		return "HTTPS token (GIT_TOKEN / GITHUB_TOKEN)"
	}
	return "none (public repo only)"
}

// SSHKeyFromSecrets picks an SSH deploy key from decrypted site secrets.
func SSHKeyFromSecrets(secrets map[string]string) string {
	for _, key := range []string{"GIT_SSH_KEY", "GITHUB_DEPLOY_KEY"} {
		if v := strings.TrimSpace(secrets[key]); v != "" {
			return v
		}
	}
	return ""
}
