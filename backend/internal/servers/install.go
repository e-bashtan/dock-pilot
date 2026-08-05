package servers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/ssh"

	"github.com/ebash/barn/backend/internal/db"
)

var unitNameRe = regexp.MustCompile(`^[a-zA-Z0-9@._:-]+\.service$`)

func ValidUnitName(name string) bool {
	return unitNameRe.MatchString(name) && len(name) <= 128
}

func (s *Service) StartAgentInstall(ctx context.Context, req CreateAgentInstallRequest) (InstallationResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return InstallationResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return InstallationResponse{}, ErrForbidden
	}
	if err := s.ensureInstallSchema(ctx); err != nil {
		return InstallationResponse{}, err
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "agent"
	}
	kind = NormalizeInstallKind(kind)
	if kind != "agent" && kind != "barn" {
		return InstallationResponse{}, ErrInvalidInput
	}
	host := strings.TrimSpace(req.Host)
	user := strings.TrimSpace(req.Username)
	if user == "" {
		user = "root"
	}
	port := req.Port
	if port <= 0 {
		port = 22
	}
	pass := req.Password
	req.Password = ""
	name := strings.TrimSpace(req.Name)
	panelURL := strings.TrimRight(strings.TrimSpace(req.PanelURL), "/")
	email := strings.TrimSpace(req.Email)
	if host == "" || pass == "" || name == "" {
		return InstallationResponse{}, ErrInvalidInput
	}
	if kind == "barn" {
		if panelURL == "" {
			return InstallationResponse{}, fmt.Errorf("%w: panel_url is required", ErrInvalidInput)
		}
		if err := validatePublicURL(panelURL); err != nil {
			return InstallationResponse{}, err
		}
		if email == "" {
			u, _ := url.Parse(panelURL)
			if u != nil && u.Hostname() != "" {
				email = "admin@" + u.Hostname()
			}
		}
		if email == "" || !strings.Contains(email, "@") {
			return InstallationResponse{}, fmt.Errorf("%w: email is required for Barn install", ErrInvalidInput)
		}
	}
	nodeUID := uuid.New()
	ttl := 10 * time.Minute
	if kind == "barn" {
		ttl = 45 * time.Minute
	}
	inst, err := s.q.CreateServersInstallation(ctx, db.CreateServersInstallationParams{
		NodeID:          pgtype.UUID{},
		Host:            host,
		Port:            int32(port),
		Username:        user,
		Status:          "checking_host_key",
		CurrentStep:     "Проверка SSH-ключа",
		ExpectedNodeUid: nodeUID,
		InstallKind:     kind,
		PanelUrl:        panelURL,
		CertEmail:       email,
		DisplayName:     name,
	})
	if err != nil {
		return InstallationResponse{}, mapErr(err)
	}
	s.installMu.Lock()
	jobCtx, cancel := context.WithCancel(context.Background())
	s.installs[inst.ID] = &installSecret{
		password:  pass,
		expiresAt: time.Now().Add(ttl),
		ctx:       jobCtx,
		cancel:    cancel,
	}
	s.installMu.Unlock()

	go s.runHostKeyProbe(jobCtx, inst.ID, host, port, user)

	return s.installationResponse(inst), nil
}

func (s *Service) runHostKeyProbe(ctx context.Context, id uuid.UUID, host string, port int, user string) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	_ = s.appendInstallLog(ctx, id, "info", "Проверка SSH "+addr+" (пароль пока не используется)")
	fp, err := probeSSHFingerprint(ctx, host, port)
	if err != nil {
		s.failInstall(ctx, id, "ssh_probe_failed", "не удалось получить SSH host key: "+err.Error())
		return
	}
	known, err := s.q.GetKnownHost(ctx, db.GetKnownHostParams{Host: host, Port: int32(port)})
	if err == nil && known.Fingerprint != "" && known.Fingerprint != fp {
		s.failInstall(ctx, id, "host_key_mismatch", "SSH fingerprint изменился")
		return
	}
	_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
		ID:             id,
		Status:         "awaiting_host_key_confirmation",
		CurrentStep:    "Подтвердите SSH fingerprint",
		SshFingerprint: fp,
		NodeID:         pgtype.UUID{},
		ErrorCode:      "",
		ErrorMessage:   "",
		CompletedAt:    pgtype.Timestamptz{},
	})
	_ = s.appendInstallLog(ctx, id, "info", "SSH fingerprint: "+fp)
}

