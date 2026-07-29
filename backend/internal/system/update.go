package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultInstallDir = "/opt/dock-pilot"
	defaultGitHubRepo = "ebasht/dock-pilot"
	upgradeStateDir   = ".upgrade"
)

var (
	ErrUpgradeBusy      = errors.New("upgrade already running")
	ErrUpgradeNotAvail  = errors.New("panel self-update is not available on this host")
	ErrUpgradeStartFail = errors.New("failed to start upgrade")
	safeVersionRe       = regexp.MustCompile(`^v?[0-9A-Za-z][0-9A-Za-z._-]*$`)
)

type UpdateInfo struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	CanUpdate       bool   `json:"can_update"`
	Reason          string `json:"reason,omitempty"`
	InstallDir      string `json:"install_dir"`
	UpgradeStatus   string `json:"upgrade_status"` // idle|running|ok|failed
	UpgradeTarget   string `json:"upgrade_target,omitempty"`
	CheckedAt       string `json:"checked_at"`
}

type UpgradeStartResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type UpgradeJobStatus struct {
	Status    string `json:"status"` // idle|running|ok|failed
	Target    string `json:"target,omitempty"`
	Log       string `json:"log"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (s *Service) installDir() string {
	if v := strings.TrimSpace(os.Getenv("DOCK_PILOT_INSTALL_DIR")); v != "" {
		return v
	}
	return defaultInstallDir
}

func (s *Service) githubRepo() string {
	if v := strings.TrimSpace(os.Getenv("DOCK_PILOT_GITHUB_REPO")); v != "" {
		return v
	}
	return defaultGitHubRepo
}

func (s *Service) hostPath(abs string) string {
	return s.host.ChrootPath(abs)
}

func (s *Service) upgradeDir() string {
	return filepath.Join(s.installDir(), upgradeStateDir)
}

func (s *Service) GetUpdateInfo(ctx context.Context) (UpdateInfo, error) {
	info := UpdateInfo{
		InstallDir:    s.installDir(),
		UpgradeStatus: "idle",
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	info.Current = s.readCurrentVersion(ctx)
	latest, latestErr := s.fetchLatestRelease(ctx)
	if latestErr == nil {
		info.Latest = latest
	}

	job := s.readUpgradeJob()
	info.UpgradeStatus = job.Status
	info.UpgradeTarget = job.Target

	canUpdate, canReason := s.canUpdate()
	info.CanUpdate = canUpdate
	if !info.CanUpdate {
		info.Reason = canReason
	} else if latestErr != nil {
		info.Reason = latestErr.Error()
	}
	info.UpdateAvailable = versionsDiffer(info.Current, info.Latest)
	if info.UpgradeStatus == "running" {
		info.CanUpdate = false
		info.Reason = "upgrade already running"
	}
	return info, nil
}

func (s *Service) canUpdate() (bool, string) {
	root := s.hostPath(s.installDir())
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return false, "install dir not found (dev or non-standard install)"
	}
	script := s.hostPath(filepath.Join(s.installDir(), "scripts", "dock-pilot-upgrade.sh"))
	if st, err := os.Stat(script); err != nil || st.IsDir() {
		return false, "upgrade script missing"
	}
	if !s.host.UsesChroot() {
		if _, err := os.Stat(filepath.Join(s.installDir(), ".env")); err != nil {
			return false, "HOST_ROOT not set; panel update requires containerized API"
		}
	}
	return true, ""
}

func (s *Service) readCurrentVersion(ctx context.Context) string {
	// Prefer the running frontend image — /opt/dock-pilot/VERSION can lag after CLI upgrades.
	if v := s.readFrontendImageVersion(ctx); v != "" {
		return v
	}
	if v := normalizeVersion(os.Getenv("APP_VERSION")); v != "" {
		return v
	}
	p := s.hostPath(filepath.Join(s.installDir(), "VERSION"))
	if b, err := os.ReadFile(p); err == nil {
		if v := normalizeVersion(string(b)); v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) readFrontendImageVersion(ctx context.Context) string {
	out, err := s.host.RunHostCombined(ctx, "docker", "inspect",
		"--format", `{{range .Config.Env}}{{println .}}{{end}}`,
		"dock-pilot-frontend")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NEXT_PUBLIC_APP_VERSION=") {
			return normalizeVersion(strings.TrimPrefix(line, "NEXT_PUBLIC_APP_VERSION="))
		}
	}
	return ""
}

func (s *Service) fetchLatestRelease(ctx context.Context) (string, error) {
	repo := s.githubRepo()
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dock-pilot-panel")

	client := &http.Client{Timeout: 12 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github HTTP %d", res.StatusCode)
	}
	var parsed struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	v := normalizeVersion(parsed.TagName)
	if v == "" {
		return "", errors.New("github: empty tag_name")
	}
	return v, nil
}

func (s *Service) StartUpgrade(ctx context.Context, target string) (UpgradeStartResult, error) {
	ok, reason := s.canUpdate()
	if !ok {
		return UpgradeStartResult{}, fmt.Errorf("%w: %s", ErrUpgradeNotAvail, reason)
	}
	job := s.readUpgradeJob()
	if job.Status == "running" {
		return UpgradeStartResult{}, ErrUpgradeBusy
	}

	target = strings.TrimSpace(target)
	if target == "" {
		target = "latest"
	}
	if target != "latest" {
		target = normalizeVersion(target)
		if target == "" || !safeVersionRe.MatchString(target) {
			return UpgradeStartResult{}, fmt.Errorf("%w: invalid target", ErrUpgradeStartFail)
		}
	}

	installDir := s.installDir()
	stateDir := filepath.Join(installDir, upgradeStateDir)
	scriptPath := filepath.Join(installDir, "scripts", "dock-pilot-upgrade.sh")

	// One host shell: mark running, spawn upgrade in background, write final status.
	launcher := fmt.Sprintf(`
set -eu
STATE=%q
INSTALL=%q
SCRIPT=%q
TARGET=%q
mkdir -p "$STATE"
echo "$TARGET" > "$STATE/target"
: > "$STATE/log"
echo running > "$STATE/status"
nohup env DOCK_PILOT_FORCE_PROGRESS=1 DOCK_PILOT_INSTALL_DIR="$INSTALL" \
  sh -c 'bash "$1" "$2" >>"$3/log" 2>&1; ec=$?; if [ "$ec" -eq 0 ]; then echo ok > "$3/status"; else echo failed > "$3/status"; fi; exit "$ec"' \
  sh "$SCRIPT" "$TARGET" "$STATE" >/dev/null 2>&1 &
echo $! > "$STATE/pid"
`, stateDir, installDir, scriptPath, target)

	var err error
	if s.host.UsesChroot() {
		_, err = s.host.RunHostCombined(ctx, "nsenter", "-t", "1", "-m", "-n", "-u", "-i", "-p", "--", "sh", "-c", launcher)
	} else {
		_, err = s.host.RunShellCombined(ctx, launcher)
	}
	if err != nil {
		return UpgradeStartResult{}, fmt.Errorf("%w: %v", ErrUpgradeStartFail, err)
	}

	return UpgradeStartResult{
		Status:  "running",
		Message: "Upgrade started on the host; the panel may briefly disconnect while containers recreate",
	}, nil
}

func (s *Service) GetUpgradeJob(_ context.Context) UpgradeJobStatus {
	return s.readUpgradeJob()
}

func (s *Service) readUpgradeJob() UpgradeJobStatus {
	stateDir := s.hostPath(s.upgradeDir())
	statusPath := filepath.Join(stateDir, "status")
	logPath := filepath.Join(stateDir, "log")
	targetPath := filepath.Join(stateDir, "target")

	out := UpgradeJobStatus{Status: "idle"}
	if b, err := os.ReadFile(statusPath); err == nil {
		st := strings.TrimSpace(string(b))
		switch st {
		case "running", "ok", "failed":
			out.Status = st
		default:
			if st != "" {
				out.Status = st
			}
		}
		if fi, err := os.Stat(statusPath); err == nil {
			out.UpdatedAt = fi.ModTime().UTC().Format(time.RFC3339)
		}
	}
	if b, err := os.ReadFile(targetPath); err == nil {
		out.Target = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(logPath); err == nil {
		out.Log = trimLogTail(string(b), 64*1024)
	}

	if out.Status == "running" {
		pidPath := filepath.Join(stateDir, "pid")
		if b, err := os.ReadFile(pidPath); err == nil {
			pidStr := strings.TrimSpace(string(b))
			if pid, err := strconv.Atoi(pidStr); err == nil && pid > 1 {
				alive := s.hostPIDAlive(pid)
				if !alive {
					if fi, err := os.Stat(statusPath); err == nil && time.Since(fi.ModTime()) > 20*time.Second {
						out.Status = "failed"
						_ = os.WriteFile(statusPath, []byte("failed\n"), 0o644)
					}
				}
			}
		}
	}
	return out
}

func (s *Service) hostPIDAlive(pid int) bool {
	check := fmt.Sprintf("kill -0 %d 2>/dev/null", pid)
	var err error
	if s.host.UsesChroot() {
		_, err = s.host.RunHostCombined(context.Background(), "nsenter", "-t", "1", "-m", "-n", "-u", "-i", "-p", "--", "sh", "-c", check)
	} else {
		_, err = s.host.RunShellCombined(context.Background(), check)
	}
	return err == nil
}

func normalizeVersion(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || strings.EqualFold(v, "dev") {
		return ""
	}
	if !strings.HasPrefix(v, "v") && v[0] >= '0' && v[0] <= '9' {
		v = "v" + v
	}
	return v
}

func versionsDiffer(current, latest string) bool {
	c := normalizeVersion(current)
	l := normalizeVersion(latest)
	if l == "" {
		return false
	}
	if c == "" {
		return true
	}
	return c != l
}

func trimLogTail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
