package servers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/metrics"
)

type SettingsResponse struct {
	Mode             string `json:"mode"`
	NodeUID          string `json:"node_uid"`
	NodeName         string `json:"node_name"`
	PublicURL        string `json:"public_url"`
	MasterURL        string `json:"master_url"`
	NotificationMode string `json:"notification_mode"`
	HasMasterToken   bool   `json:"has_master_token"`
}

type UpdateSettingsRequest struct {
	Mode             *string `json:"mode"`
	NodeName         *string `json:"node_name"`
	PublicURL        *string `json:"public_url"`
	MasterURL        *string `json:"master_url"`
	NotificationMode *string `json:"notification_mode"`
	EnableMaster     *bool   `json:"enable_master"`
	DisableMaster    *bool   `json:"disable_master"`
}

type MetricsDTO struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedBytes  uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes uint64  `json:"memory_total_bytes"`
	DiskUsedPercent  float64 `json:"disk_used_percent"`
	UptimeSeconds    int64   `json:"uptime_seconds"`
	Load1            float64 `json:"load_1,omitempty"`
	Load5            float64 `json:"load_5,omitempty"`
	Load15           float64 `json:"load_15,omitempty"`
}

type AppsDTO struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Unhealthy int `json:"unhealthy"`
}

type BillingDTO struct {
	CostMinor        int64   `json:"cost_minor"`
	Currency         string  `json:"currency"`
	Period           string  `json:"period"`
	NextDueDate      *string `json:"next_due_date,omitempty"`
	AutoRenew        bool    `json:"auto_renew"`
	Provider         string  `json:"provider_name"`
	ProviderURL      string  `json:"provider_url"`
	MonthlyEquiv     int64   `json:"monthly_equiv_minor"`
	Mode             string  `json:"mode,omitempty"`
	BillingAccountID string  `json:"billing_account_id,omitempty"`
	ServerIP         string  `json:"server_ip,omitempty"`
	CostRaw          string  `json:"cost_raw,omitempty"`
	DaysLeft         *int    `json:"days_left,omitempty"`
	AlertDays        int     `json:"alert_days,omitempty"`
}

// UpdateNodeBillingRequest links a server node to a VPS payment account from /payments.
// Empty billing_account_id clears the link. Manual cost fields are kept for backward compatibility.
type UpdateNodeBillingRequest struct {
	BillingAccountID string `json:"billing_account_id"`
	CostMinor        int64  `json:"cost_minor"`
	Currency         string `json:"currency"`
	Period           string `json:"period"`
	NextDueDate      string `json:"next_due_date"`
	AutoRenew        bool   `json:"auto_renew"`
	AlertDays        int    `json:"alert_days"`
	Provider         string `json:"provider_name"`
	ProviderURL      string `json:"provider_url"`
	Comment          string `json:"comment"`
}

type UpdateNodeRequest struct {
	Name string `json:"name"`
}

type NodeResponse struct {
	ID             uuid.UUID            `json:"id"`
	NodeUID        string               `json:"node_uid"`
	Name           string               `json:"name"`
	Role           string               `json:"role"`
	ConnectionType string               `json:"connection_type"`
	BaseURL        string               `json:"base_url"`
	Status         string               `json:"status"`
	Version        string               `json:"version"`
	AgentVersion   string               `json:"agent_version,omitempty"`
	LastSeenAt     *time.Time           `json:"last_seen_at,omitempty"`
	Capabilities   []string             `json:"capabilities"`
	Metrics        *MetricsDTO          `json:"metrics,omitempty"`
	Applications   *AppsDTO             `json:"applications,omitempty"`
	Billing        *BillingDTO          `json:"billing,omitempty"`
	OpenIncidents  int                  `json:"open_incidents"`
	Hostname       string               `json:"hostname,omitempty"`
	OSName         string               `json:"os_name,omitempty"`
	OSVersion      string               `json:"os_version,omitempty"`
	Kernel         string               `json:"kernel,omitempty"`
	Architecture   string               `json:"architecture,omitempty"`
	Filesystems    []metrics.Filesystem `json:"filesystems,omitempty"`
	Services       []ServiceStatus      `json:"services,omitempty"`
}

type ServiceStatus struct {
	UnitName string `json:"unit_name"`
	State    string `json:"state"`
}

type OverviewResponse struct {
	ServersTotal     int     `json:"servers_total"`
	ServersOnline    int     `json:"servers_online"`
	ServersWarning   int     `json:"servers_warning"`
	ServersOffline   int     `json:"servers_offline"`
	AppsTotal        int     `json:"apps_total"`
	AppsRunning      int     `json:"apps_running"`
	AppsUnhealthy    int     `json:"apps_unhealthy"`
	MonthlyCostMinor int64   `json:"monthly_cost_minor"`
	Currency         string  `json:"currency"`
	NextDueDate      *string `json:"next_due_date,omitempty"`
	OpenIncidents    int     `json:"open_incidents"`
}