func probeSSHFingerprint(ctx context.Context, host string, port int) (string, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	d := net.Dialer{Timeout: 10 * time.Second}
	tcp, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("нет TCP до %s (%w)", addr, err)
	}
	_ = tcp.Close()

	var key ssh.PublicKey
	config := &ssh.ClientConfig{
		User: "barn-probe",
		Auth: []ssh.AuthMethod{
			ssh.Password("__probe__"),
		},
		HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
			key = k
			return fmt.Errorf("stop")
		},
		Timeout: 10 * time.Second,
	}
	conn, err := ssh.Dial("tcp", addr, config)
	if conn != nil {
		_ = conn.Close()
	}
	if key == nil {
		if err != nil && !strings.Contains(err.Error(), "stop") {
			return "", fmt.Errorf("SSH handshake %s: %w", addr, err)
		}
		return "", fmt.Errorf("сервер не отдал host key на %s", addr)
	}
	return ssh.FingerprintSHA256(key), nil
}

func (s *Service) ConfirmHostKey(ctx context.Context, id uuid.UUID) (InstallationResponse, error) {
	inst, err := s.q.GetServersInstallation(ctx, id)
	if err != nil {
		return InstallationResponse{}, mapErr(err)
	}
	if inst.Status != "awaiting_host_key_confirmation" {
		return InstallationResponse{}, ErrConflict
	}
	s.installMu.Lock()
	sec := s.installs[id]
	s.installMu.Unlock()
	if sec == nil || time.Now().After(sec.expiresAt) {
		s.failInstall(ctx, id, "credentials_expired", "SSH credentials истекли — повторите установку")
		return InstallationResponse{}, ErrInvalidInput
	}
	_, _ = s.q.UpsertKnownHost(ctx, db.UpsertKnownHostParams{
		Host:        inst.Host,
		Port:        inst.Port,
		Fingerprint: inst.SshFingerprint,
	})
	go s.runInstall(sec.ctx, id)
	inst2, _ := s.q.GetServersInstallation(ctx, id)
	return s.installationResponse(inst2), nil
}

func (s *Service) runInstall(parent context.Context, id uuid.UUID) {
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	inst, err := s.q.GetServersInstallation(ctx, id)
	if err != nil {
		return
	}
	s.installMu.Lock()
	sec := s.installs[id]
	s.installMu.Unlock()
	if sec == nil {
		s.failInstall(ctx, id, "credentials_missing", "нет SSH credentials в памяти")
		return
	}
	password := sec.password

	set := func(status, step string) {
		_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
			ID: id, Status: status, CurrentStep: step, SshFingerprint: "",
			NodeID: pgtype.UUID{}, ErrorCode: "", ErrorMessage: "", CompletedAt: pgtype.Timestamptz{},
		})
		_ = s.appendInstallLog(ctx, id, "info", step)
	}

	set("connecting", "Подключение к серверу")
	client, err := sshDial(inst.Host, int(inst.Port), inst.Username, password, inst.SshFingerprint)
	clearString(&password)
	if err != nil {
		s.failInstall(ctx, id, "ssh_connect_failed", "не удалось подключиться по SSH: "+err.Error())
		return
	}
	defer client.Close()

	if NormalizeInstallKind(inst.InstallKind) == "barn" {
		s.runBarnInstall(ctx, client, id, inst, set)
		return
	}
	if inst.InstallKind == "agent_update" {
		s.runAgentUpdate(ctx, client, id, inst, set)
		return
	}
	s.runAgentInstall(ctx, client, id, inst, set)
}

