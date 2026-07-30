package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/metrics"
	"github.com/ebash/dock-pilot/backend/internal/secrets"
)

type SiteHealthProvider interface {
	CountApps(ctx context.Context) (total, running, unhealthy int, err error)
}

type Notifier interface {
	SendText(ctx context.Context, text string) error
}

// HostIPFunc returns the master's public/egress IP for matching VPS billing accounts.
type HostIPFunc func(ctx context.Context) string

type Service struct {
	q          *db.Queries
	pool       DBExec
	cipher     *secrets.Cipher
	logger     *slog.Logger
	metrics    *metrics.Collector
	sites      SiteHealthProvider
	notify     Notifier
	hostIP     HostIPFunc
	appVersion string
	agentDir   string // path to embedded agent binaries inside API image

	installMu sync.Mutex
	installs  map[uuid.UUID]*installSecret // in-memory SSH passwords
}

// DBExec runs schema-ensure statements (subset of pgxpool.Pool).
type DBExec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}


type installSecret struct {
	password  string
	expiresAt time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewService(
	q *db.Queries,
	pool DBExec,
	cipher *secrets.Cipher,
	logger *slog.Logger,
	hostRoot string,
	sites SiteHealthProvider,
	notify Notifier,
	appVersion string,
	agentDir string,
	hostIP HostIPFunc,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		q:          q,
		pool:       pool,
		cipher:     cipher,
		logger:     logger,
		metrics:    metrics.New(hostRoot),
		sites:      sites,
		notify:     notify,
		hostIP:     hostIP,
		appVersion: appVersion,
		agentDir:   agentDir,
		installs:   map[uuid.UUID]*installSecret{},
	}
}

