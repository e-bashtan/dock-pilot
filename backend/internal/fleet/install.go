package fleet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/ssh"

	"github.com/ebash/dock-pilot/backend/internal/db"
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
	if host == "" || pass == "" || strings.TrimSpace(req.Name) == "" {
		return InstallationResponse{}, ErrInvalidInput
	}
	nodeUID := uuid.New()
	inst, err := s.q.CreateFleetInstallation(ctx, db.CreateFleetInstallationParams{
		NodeID:          pgtype.UUID{},
		Host:            host,
		Port:            int32(port),
		Username:        user,
		Status:          "checking_host_key",
		CurrentStep:     "Проверка SSH-ключа",
		ExpectedNodeUid: nodeUID,
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

func (s *Service) runHostKeyProbe(ctx context.Context, id uuid.UUID, host string, port int, user string) {
	fp, err := probeSSHFingerprint(ctx, host, port)
	if err != nil {
		s.failInstall(ctx, id, "ssh_probe_failed", "не удалось получить SSH host key")
		return
	}
	known, err := s.q.GetKnownHost(ctx, db.GetKnownHostParams{Host: host, Port: int32(port)})
	if err == nil && known.Fingerprint != "" && known.Fingerprint != fp {
		s.failInstall(ctx, id, "host_key_mismatch", "SSH fingerprint изменился")
		return
	}
	_, _ = s.q.UpdateFleetInstallation(ctx, db.UpdateFleetInstallationParams{
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
	var key ssh.PublicKey
	config := &ssh.ClientConfig{
		User: "probe",
		HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
			key = k
			return fmt.Errorf("stop")
		},
		Timeout: 10 * time.Second,
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := ssh.Dial("tcp", addr, config)
	if conn != nil {
		_ = conn.Close()
	}
	if key == nil {
		if err != nil && !strings.Contains(err.Error(), "stop") {
			return "", err
		}
		return "", fmt.Errorf("no host key")
	}
	return ssh.FingerprintSHA256(key), nil
}

func (s *Service) ConfirmHostKey(ctx context.Context, id uuid.UUID) (InstallationResponse, error) {
	inst, err := s.q.GetFleetInstallation(ctx, id)
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
	inst2, _ := s.q.GetFleetInstallation(ctx, id)
	return s.installationResponse(inst2), nil
}

func (s *Service) runInstall(parent context.Context, id uuid.UUID) {
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	inst, err := s.q.GetFleetInstallation(ctx, id)
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
		_, _ = s.q.UpdateFleetInstallation(ctx, db.UpdateFleetInstallationParams{
			ID: id, Status: status, CurrentStep: step, SshFingerprint: "",
			NodeID: pgtype.UUID{}, ErrorCode: "", ErrorMessage: "", CompletedAt: pgtype.Timestamptz{},
		})
		_ = s.appendInstallLog(ctx, id, "info", step)
	}

	set("connecting", "Подключение к серверу")
	client, err := sshDial(inst.Host, int(inst.Port), inst.Username, password, inst.SshFingerprint)
	clearString(&password)
	if err != nil {
		s.failInstall(ctx, id, "ssh_connect_failed", "не удалось подключиться по SSH")
		return
	}
	defer client.Close()

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
	remoteTmp := "/tmp/dockpilot-agent." + hex.EncodeToString([]byte(sum)[:6])
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
		node, err := s.q.GetFleetNodeByUID(ctx, inst.ExpectedNodeUid)
		if err == nil && node.LastHeartbeatAt.Valid {
			_, _ = s.q.UpdateFleetInstallation(ctx, db.UpdateFleetInstallationParams{
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
	name := req.Hostname
	if name == "" {
		name = "agent-" + nodeUID.String()[:8]
	}
	node, err := s.q.CreateFleetNode(ctx, db.CreateFleetNodeParams{
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
	_, _ = s.q.CreateFleetCredential(ctx, db.CreateFleetCredentialParams{
		NodeID:         node.ID,
		Direction:      "inbound",
		Purpose:        "agent_heartbeat",
		Scopes:         []string{ScopeHeartbeatWrite, ScopeEventsWrite},
		TokenHash:      HashToken(permanent),
		EncryptedToken: nil,
	})
	if row.InstallationID.Valid {
		instID := uuid.UUID(row.InstallationID.Bytes)
		_, _ = s.q.UpdateFleetInstallation(ctx, db.UpdateFleetInstallationParams{
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
	inst, err := s.q.GetFleetInstallation(ctx, id)
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
	_, err := s.q.UpdateFleetInstallation(ctx, db.UpdateFleetInstallationParams{
		ID: id, Status: "cancelled", CurrentStep: "Отменено",
		SshFingerprint: "", NodeID: pgtype.UUID{},
		ErrorCode: "cancelled", ErrorMessage: "cancelled",
		CompletedAt: pgTimestamptz(time.Now().UTC()),
	})
	return mapErr(err)
}

func (s *Service) failInstall(ctx context.Context, id uuid.UUID, code, msg string) {
	_, _ = s.q.UpdateFleetInstallation(ctx, db.UpdateFleetInstallationParams{
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
	_, err := s.q.InsertFleetInstallationLog(ctx, db.InsertFleetInstallationLogParams{
		InstallationID: id, Level: level, Message: msg,
	})
	return err
}

func (s *Service) installationResponse(inst db.FleetInstallation) InstallationResponse {
	out := InstallationResponse{
		ID:             inst.ID,
		Status:         inst.Status,
		CurrentStep:    inst.CurrentStep,
		SSHFingerprint: inst.SshFingerprint,
		Host:           inst.Host,
		Port:           int(inst.Port),
		Username:       inst.Username,
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
	if _, err := s.q.GetFleetInstallation(ctx, id); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.q.ListFleetInstallationLogs(ctx, db.ListFleetInstallationLogsParams{
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
	name := "dockpilot-agent-linux-" + goArch
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
	return fmt.Sprintf(`set -euo pipefail
id dockpilot-agent >/dev/null 2>&1 || useradd --system --home /var/lib/dockpilot-agent --shell /usr/sbin/nologin dockpilot-agent
mkdir -p /opt/dockpilot-agent /etc/dockpilot-agent /var/lib/dockpilot-agent
SUM=$(sha256sum %s | awk '{print $1}')
test "$SUM" = %s
install -o root -g root -m 0755 %s /opt/dockpilot-agent/dockpilot-agent
cat > /etc/dockpilot-agent/config.json <<EOF
{"master_url":%q,"node_uid":%q,"node_token":"","heartbeat_interval_seconds":30}
EOF
chmod 0600 /etc/dockpilot-agent/config.json
chown dockpilot-agent:dockpilot-agent /etc/dockpilot-agent/config.json /var/lib/dockpilot-agent
cat > /etc/systemd/system/dockpilot-agent.service <<'UNIT'
[Unit]
Description=DockPilot Agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=dockpilot-agent
Group=dockpilot-agent
ExecStart=/opt/dockpilot-agent/dockpilot-agent -config /etc/dockpilot-agent/config.json
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/dockpilot-agent /etc/dockpilot-agent
[Install]
WantedBy=multi-user.target
UNIT
/opt/dockpilot-agent/dockpilot-agent -config /etc/dockpilot-agent/config.json -register -master-url %s -registration-token %s -node-uid %s
systemctl daemon-reload
systemctl enable --now dockpilot-agent.service
rm -f %s
`, strconv.Quote(tmpPath), strconv.Quote(checksum), strconv.Quote(tmpPath),
		masterURL, nodeUID, strconv.Quote(masterURL), strconv.Quote(regToken), strconv.Quote(nodeUID), strconv.Quote(tmpPath))
}

// prevent unused rand import complaint in some builds
var _ = rand.Reader