// StartAgentUpdate redeploys the agent binary on an existing agent node over SSH.
// Keeps /etc/barn-agent/config.json (node_uid + token); does not re-register.
func (s *Service) StartAgentUpdate(ctx context.Context, nodeID uuid.UUID, req UpdateAgentRequest) (InstallationResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return InstallationResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return InstallationResponse{}, ErrForbidden
	}
	if err := s.ensureInstallSchema(ctx); err != nil {
		return InstallationResponse{}, err
	}
	node, err := s.q.GetServersNode(ctx, nodeID)
	if err != nil {
		return InstallationResponse{}, mapErr(err)
	}
	if node.ConnectionType != ConnAgent {
		return InstallationResponse{}, fmt.Errorf("%w: only agent nodes can be updated this way", ErrInvalidInput)
	}
	host := strings.TrimSpace(req.Host)
	user := strings.TrimSpace(req.Username)
	if user == "" {
		user = "root"
	}
	port := req.Port
	if port <= 0 {
		port = 22
	}
	pass := req.Password
	req.Password = ""
	if host == "" || pass == "" {
		return InstallationResponse{}, ErrInvalidInput
	}
	inst, err := s.q.CreateServersInstallation(ctx, db.CreateServersInstallationParams{
		NodeID:          pgUUID(node.ID),
		Host:            host,
		Port:            int32(port),
		Username:        user,
		Status:          "checking_host_key",
		CurrentStep:     "Проверка SSH-ключа",
		ExpectedNodeUid: node.NodeUid,
		InstallKind:     "agent_update",
		PanelUrl:        "",
		CertEmail:       "",
		DisplayName:     node.Name,
	})
	if err != nil {
		return InstallationResponse{}, mapErr(err)
	}
	s.installMu.Lock()
	jobCtx, cancel := context.WithCancel(context.Background())
	s.installs[inst.ID] = &installSecret{
		password:  pass,
		expiresAt: time.Now().Add(10 * time.Minute),
		ctx:       jobCtx,
		cancel:    cancel,
	}
	s.installMu.Unlock()

	go s.runHostKeyProbe(jobCtx, inst.ID, host, port, user)
	return s.installationResponse(inst), nil
}

func (s *Service) runAgentUpdate(ctx context.Context, client *ssh.Client, id uuid.UUID, inst db.ServersInstallation, set func(status, step string)) {
	set("detecting_system", "Определение системы")
	osRelease, _ := sshRun(client, "cat /etc/os-release")
	unameM, _ := sshRun(client, "uname -m")
	arch := strings.TrimSpace(unameM)
	goArch := ""
	switch arch {
	case "x86_64", "amd64":
		goArch = "amd64"
	case "aarch64", "arm64":
		goArch = "arm64"
	default:
		s.failInstall(ctx, id, "unsupported_arch", "неподдерживаемая архитектура: "+arch)
		return
	}
	if !strings.Contains(osRelease, "Ubuntu") && !strings.Contains(osRelease, "Debian") {
		s.failInstall(ctx, id, "unsupported_os", "поддерживаются Ubuntu/Debian")
		return
	}
	if _, err := sshRun(client, "command -v systemctl"); err != nil {
		s.failInstall(ctx, id, "no_systemd", "systemd не найден")
		return
	}
	_ = s.appendInstallLog(ctx, id, "info", "Определение "+detectPrettyOS(osRelease)+" "+goArch)

	// Prefer barn-agent; fall back to legacy dockpilot-agent unit/binary.
	unit := "barn-agent.service"
	binRemote := "/opt/barn-agent/barn-agent"
	if _, err := sshRun(client, "test -f /etc/barn-agent/config.json"); err != nil {
		if _, err2 := sshRun(client, "test -f /etc/dockpilot-agent/config.json"); err2 != nil {
			s.failInstall(ctx, id, "agent_not_found", "конфиг агента не найден (/etc/barn-agent или /etc/dockpilot-agent)")
			return
		}
		unit = "dockpilot-agent.service"
		binRemote = "/usr/local/bin/dockpilot-agent"
		if out, err := sshRun(client, "test -x /opt/dockpilot-agent/dockpilot-agent && echo opt"); err == nil && strings.Contains(out, "opt") {
			binRemote = "/opt/dockpilot-agent/dockpilot-agent"
		}
		_ = s.appendInstallLog(ctx, id, "info", "Найден legacy dockpilot-agent")
	} else {
		_ = s.appendInstallLog(ctx, id, "info", "Найден barn-agent")
	}

	prevHB := time.Time{}
	if node, err := s.q.GetServersNodeByUID(ctx, inst.ExpectedNodeUid); err == nil && node.LastHeartbeatAt.Valid {
		prevHB = node.LastHeartbeatAt.Time
	}

	set("uploading_agent", "Загрузка новой версии агента")
	binPath := s.agentBinaryPath(goArch)
	bin, err := os.ReadFile(binPath)
	if err != nil {
		s.failInstall(ctx, id, "agent_missing", "agent binary не найден в API image")
		return
	}
	sum := sha256Hex(bin)
	remoteTmp := "/tmp/barn-agent-update." + hex.EncodeToString([]byte(sum)[:6])
	if err := sshUpload(client, remoteTmp, bin); err != nil {
		s.failInstall(ctx, id, "upload_failed", "не удалось загрузить agent")
		return
	}

	set("installing_service", "Замена бинарника и перезапуск")
	script := buildAgentUpdateScript(remoteTmp, sum, binRemote, unit)
	if out, err := sshRun(client, script); err != nil {
		_ = s.appendInstallLog(ctx, id, "error", truncateLog(out, 500))
		s.failInstall(ctx, id, "update_failed", "ошибка обновления agent")
		return
	}
	_ = s.appendInstallLog(ctx, id, "info", "Сервис перезапущен: "+unit)

	set("waiting_for_registration", "Ожидание heartbeat после обновления")
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			s.failInstall(context.Background(), id, "cancelled", "обновление отменено")
			return
		case <-time.After(3 * time.Second):
		}
		node, err := s.q.GetServersNodeByUID(ctx, inst.ExpectedNodeUid)
		if err != nil {
			continue
		}
		if !node.LastHeartbeatAt.Valid {
			continue
		}
		if node.LastHeartbeatAt.Time.After(prevHB) {
			_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
				ID: id, Status: "completed", CurrentStep: "Агент обновлён",
				SshFingerprint: "", NodeID: pgUUID(node.ID),
				ErrorCode: "", ErrorMessage: "",
				CompletedAt: pgTimestamptz(time.Now().UTC()),
			})
			ver := node.AgentVersion
			if ver == "" {
				ver = "ok"
			}
			_ = s.appendInstallLog(ctx, id, "info", "Heartbeat получен, версия агента: "+ver)
			s.clearInstallSecret(id)
			return
		}
	}
	s.failInstall(ctx, id, "heartbeat_timeout", "агент не прислал heartbeat после обновления")
}