// ensureInstallSchema applies 00017 columns if the migrate image was skipped on upgrade.
func (s *Service) ensureInstallSchema(ctx context.Context) error {
	if s.pool == nil {
		return nil
	}
	stmts := []string{
		`ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS install_kind TEXT NOT NULL DEFAULT 'agent'`,
		`ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS panel_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS cert_email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range stmts {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return mapErr(err)
		}
	}
	_, _ = s.pool.Exec(ctx, `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fleet_installations_install_kind_check'
  ) THEN
    ALTER TABLE fleet_installations
      ADD CONSTRAINT fleet_installations_install_kind_check
      CHECK (install_kind IN ('agent', 'dockpilot'));
  END IF;
END $$`)
	return nil
}

func (s *Service) ensureSettings(ctx context.Context) (db.FleetSetting, error) {
	_ = s.q.EnsureFleetSettings(ctx)
	row, err := s.q.GetFleetSettings(ctx)
	if err != nil {
		return db.FleetSetting{}, mapErr(err)
	}
	return row, nil
}

func (s *Service) GetSettings(ctx context.Context) (SettingsResponse, error) {
	row, err := s.ensureSettings(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	return SettingsResponse{
		Mode:             row.Mode,
		NodeUID:          row.NodeUid.String(),
		NodeName:         row.NodeName,
		PublicURL:        row.PublicUrl,
		MasterURL:        row.MasterUrl,
		NotificationMode: row.NotificationMode,
		HasMasterToken:   len(row.EncryptedMasterToken) > 0,
	}, nil
}

func (s *Service) UpdateSettings(ctx context.Context, req UpdateSettingsRequest) (SettingsResponse, error) {
	row, err := s.ensureSettings(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}

	mode := row.Mode
	name := row.NodeName
	publicURL := row.PublicUrl
	masterURL := row.MasterUrl
	notifyMode := row.NotificationMode
	encToken := row.EncryptedMasterToken

	if req.EnableMaster != nil && *req.EnableMaster {
		if mode == ModeManagedNode {
			return SettingsResponse{}, ErrCannotNest
		}
		mode = ModeMaster
		if req.NodeName != nil && strings.TrimSpace(*req.NodeName) != "" {
			name = strings.TrimSpace(*req.NodeName)
		}
		if name == "" {
			name = "Master"
		}
		if req.PublicURL != nil {
			publicURL = strings.TrimSpace(*req.PublicURL)
		}
		if err := validatePublicURL(publicURL); err != nil {
			return SettingsResponse{}, err
		}
		if _, err := s.ensureLocalNode(ctx, row.NodeUid, name); err != nil {
			return SettingsResponse{}, err
		}
	}

	if req.DisableMaster != nil && *req.DisableMaster {
		n, err := s.q.CountActiveRemoteFleetNodes(ctx)
		if err != nil {
			return SettingsResponse{}, mapErr(err)
		}
		if n > 0 {
			return SettingsResponse{}, ErrHasRemotes
		}
		mode = ModeStandalone
	}

	if req.Mode != nil {
		m := strings.TrimSpace(*req.Mode)
		switch m {
		case ModeStandalone, ModeMaster, ModeManagedNode:
			if m == ModeStandalone && mode == ModeMaster {
				n, err := s.q.CountActiveRemoteFleetNodes(ctx)
				if err != nil {
					return SettingsResponse{}, mapErr(err)
				}
				if n > 0 {
					return SettingsResponse{}, ErrHasRemotes
				}
			}
			mode = m
		default:
			return SettingsResponse{}, ErrMode
		}
	}
	if req.NodeName != nil {
		name = strings.TrimSpace(*req.NodeName)
	}
	if req.PublicURL != nil {
		publicURL = strings.TrimSpace(*req.PublicURL)
		if publicURL != "" {
			if err := validatePublicURL(publicURL); err != nil {
				return SettingsResponse{}, err
			}
		}
	}
	if req.MasterURL != nil {
		masterURL = strings.TrimSpace(*req.MasterURL)
	}
	if req.NotificationMode != nil {
		nm := strings.TrimSpace(*req.NotificationMode)
		switch nm {
		case NotifyLocal, NotifyMaster, NotifyDisabled:
			notifyMode = nm
		default:
			return SettingsResponse{}, ErrInvalidInput
		}
	}

	updated, err := s.q.UpdateFleetSettings(ctx, db.UpdateFleetSettingsParams{
		ID:                   1,
		Mode:                 mode,
		NodeName:             name,
		PublicUrl:            publicURL,
		MasterUrl:            masterURL,
		NotificationMode:     notifyMode,
		EncryptedMasterToken: encToken,
	})
	if err != nil {
		return SettingsResponse{}, mapErr(err)
	}
	if mode == ModeMaster {
		_, _ = s.ensureLocalNode(ctx, updated.NodeUid, updated.NodeName)
	}
	return s.GetSettings(ctx)
}

func (s *Service) ensureLocalNode(ctx context.Context, nodeUID uuid.UUID, name string) (db.FleetNode, error) {
	existing, err := s.q.GetLocalFleetNode(ctx)
	if err == nil {
		if existing.Name != name && name != "" {
			return s.q.UpdateFleetNode(ctx, db.UpdateFleetNodeParams{
				ID:       existing.ID,
				Name:     name,
				BaseUrl:  existing.BaseUrl,
				Metadata: existing.Metadata,
			})
		}
		return existing, nil
	}
	if !isNoRows(err) {
		return db.FleetNode{}, mapErr(err)
	}
	caps, _ := json.Marshal(MasterCapabilities())
	now := time.Now().UTC()
	return s.q.CreateFleetNode(ctx, db.CreateFleetNodeParams{
		NodeUid:        nodeUID,
		Name:           name,
		Role:           RoleMaster,
		ConnectionType: ConnLocal,
		BaseUrl:        "",
		Status:         StatusOnline,
		Capabilities:   caps,
		Version:        s.appVersion,
		AgentVersion:   "",
		LastSeenAt:     pgTimestamptz(now),
		PairedAt:       pgTimestamptz(now),
		Metadata:       []byte("{}"),
	})
}

func (s *Service) Overview(ctx context.Context) (OverviewResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return OverviewResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return OverviewResponse{}, ErrForbidden
	}
	nodes, err := s.ListNodes(ctx)
	if err != nil {
		return OverviewResponse{}, err
	}
	out := OverviewResponse{Currency: "RUB"}
	var nextDue *time.Time
	for _, n := range nodes {
		out.ServersTotal++
		switch n.Status {
		case StatusOnline:
			out.ServersOnline++
		case StatusWarning:
			out.ServersWarning++
		default:
			out.ServersOffline++
		}
		if n.Applications != nil {
			out.AppsTotal += n.Applications.Total
			out.AppsRunning += n.Applications.Running
			out.AppsUnhealthy += n.Applications.Unhealthy
		}
		out.OpenIncidents += n.OpenIncidents
		if n.Billing != nil {
			out.MonthlyCostMinor += n.Billing.MonthlyEquiv
			if n.Billing.Currency != "" {
				out.Currency = n.Billing.Currency
			}
			if n.Billing.NextDueDate != nil {
				if t, err := time.Parse("2006-01-02", *n.Billing.NextDueDate); err == nil {
					if nextDue == nil || t.Before(*nextDue) {
						tt := t
						nextDue = &tt
					}
				}
			}
		}
	}
	if nextDue != nil {
		d := nextDue.Format("2006-01-02")
		out.NextDueDate = &d
	}
	return out, nil
}

func (s *Service) ListNodes(ctx context.Context) ([]NodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return nil, err
	}
	if settings.Mode != ModeMaster {
		return nil, ErrForbidden
	}
	_, _ = s.ensureLocalNode(ctx, settings.NodeUid, settings.NodeName)
	_ = s.refreshLocalSnapshot(ctx)

	rows, err := s.q.ListFleetNodes(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	accounts := s.listBillingAccounts(ctx)
	claimed := map[uuid.UUID]bool{}
	localIP := ""
	if s.hostIP != nil {
		localIP = strings.TrimSpace(s.hostIP(ctx))
	}
	out := make([]NodeResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.toNodeResponse(ctx, row, accounts, claimed, localIP))
	}
	return out, nil
}

func (s *Service) GetNode(ctx context.Context, id uuid.UUID) (NodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return NodeResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return NodeResponse{}, ErrForbidden
	}
	row, err := s.q.GetFleetNode(ctx, id)
	if err != nil {
		return NodeResponse{}, mapErr(err)
	}
	if row.ConnectionType == ConnLocal {
		_ = s.refreshLocalSnapshot(ctx)
		row, _ = s.q.GetFleetNode(ctx, id)
	}
	accounts := s.listBillingAccounts(ctx)
	claimed := map[uuid.UUID]bool{}
	localIP := ""
	if s.hostIP != nil {
		localIP = strings.TrimSpace(s.hostIP(ctx))
	}
	return s.toNodeResponse(ctx, row, accounts, claimed, localIP), nil
}

func (s *Service) UpdateNode(ctx context.Context, id uuid.UUID, req UpdateNodeRequest) (NodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return NodeResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return NodeResponse{}, ErrForbidden
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return NodeResponse{}, fmt.Errorf("%w: name required", ErrInvalidInput)
	}
	row, err := s.q.GetFleetNode(ctx, id)
	if err != nil {
		return NodeResponse{}, mapErr(err)
	}
	_, err = s.q.UpdateFleetNode(ctx, db.UpdateFleetNodeParams{
		ID:       id,
		Name:     name,
		BaseUrl:  row.BaseUrl,
		Metadata: row.Metadata,
	})
	if err != nil {
		return NodeResponse{}, mapErr(err)
	}
	return s.GetNode(ctx, id)
}

func (s *Service) UpdateNodeBilling(ctx context.Context, id uuid.UUID, req UpdateNodeBillingRequest) (NodeResponse, error) {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return NodeResponse{}, err
	}
	if settings.Mode != ModeMaster {
		return NodeResponse{}, ErrForbidden
	}
	if _, err := s.q.GetFleetNode(ctx, id); err != nil {
		return NodeResponse{}, mapErr(err)
	}

	accountID := strings.TrimSpace(req.BillingAccountID)
	if accountID != "" {
		aid, err := uuid.Parse(accountID)
		if err != nil {
			return NodeResponse{}, fmt.Errorf("%w: billing_account_id", ErrInvalidInput)
		}
		acc, err := s.q.GetBillingAccount(ctx, aid)
		if err != nil {
			return NodeResponse{}, mapErr(err)
		}
		_, err = s.q.UpsertFleetNodeBilling(ctx, db.UpsertFleetNodeBillingParams{
			NodeID:            id,
			BillingAccountID:  pgUUID(acc.ID),
			Mode:              "planetahost",
			ProviderName:      acc.Provider,
			ProviderUrl:       acc.BillmgrUrl,
			ExternalServiceID: "",
			CostMinor:         0,
			Currency:          "RUB",
			Period:            "monthly",
			NextDueDate:       pgtype.Date{},
			AutoRenew:         false,
			Comment:           strings.TrimSpace(req.Comment),
		})
		if err != nil {
			return NodeResponse{}, mapErr(err)
		}
		return s.GetNode(ctx, id)
	}

	// Explicit unlink / legacy manual entry when no account id is provided.
	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "RUB"
	}
	period := strings.TrimSpace(req.Period)
	if period == "" {
		period = "monthly"
	}
	switch period {
	case "monthly", "quarterly", "yearly", "custom":
	default:
		return NodeResponse{}, ErrInvalidInput
	}
	var due pgtype.Date
	if d := strings.TrimSpace(req.NextDueDate); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return NodeResponse{}, fmt.Errorf("%w: next_due_date", ErrInvalidInput)
		}
		due = pgtype.Date{Time: t, Valid: true}
	}
	_, err = s.q.UpsertFleetNodeBilling(ctx, db.UpsertFleetNodeBillingParams{
		NodeID:            id,
		BillingAccountID:  pgtype.UUID{},
		Mode:              "manual",
		ProviderName:      strings.TrimSpace(req.Provider),
		ProviderUrl:       strings.TrimSpace(req.ProviderURL),
		ExternalServiceID: "",
		CostMinor:         req.CostMinor,
		Currency:          currency,
		Period:            period,
		NextDueDate:       due,
		AutoRenew:         req.AutoRenew,
		Comment:           strings.TrimSpace(req.Comment),
	})
	if err != nil {
		return NodeResponse{}, mapErr(err)
	}
	return s.GetNode(ctx, id)
}

func (s *Service) refreshLocalSnapshot(ctx context.Context) error {
	local, err := s.q.GetLocalFleetNode(ctx)
	if err != nil {
		return err
	}
	snap, err := s.metrics.Collect()
	if err != nil {
		return err
	}
	apps := AppsDTO{}
	if s.sites != nil {
		t, r, u, err := s.sites.CountApps(ctx)
		if err == nil {
			apps = AppsDTO{Total: t, Running: r, Unhealthy: u}
		}
	}
	hostIP := ""
	if s.hostIP != nil {
		hostIP = strings.TrimSpace(s.hostIP(ctx))
	}
	payload, _ := json.Marshal(map[string]any{
		"metrics":      snap,
		"applications": apps,
		"host_ip":      hostIP,
		"billing":      remoteBillingFromAccounts(s.listBillingAccounts(ctx)),
	})
	now := time.Now().UTC()
	_, _ = s.q.InsertFleetSnapshot(ctx, db.InsertFleetSnapshotParams{
		NodeID:           local.ID,
		CollectedAt:      now,
		CpuPercent:       pgFloat8(snap.CPUPercent),
		MemoryUsedBytes:  pgInt8(int64(snap.MemoryUsed)),
		MemoryTotalBytes: pgInt8(int64(snap.MemoryTotal)),
		DiskUsedPercent:  pgFloat8(snap.DiskUsedPct),
		UptimeSeconds:    pgInt8(snap.UptimeSeconds),
		AppsTotal:        pgInt4(apps.Total),
		AppsRunning:      pgInt4(apps.Running),
		AppsUnhealthy:    pgInt4(apps.Unhealthy),
		Payload:          payload,
	})
	_, _ = s.q.UpdateFleetNodeHeartbeat(ctx, db.UpdateFleetNodeHeartbeatParams{
		ID:           local.ID,
		Status:       StatusOnline,
		LastSeenAt:   pgTimestamptz(now),
		Version:      s.appVersion,
		AgentVersion: "",
	})
	return nil
}

func (s *Service) toNodeResponse(
	ctx context.Context,
	row db.FleetNode,
	accounts []db.BillingAccount,
	claimed map[uuid.UUID]bool,
	localIP string,
) NodeResponse {
	var caps []string
	_ = json.Unmarshal(row.Capabilities, &caps)
	resp := NodeResponse{
		ID:             row.ID,
		NodeUID:        row.NodeUid.String(),
		Name:           row.Name,
		Role:           row.Role,
		ConnectionType: row.ConnectionType,
		BaseURL:        row.BaseUrl,
		Status:         row.Status,
		Version:        row.Version,
		AgentVersion:   row.AgentVersion,
		Capabilities:   caps,
	}
	if row.LastSeenAt.Valid {
		t := row.LastSeenAt.Time
		resp.LastSeenAt = &t
		if row.ConnectionType != ConnLocal {
			// Derive live status from last contact so UI matches heartbeat age.
			resp.Status = ComputeStatus(&t, time.Now().UTC())
		}
	}
	if snap, err := s.q.GetLatestFleetSnapshot(ctx, row.ID); err == nil {
		m := &MetricsDTO{}
		if snap.CpuPercent.Valid {
			m.CPUPercent = snap.CpuPercent.Float64
		}
		if snap.MemoryUsedBytes.Valid {
			m.MemoryUsedBytes = uint64(snap.MemoryUsedBytes.Int64)
		}
		if snap.MemoryTotalBytes.Valid {
			m.MemoryTotalBytes = uint64(snap.MemoryTotalBytes.Int64)
		}
		if snap.DiskUsedPercent.Valid {
			m.DiskUsedPercent = snap.DiskUsedPercent.Float64
		}
		if snap.UptimeSeconds.Valid {
			m.UptimeSeconds = snap.UptimeSeconds.Int64
		}
		resp.Metrics = m
		if snap.AppsTotal.Valid || snap.AppsRunning.Valid || snap.AppsUnhealthy.Valid {
			resp.Applications = &AppsDTO{
				Total:     int(snap.AppsTotal.Int32),
				Running:   int(snap.AppsRunning.Int32),
				Unhealthy: int(snap.AppsUnhealthy.Int32),
			}
		}
		var payload map[string]json.RawMessage
		if json.Unmarshal(snap.Payload, &payload) == nil {
			if raw, ok := payload["metrics"]; ok {
				var full metrics.Snapshot
				if json.Unmarshal(raw, &full) == nil {
					resp.Hostname = full.Hostname
					resp.OSName = full.OSName
					resp.OSVersion = full.OSVersion
					resp.Kernel = full.Kernel
					resp.Architecture = full.Architecture
					resp.Filesystems = full.Filesystems
					if resp.Metrics != nil {
						resp.Metrics.Load1 = full.Load1
						resp.Metrics.Load5 = full.Load5
						resp.Metrics.Load15 = full.Load15
					}
				}
			}
			if raw, ok := payload["services"]; ok {
				var svcs []ServiceStatus
				if json.Unmarshal(raw, &svcs) == nil {
					resp.Services = svcs
				}
			}
		}
	}
	if n, err := s.q.CountOpenIncidentsByNode(ctx, pgtype.UUID{Bytes: row.ID, Valid: true}); err == nil {
		resp.OpenIncidents = int(n)
	}
	resp.Billing = s.resolveNodeBilling(ctx, row, accounts, claimed, localIP)
	return resp
}

func monthlyEquiv(cost int64, period string) int64 {
	switch period {
	case "quarterly":
		return cost / 3
	case "yearly":
		return cost / 12
	default:
		return cost
	}
}

func validatePublicURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%w: public_url required", ErrInvalidInput)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%w: invalid public_url", ErrInvalidInput)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "http":
	default:
		return fmt.Errorf("%w: public_url must be http(s)", ErrInvalidInput)
	}
	return nil
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return ErrNotFound
	}
	msg := err.Error()
	if strings.Contains(msg, "fleet_") && strings.Contains(msg, "does not exist") {
		return ErrMigration
	}
	return err
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows || err.Error() == "no rows in result set"
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgFloat8(v float64) pgtype.Float8 {
	return pgtype.Float8{Float64: v, Valid: true}
}

func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func pgInt4(v int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

// SuppressLocalAlerts implements notifications.LocalAlertGate.
// Managed nodes with notification_mode=master|disabled skip local Telegram;
// events are delivered to Master via outbox instead (when mode=master).
func (s *Service) SuppressLocalAlerts(ctx context.Context) bool {
	row, err := s.ensureSettings(ctx)
	if err != nil {
		return false
	}
	if row.Mode != ModeManagedNode {
		return false
	}
	return row.NotificationMode == NotifyMaster || row.NotificationMode == NotifyDisabled
}

func (s *Service) Mode(ctx context.Context) (string, error) {
	row, err := s.ensureSettings(ctx)
	if err != nil {
		return ModeStandalone, err
	}
	return row.Mode, nil
}
