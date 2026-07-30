package fleet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/dock-pilot/backend/internal/db"
)

func (s *Service) CreatePairingCode(ctx context.Context) (PairingCodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return PairingCodeResponse{}, err
	}
	if settings.Mode == ModeMaster {
		return PairingCodeResponse{}, fmt.Errorf("%w: master cannot generate pairing code for joining", ErrForbidden)
	}
	code, err := GeneratePairingCode()
	if err != nil {
		return PairingCodeResponse{}, err
	}
	exp := time.Now().UTC().Add(PairingTTL)
	_, err = s.q.CreatePairingCode(ctx, db.CreatePairingCodeParams{
		CodeHash:  HashToken(code),
		ExpiresAt: exp,
	})
	if err != nil {
		return PairingCodeResponse{}, mapErr(err)
	}
	return PairingCodeResponse{Code: code, ExpiresAt: exp}, nil
}

func (s *Service) PairRemoteDockpilot(ctx context.Context, req PairDockpilotRequest) (NodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return NodeResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return NodeResponse{}, ErrForbidden
	}
	name := strings.TrimSpace(req.Name)
	baseURL := strings.TrimRight(strings.TrimSpace(req.URL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = strings.TrimSpace(req.PairingCode)
	}
	if name == "" || baseURL == "" || code == "" {
		return NodeResponse{}, ErrInvalidInput
	}
	if err := validateRemoteURL(baseURL); err != nil {
		return NodeResponse{}, err
	}

	pairReq := PairNodeRequest{
		MasterURL:  settings.PublicUrl,
		MasterName: settings.NodeName,
		NodeName:   name,
		Code:       code,
	}
	body, _ := json.Marshal(pairReq)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/fleet/node/pair", bytes.NewReader(body))
	if err != nil {
		return NodeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: limitRedirects}
	res, err := client.Do(httpReq)
	if err != nil {
		return NodeResponse{}, fmt.Errorf("%w: pair request failed: %v", ErrInvalidInput, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return NodeResponse{}, fmt.Errorf("%w: remote pair failed: %s", ErrInvalidInput, strings.TrimSpace(string(raw)))
	}
	var pairRes PairNodeResponse
	if err := json.Unmarshal(raw, &pairRes); err != nil {
		return NodeResponse{}, ErrInvalidInput
	}
	nodeUID, err := uuid.Parse(pairRes.NodeUID)
	if err != nil {
		return NodeResponse{}, ErrInvalidInput
	}
	if existing, err := s.q.GetFleetNodeByUID(ctx, nodeUID); err == nil {
		_ = existing
		return NodeResponse{}, ErrConflict
	}

	encNodeToken, err := s.cipher.Encrypt(pairRes.NodeToken)
	if err != nil {
		return NodeResponse{}, err
	}
	caps, _ := json.Marshal(DockpilotCapabilities())
	now := time.Now().UTC()
	node, err := s.q.CreateFleetNode(ctx, db.CreateFleetNodeParams{
		NodeUid:        nodeUID,
		Name:           name,
		Role:           RoleNode,
		ConnectionType: ConnDockpilot,
		BaseUrl:        baseURL,
		Status:         StatusOnline,
		Capabilities:   caps,
		Version:        "",
		AgentVersion:   "",
		LastSeenAt:     pgTimestamptz(now),
		PairedAt:       pgTimestamptz(now),
		Metadata:       []byte("{}"),
	})
	if err != nil {
		return NodeResponse{}, mapErr(err)
	}
	_, _ = s.q.CreateFleetCredential(ctx, db.CreateFleetCredentialParams{
		NodeID:         node.ID,
		Direction:      "outbound",
		Purpose:        "master_to_node",
		Scopes:         []string{ScopeStatusRead, ScopeAppsRead, ScopeBackupsRead, ScopeVersionRead},
		TokenHash:      nil,
		EncryptedToken: encNodeToken,
	})
	inboundHash := HashToken(pairRes.MasterToken)
	_, _ = s.q.CreateFleetCredential(ctx, db.CreateFleetCredentialParams{
		NodeID:         node.ID,
		Direction:      "inbound",
		Purpose:        "node_to_master",
		Scopes:         []string{ScopeHeartbeatWrite, ScopeEventsWrite},
		TokenHash:      inboundHash,
		EncryptedToken: nil,
	})
	accounts := s.listBillingAccounts(ctx)
	claimed := map[uuid.UUID]bool{}
	localIP := ""
	if s.hostIP != nil {
		localIP = strings.TrimSpace(s.hostIP(ctx))
	}
	return s.toNodeResponse(ctx, node, accounts, claimed, localIP), nil
}

// AcceptPair is called on the node being paired (remote DockPilot).
func (s *Service) AcceptPair(ctx context.Context, req PairNodeRequest) (PairNodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return PairNodeResponse{}, err
	}
	if settings.Mode == ModeMaster {
		return PairNodeResponse{}, ErrCannotNest
	}
	if settings.Mode == ModeManagedNode && len(settings.EncryptedMasterToken) > 0 {
		return PairNodeResponse{}, ErrAlreadyPaired
	}
	code := strings.TrimSpace(req.Code)
	row, err := s.q.GetValidPairingCodeByHash(ctx, HashToken(code))
	if err != nil {
		return PairNodeResponse{}, ErrPairingExpired
	}
	if err := s.q.MarkPairingCodeUsed(ctx, row.ID); err != nil {
		return PairNodeResponse{}, mapErr(err)
	}

	masterToken, err := GenerateToken("flt")
	if err != nil {
		return PairNodeResponse{}, err
	}
	nodeToken, err := GenerateToken("flt")
	if err != nil {
		return PairNodeResponse{}, err
	}
	encMaster, err := s.cipher.Encrypt(masterToken)
	if err != nil {
		return PairNodeResponse{}, err
	}
	masterURL := strings.TrimRight(strings.TrimSpace(req.MasterURL), "/")
	if err := validateRemoteURL(masterURL); err != nil {
		return PairNodeResponse{}, err
	}
	name := settings.NodeName
	if name == "" {
		name = strings.TrimSpace(req.NodeName)
	}
	if name == "" {
		name = "Node"
	}
	_, err = s.q.UpdateFleetSettings(ctx, db.UpdateFleetSettingsParams{
		ID:                   1,
		Mode:                 ModeManagedNode,
		NodeName:             name,
		PublicUrl:            settings.PublicUrl,
		MasterUrl:            masterURL,
		NotificationMode:     NotifyMaster,
		EncryptedMasterToken: encMaster,
	})
	if err != nil {
		return PairNodeResponse{}, mapErr(err)
	}
	if err := s.storeNodeInboundToken(ctx, nodeToken); err != nil {
		return PairNodeResponse{}, err
	}

	return PairNodeResponse{
		NodeUID:     settings.NodeUid.String(),
		NodeName:    name,
		PublicURL:   settings.PublicUrl,
		MasterToken: masterToken,
		NodeToken:   nodeToken,
		Scopes:      []string{ScopeStatusRead, ScopeAppsRead, ScopeBackupsRead, ScopeVersionRead},
	}, nil
}