func (s *Service) runAgentInstall(ctx context.Context, client *ssh.Client, id uuid.UUID, inst db.ServersInstallation, set func(status, step string)) {
	set("detecting_system", "Определение системы")
	osRelease, _ := sshRun(client, "cat /etc/os-release")
	unameM, _ := sshRun(client, "uname -m")
	arch := strings.TrimSpace(unameM)
	goArch := ""
	switch arch {
	case "x86_64", "amd64":
		goArch = "amd64"
	case "aarch64", "arm64":
		goArch = "arm64"
	default:
		s.failInstall(ctx, id, "unsupported_arch", "неподдерживаемая архитектура: "+arch)
		return
	}
	if !strings.Contains(osRelease, "Ubuntu") && !strings.Contains(osRelease, "Debian") {
		s.failInstall(ctx, id, "unsupported_os", "поддерживаются Ubuntu/Debian")
		return
	}
	if _, err := sshRun(client, "command -v systemctl"); err != nil {
		s.failInstall(ctx, id, "no_systemd", "systemd не найден")
		return
	}
	_ = s.appendInstallLog(ctx, id, "info", "Определение "+detectPrettyOS(osRelease)+" "+goArch)

	set("uploading_agent", "Загрузка агента")
	binPath := s.agentBinaryPath(goArch)
	bin, err := os.ReadFile(binPath)
	if err != nil {
		s.failInstall(ctx, id, "agent_missing", "agent binary не найден в API image")
		return
	}
	sum := sha256Hex(bin)
	remoteTmp := "/tmp/barn-agent." + hex.EncodeToString([]byte(sum)[:6])
	if err := sshUpload(client, remoteTmp, bin); err != nil {
		s.failInstall(ctx, id, "upload_failed", "не удалось загрузить agent")
		return
	}

	set("installing_service", "Установка systemd service")
	regToken, _ := GenerateToken("reg")
	_, _ = s.q.CreateRegistrationToken(ctx, db.CreateRegistrationTokenParams{
		InstallationID:  pgUUID(id),
		ExpectedNodeUid: inst.ExpectedNodeUid,
		TokenHash:       HashToken(regToken),
		ExpiresAt:       time.Now().UTC().Add(RegTokenTTL),
	})
	settings, _ := s.ensureSettings(ctx)
	masterURL := settings.PublicUrl
	script := buildInstallScript(remoteTmp, sum, masterURL, regToken, inst.ExpectedNodeUid.String())
	if _, err := sshRun(client, script); err != nil {
		s.failInstall(ctx, id, "install_failed", "ошибка установки agent")
		return
	}

	set("waiting_for_registration", "Ожидание регистрации агента")
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			s.failInstall(context.Background(), id, "cancelled", "установка отменена")
			return
		case <-time.After(3 * time.Second):
		}
		node, err := s.q.GetServersNodeByUID(ctx, inst.ExpectedNodeUid)
		if err == nil && node.LastHeartbeatAt.Valid {
			_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
				ID: id, Status: "completed", CurrentStep: "Сервер подключён",
				SshFingerprint: "", NodeID: pgUUID(node.ID),
				ErrorCode: "", ErrorMessage: "",
				CompletedAt: pgTimestamptz(time.Now().UTC()),
			})
			_ = s.appendInstallLog(ctx, id, "info", "Сервер подключён")
			s.clearInstallSecret(id)
			return
		}
	}
	s.failInstall(ctx, id, "registration_timeout", "агент не зарегистрировался вовремя")
}