type EventResponse struct {
	ID           uuid.UUID  `json:"id"`
	EventID      uuid.UUID  `json:"event_id"`
	NodeID       *uuid.UUID `json:"node_id,omitempty"`
	NodeUID      string     `json:"node_uid,omitempty"`
	EventType    string     `json:"event_type"`
	Severity     string     `json:"severity"`
	ResourceType string     `json:"resource_type"`
	ResourceID   string     `json:"resource_id"`
	Title        string     `json:"title"`
	Message      string     `json:"message"`
	OccurredAt   time.Time  `json:"occurred_at"`
}

type IncidentResponse struct {
	ID          uuid.UUID  `json:"id"`
	NodeID      *uuid.UUID `json:"node_id,omitempty"`
	EventType   string     `json:"event_type"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Count       int        `json:"count"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

type PairingCodeResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PairBarnRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	BaseURL     string `json:"base_url"`
	Code        string `json:"code"`
	PairingCode string `json:"pairing_code"`
}

type PairNodeRequest struct {
	MasterURL  string `json:"master_url"`
	MasterName string `json:"master_name"`
	NodeName   string `json:"node_name"`
	Code       string `json:"code"`
}

type PairNodeResponse struct {
	NodeUID     string   `json:"node_uid"`
	NodeName    string   `json:"node_name"`
	PublicURL   string   `json:"public_url"`
	MasterToken string   `json:"master_token"` // node→master
	NodeToken   string   `json:"node_token"`   // master→node
	Scopes      []string `json:"scopes"`
}

type HeartbeatRequest struct {
	NodeUID      string                 `json:"node_uid"`
	Version      string                 `json:"version"`
	AgentVersion string                 `json:"agent_version"`
	Metrics      metrics.Snapshot       `json:"metrics"`
	Apps         *AppsDTO               `json:"applications,omitempty"`
	Services     []ServiceStatus        `json:"services,omitempty"`
	HostIP       string                 `json:"host_ip,omitempty"`
	Billing      []RemoteBillingAccount `json:"billing,omitempty"`
}

type IngestEvent struct {
	EventID      uuid.UUID       `json:"event_id"`
	EventType    string          `json:"event_type"`
	Severity     string          `json:"severity"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Title        string          `json:"title"`
	Message      string          `json:"message"`
	Payload      json.RawMessage `json:"payload"`
	OccurredAt   time.Time       `json:"occurred_at"`
	NodeUID      string          `json:"node_uid"`
}

type AgentRegisterRequest struct {
	RegistrationToken string           `json:"registration_token"`
	NodeUID           string           `json:"node_uid"`
	Hostname          string           `json:"hostname"`
	AgentVersion      string           `json:"agent_version"`
	Metrics           metrics.Snapshot `json:"metrics"`
}

type AgentRegisterResponse struct {
	NodeToken        string `json:"node_token"`
	MasterURL        string `json:"master_url"`
	HeartbeatSeconds int    `json:"heartbeat_interval_seconds"`
}

type CreateAgentInstallRequest struct {
	Kind         string `json:"kind"` // agent | barn | dockpilot (legacy)
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	PanelURL     string `json:"panel_url"`
	Email        string `json:"email"`
	Purpose      string `json:"purpose"`
	CostMinor    int64  `json:"cost_minor"`
	Currency     string `json:"currency"`
	Period       string `json:"period"`
	NextDueDate  string `json:"next_due_date"`
	AutoRenew    bool   `json:"auto_renew"`
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
	Comment      string `json:"comment"`
}

type InstallationResponse struct {
	ID             uuid.UUID  `json:"id"`
	Status         string     `json:"status"`
	CurrentStep    string     `json:"current_step"`
	SSHFingerprint string     `json:"ssh_fingerprint"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	Username       string     `json:"username"`
	InstallKind    string     `json:"install_kind"`
	PanelURL       string     `json:"panel_url,omitempty"`
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	NodeID         *uuid.UUID `json:"node_id,omitempty"`
}

type NodeStatusPayload struct {
	NodeUID string                 `json:"node_uid"`
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Mode    string                 `json:"mode"`
	Metrics metrics.Snapshot       `json:"metrics"`
	Apps    AppsDTO                `json:"applications"`
	Status  string                 `json:"status"`
	HostIP  string                 `json:"host_ip,omitempty"`
	Billing []RemoteBillingAccount `json:"billing,omitempty"`
}

// RemoteBillingAccount is a credential-free payment snapshot from a managed Barn node.
type RemoteBillingAccount struct {
	ServerIP   string  `json:"server_ip"`
	Provider   string  `json:"provider"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Cost       string  `json:"cost"`
	ExpireDate *string `json:"expire_date,omitempty"`
	DaysLeft   *int    `json:"days_left,omitempty"`
	AlertDays  int     `json:"alert_days,omitempty"`
	Enabled    bool    `json:"enabled"`
}