func (s *Service) DisconnectMaster(ctx context.Context) error {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return err
	}
	if settings.Mode != ModeManagedNode {
		return ErrForbidden
	}
	_, err = s.q.UpdateFleetSettings(ctx, db.UpdateFleetSettingsParams{
		ID:                   1,
		Mode:                 ModeStandalone,
		NodeName:             settings.NodeName,
		PublicUrl:            settings.PublicUrl,
		MasterUrl:            "",
		NotificationMode:     NotifyLocal,
		EncryptedMasterToken: nil,
	})
	return mapErr(err)
}

func (s *Service) DeleteNode(ctx context.Context, id uuid.UUID) error {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return err
	}
	if settings.Mode != ModeMaster {
		return ErrForbidden
	}
	node, err := s.q.GetFleetNode(ctx, id)
	if err != nil {
		return mapErr(err)
	}
	if node.ConnectionType == ConnLocal {
		return fmt.Errorf("%w: cannot delete local master node", ErrForbidden)
	}
	_ = s.q.RevokeNodeCredentials(ctx, id)
	return mapErr(s.q.SoftDeleteFleetNode(ctx, id))
}

func (s *Service) LocalNodeStatus(ctx context.Context) (NodeStatusPayload, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return NodeStatusPayload{}, err
	}
	snap, _ := s.metrics.Collect()
	apps := AppsDTO{}
	if s.sites != nil {
		t, r, u, e := s.sites.CountApps(ctx)
		if e == nil {
			apps = AppsDTO{Total: t, Running: r, Unhealthy: u}
		}
	}
	name := settings.NodeName
	if name == "" {
		name = snap.Hostname
	}
	hostIP := ""
	if s.hostIP != nil {
		hostIP = strings.TrimSpace(s.hostIP(ctx))
	}
	billing := remoteBillingFromAccounts(s.listBillingAccounts(ctx))
	return NodeStatusPayload{
		NodeUID: settings.NodeUid.String(),
		Name:    name,
		Version: s.appVersion,
		Mode:    settings.Mode,
		Metrics: snap,
		Apps:    apps,
		Status:  StatusOnline,
		HostIP:  hostIP,
		Billing: billing,
	}, nil
}

func remoteBillingFromAccounts(rows []db.BillingAccount) []RemoteBillingAccount {
	out := make([]RemoteBillingAccount, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		item := RemoteBillingAccount{
			ServerIP:  row.ServerIp,
			Provider:  row.Provider,
			Name:      row.CachedName,
			Status:    row.CachedStatus,
			Cost:      row.CachedCost,
			Enabled:   true,
			AlertDays: int(row.AlertDays),
		}
		if row.CachedExpireDate.Valid {
			d := row.CachedExpireDate.Time.Format("2006-01-02")
			item.ExpireDate = &d
			days := int(row.CachedExpireDate.Time.UTC().Truncate(24*time.Hour).Sub(time.Now().UTC().Truncate(24*time.Hour)).Hours() / 24)
			item.DaysLeft = &days
		}
		out = append(out, item)
	}
	return out
}


func validateRemoteURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%w: invalid URL", ErrInvalidInput)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "http":
	default:
		return fmt.Errorf("%w: URL must be http(s)", ErrInvalidInput)
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		// allow for tests/dev
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			return fmt.Errorf("%w: URL host not allowed", ErrInvalidInput)
		}
	}
	return nil
}

func limitRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return fmt.Errorf("too many redirects")
	}
	switch req.URL.Scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("redirect to disallowed scheme")
	}
}

func tokenHashEqual(got, want []byte) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