func (s *Service) runBarnInstall(ctx context.Context, client *ssh.Client, id uuid.UUID, inst db.ServersInstallation, set func(status, step string)) {
	set("detecting_system", "Определение системы")
	osRelease, _ := sshRun(client, "cat /etc/os-release")
	if !strings.Contains(osRelease, "Ubuntu") && !strings.Contains(osRelease, "Debian") {
		s.failInstall(ctx, id, "unsupported_os", "поддерживаются Ubuntu/Debian")
		return
	}
	if _, err := sshRun(client, "command -v systemctl"); err != nil {
		s.failInstall(ctx, id, "no_systemd", "systemd не найден")
		return
	}
	_ = s.appendInstallLog(ctx, id, "info", "Определение "+detectPrettyOS(osRelease))

	panelURL := strings.TrimRight(inst.PanelUrl, "/")
	u, err := url.Parse(panelURL)
	if err != nil || u.Hostname() == "" {
		s.failInstall(ctx, id, "bad_panel_url", "некорректный URL панели")
		return
	}
	domain := u.Hostname()
	email := inst.CertEmail
	if email == "" {
		email = "admin@" + domain
	}
	apiToken, err := GenerateToken("barn")
	if err != nil {
		s.failInstall(ctx, id, "token_failed", "не удалось создать API token")
		return
	}

	set("installing_service", "Установка Barn")
	repo := githubInstallRepo()
	scriptURL := "https://raw.githubusercontent.com/" + repo + "/main/scripts/install.sh"
	script := buildBarnInstallScript(scriptURL, domain, email, apiToken)
	out, err := sshRun(client, script)
	if err != nil {
		_ = s.appendInstallLog(ctx, id, "error", truncateLog(out, 500))
		clearString(&apiToken)
		s.failInstall(ctx, id, "install_failed", "ошибка установки Barn")
		return
	}
	_ = s.appendInstallLog(ctx, id, "info", "Установщик завершился")

	set("waiting_for_registration", "Ожидание запуска панели")
	deadline := time.Now().Add(20 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			clearString(&apiToken)
			s.failInstall(context.Background(), id, "cancelled", "установка отменена")
			return
		case <-time.After(5 * time.Second):
		}
		if panelHealthy(ctx, panelURL) {
			break
		}
	}
	if !panelHealthy(ctx, panelURL) {
		clearString(&apiToken)
		s.failInstall(ctx, id, "panel_timeout", "панель не ответила вовремя")
		return
	}

	set("starting_agent", "Сопряжение с Master")
	code, err := fetchRemotePairingCode(ctx, panelURL, apiToken)
	clearString(&apiToken)
	if err != nil {
		s.failInstall(ctx, id, "pairing_code_failed", "не удалось получить pairing code с новой панели")
		return
	}
	name := inst.DisplayName
	if name == "" {
		name = domain
	}
	node, err := s.PairRemoteBarn(ctx, PairBarnRequest{
		Name:        name,
		BaseURL:     panelURL,
		PairingCode: code,
	})
	if err != nil {
		s.failInstall(ctx, id, "pair_failed", "не удалось сопрячь панель с Master")
		return
	}
	_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
		ID: id, Status: "completed", CurrentStep: "Сервер подключён",
		SshFingerprint: "", NodeID: pgUUID(node.ID),
		ErrorCode: "", ErrorMessage: "",
		CompletedAt: pgTimestamptz(time.Now().UTC()),
	})
	_ = s.appendInstallLog(ctx, id, "info", "Сервер подключён")
	s.clearInstallSecret(id)
}

func githubInstallRepo() string {
	if r := strings.TrimSpace(os.Getenv("BARN_GITHUB_REPO")); r != "" {
		return r
	}
	// Legacy env alias.
	if r := strings.TrimSpace(os.Getenv("DOCK_PILOT_GITHUB_REPO")); r != "" {
		return r
	}
	return "ebasht/barn"
}

func buildBarnInstallScript(scriptURL, domain, email, apiToken string) string {
	return fmt.Sprintf(`set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
curl -fsSL %s -o /tmp/barn-install.sh
chmod 700 /tmp/barn-install.sh
bash /tmp/barn-install.sh --domain %s --email %s --token %s
rm -f /tmp/barn-install.sh
`, strconv.Quote(scriptURL), strconv.Quote(domain), strconv.Quote(email), strconv.Quote(apiToken))
}

func panelHealthy(ctx context.Context, panelURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(panelURL, "/")+"/health", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: limitRedirects}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return res.StatusCode >= 200 && res.StatusCode < 500
}

func fetchRemotePairingCode(ctx context.Context, panelURL, apiToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(panelURL, "/")+"/api/servers/pairing-code", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: limitRedirects}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", res.StatusCode)
	}
	var out PairingCodeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Code) == "" {
		return "", fmt.Errorf("empty code")
	}
	return out.Code, nil
}

func truncateLog(s string, n int) string {
	s = sanitizeInstallLog(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (s *Service) RegisterAgent(ctx context.Context, req AgentRegisterRequest) (AgentRegisterResponse, error) {
	tok := strings.TrimSpace(req.RegistrationToken)
	row, err := s.q.GetValidRegistrationTokenByHash(ctx, HashToken(tok))
	if err != nil {
		return AgentRegisterResponse{}, ErrUnauthorized
	}
	nodeUID, err := uuid.Parse(strings.TrimSpace(req.NodeUID))
	if err != nil || nodeUID != row.ExpectedNodeUid {
		return AgentRegisterResponse{}, ErrInvalidInput
	}
	if err := s.q.MarkRegistrationTokenUsed(ctx, row.ID); err != nil {
		return AgentRegisterResponse{}, mapErr(err)
	}
	permanent, err := GenerateToken("agt")
	if err != nil {
		return AgentRegisterResponse{}, err
	}
	caps, _ := json.Marshal(AgentCapabilities())
	now := time.Now().UTC()
	name := strings.TrimSpace(req.Hostname)
	if row.InstallationID.Valid {
		instID := uuid.UUID(row.InstallationID.Bytes)
		if inst, err := s.q.GetServersInstallation(ctx, instID); err == nil {
			if dn := strings.TrimSpace(inst.DisplayName); dn != "" {
				name = dn
			}
		}
	}
	if name == "" {
		name = "agent-" + nodeUID.String()[:8]
	}
	node, err := s.q.CreateServersNode(ctx, db.CreateServersNodeParams{
		NodeUid:        nodeUID,
		Name:           name,
		Role:           RoleAgent,
		ConnectionType: ConnAgent,
		BaseUrl:        "",
		Status:         StatusOnline,
		Capabilities:   caps,
		Version:        "",
		AgentVersion:   req.AgentVersion,
		LastSeenAt:     pgTimestamptz(now),
		PairedAt:       pgTimestamptz(now),
		Metadata:       []byte("{}"),
	})
	if err != nil {
		return AgentRegisterResponse{}, mapErr(err)
	}
	_, _ = s.q.CreateServersCredential(ctx, db.CreateServersCredentialParams{
		NodeID:         node.ID,
		Direction:      "inbound",
		Purpose:        "agent_heartbeat",
		Scopes:         []string{ScopeHeartbeatWrite, ScopeEventsWrite},
		TokenHash:      HashToken(permanent),
		EncryptedToken: nil,
	})
	if row.InstallationID.Valid {
		instID := uuid.UUID(row.InstallationID.Bytes)
		_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
			ID: instID, Status: "starting_agent", CurrentStep: "Агент зарегистрирован",
			SshFingerprint: "", NodeID: pgUUID(node.ID),
			ErrorCode: "", ErrorMessage: "", CompletedAt: pgtype.Timestamptz{},
		})
	}
	_ = s.IngestHeartbeat(ctx, node.ID, HeartbeatRequest{
		NodeUID: nodeUID.String(), AgentVersion: req.AgentVersion, Metrics: req.Metrics,
	})
	settings, _ := s.ensureSettings(ctx)
	return AgentRegisterResponse{
		NodeToken:        permanent,
		MasterURL:        settings.PublicUrl,
		HeartbeatSeconds: 30,
	}, nil
}

func (s *Service) GetInstallation(ctx context.Context, id uuid.UUID) (InstallationResponse, error) {
	inst, err := s.q.GetServersInstallation(ctx, id)
	if err != nil {
		return InstallationResponse{}, mapErr(err)
	}
	return s.installationResponse(inst), nil
}

func (s *Service) CancelInstallation(ctx context.Context, id uuid.UUID) error {
	s.installMu.Lock()
	if sec, ok := s.installs[id]; ok && sec.cancel != nil {
		sec.cancel()
	}
	s.installMu.Unlock()
	s.clearInstallSecret(id)
	_, err := s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
		ID: id, Status: "cancelled", CurrentStep: "Отменено",
		SshFingerprint: "", NodeID: pgtype.UUID{},
		ErrorCode: "cancelled", ErrorMessage: "cancelled",
		CompletedAt: pgTimestamptz(time.Now().UTC()),
	})
	return mapErr(err)
}

func (s *Service) failInstall(ctx context.Context, id uuid.UUID, code, msg string) {
	_, _ = s.q.UpdateServersInstallation(ctx, db.UpdateServersInstallationParams{
		ID: id, Status: "failed", CurrentStep: msg,
		SshFingerprint: "", NodeID: pgtype.UUID{},
		ErrorCode: code, ErrorMessage: msg,
		CompletedAt: pgTimestamptz(time.Now().UTC()),
	})
	_ = s.appendInstallLog(ctx, id, "error", msg)
	s.clearInstallSecret(id)
}

func (s *Service) clearInstallSecret(id uuid.UUID) {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	if sec, ok := s.installs[id]; ok {
		clearString(&sec.password)
		delete(s.installs, id)
	}
}

func (s *Service) appendInstallLog(ctx context.Context, id uuid.UUID, level, msg string) error {
	msg = sanitizeInstallLog(msg)
	_, err := s.q.InsertServersInstallationLog(ctx, db.InsertServersInstallationLogParams{
		InstallationID: id, Level: level, Message: msg,
	})
	return err
}

func (s *Service) installationResponse(inst db.ServersInstallation) InstallationResponse {
	out := InstallationResponse{
		ID:             inst.ID,
		Status:         inst.Status,
		CurrentStep:    inst.CurrentStep,
		SSHFingerprint: inst.SshFingerprint,
		Host:           inst.Host,
		Port:           int(inst.Port),
		Username:       inst.Username,
		InstallKind:    inst.InstallKind,
		PanelURL:       inst.PanelUrl,
		ErrorCode:      inst.ErrorCode,
		ErrorMessage:   inst.ErrorMessage,
	}
	if inst.NodeID.Valid {
		id := uuid.UUID(inst.NodeID.Bytes)
		out.NodeID = &id
	}
	return out
}

type InstallationLogResponse struct {
	ID        int64     `json:"id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) ListInstallationLogs(ctx context.Context, id uuid.UUID) ([]InstallationLogResponse, error) {
	if _, err := s.q.GetServersInstallation(ctx, id); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.q.ListServersInstallationLogs(ctx, db.ListServersInstallationLogsParams{
		InstallationID: id,
		ID:             0,
		Limit:          500,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]InstallationLogResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, InstallationLogResponse{
			ID:        r.ID,
			Level:     r.Level,
			Message:   r.Message,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) agentBinaryPath(goArch string) string {
	name := "barn-agent-linux-" + goArch
	if s.agentDir != "" {
		return filepath.Join(s.agentDir, name)
	}
	return filepath.Join("/app/agents", name)
}

func sanitizeInstallLog(msg string) string {
	// strip anything that looks like a password assignment
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "password") || strings.Contains(lower, "token") {
		return "[redacted]"
	}
	return msg
}

func clearString(s *string) {
	if s == nil {
		return
	}
	b := []byte(*s)
	for i := range b {
		b[i] = 0
	}
	*s = ""
}

func detectPrettyOS(osRelease string) string {
	for _, line := range strings.Split(osRelease, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return "Linux"
}

func sshDial(host string, port int, user, password, wantFP string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			fp := ssh.FingerprintSHA256(key)
			if wantFP != "" && fp != wantFP {
				return fmt.Errorf("host key mismatch")
			}
			return nil
		},
		Timeout: 15 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)), config)
}

func sshRun(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

func sshUpload(client *ssh.Client, remote string, data []byte) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	w, err := session.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer w.Close()
		fmt.Fprintf(w, "C0755 %d %s\n", len(data), filepath.Base(remote))
		_, _ = w.Write(data)
		fmt.Fprint(w, "\x00")
	}()
	return session.Run("scp -t " + strconv.Quote(remote))
}

func buildInstallScript(tmpPath, checksum, masterURL, regToken, nodeUID string) string {
	// Fixed script; values are shell-quoted.
	// Register must not leave config owned by root: service runs as barn-agent.
	return fmt.Sprintf(`set -euo pipefail
id barn-agent >/dev/null 2>&1 || useradd --system --home /var/lib/barn-agent --shell /usr/sbin/nologin barn-agent
mkdir -p /opt/barn-agent /etc/barn-agent /var/lib/barn-agent
SUM=$(sha256sum %s | awk '{print $1}')
test "$SUM" = %s
install -o root -g root -m 0755 %s /opt/barn-agent/barn-agent
cat > /etc/barn-agent/config.json <<EOF
{"master_url":%q,"node_uid":%q,"node_token":"","heartbeat_interval_seconds":30}
EOF
chmod 0755 /etc/barn-agent
chmod 0600 /etc/barn-agent/config.json
chown -R barn-agent:barn-agent /etc/barn-agent /var/lib/barn-agent
cat > /etc/systemd/system/barn-agent.service <<'UNIT'
[Unit]
Description=Barn Agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=barn-agent
Group=barn-agent
ExecStartPre=+/bin/chown -R barn-agent:barn-agent /etc/barn-agent /var/lib/barn-agent
ExecStartPre=+/bin/chmod 0755 /etc/barn-agent
ExecStartPre=+/bin/chmod 0600 /etc/barn-agent/config.json
ExecStart=/opt/barn-agent/barn-agent -config /etc/barn-agent/config.json
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/barn-agent /etc/barn-agent
[Install]
WantedBy=multi-user.target
UNIT
# Register as the service user so config.json is never left root:0600.
if command -v runuser >/dev/null 2>&1; then
  runuser -u barn-agent -- /opt/barn-agent/barn-agent -config /etc/barn-agent/config.json -register -master-url %s -registration-token %s -node-uid %s
else
  su -s /bin/sh barn-agent -c "/opt/barn-agent/barn-agent -config /etc/barn-agent/config.json -register -master-url %s -registration-token %s -node-uid %s"
fi
chown -R barn-agent:barn-agent /etc/barn-agent /var/lib/barn-agent
chmod 0755 /etc/barn-agent
chmod 0600 /etc/barn-agent/config.json
systemctl daemon-reload
systemctl enable --now barn-agent.service
rm -f %s
`, strconv.Quote(tmpPath), strconv.Quote(checksum), strconv.Quote(tmpPath),
		masterURL, nodeUID,
		strconv.Quote(masterURL), strconv.Quote(regToken), strconv.Quote(nodeUID),
		strconv.Quote(masterURL), strconv.Quote(regToken), strconv.Quote(nodeUID),
		strconv.Quote(tmpPath))
}

// buildAgentUpdateScript replaces the agent binary and restarts the unit.
// Does not touch config.json (keeps node_uid + token).
func buildAgentUpdateScript(tmpPath, checksum, binRemote, unit string) string {
	return fmt.Sprintf(`set -euo pipefail
SUM=$(sha256sum %s | awk '{print $1}')
test "$SUM" = %s
mkdir -p "$(dirname %s)"
systemctl stop %s || true
install -o root -g root -m 0755 %s %s
systemctl daemon-reload
systemctl enable %s || true
systemctl restart %s
systemctl is-active %s
rm -f %s
`,
		strconv.Quote(tmpPath), strconv.Quote(checksum),
		strconv.Quote(binRemote),
		strconv.Quote(unit),
		strconv.Quote(tmpPath), strconv.Quote(binRemote),
		strconv.Quote(unit),
		strconv.Quote(unit),
		strconv.Quote(unit),
		strconv.Quote(tmpPath),
	)
}

// prevent unused rand import complaint in some builds
var _ = rand.Reader
