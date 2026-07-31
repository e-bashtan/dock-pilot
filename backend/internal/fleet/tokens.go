package fleet

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ModeStandalone  = "standalone"
	ModeMaster      = "master"
	ModeManagedNode = "managed_node"

	ConnLocal = "local"
	ConnBarn  = "barn"
	ConnAgent = "agent"

	RoleMaster = "master"
	RoleNode   = "node"
	RoleAgent  = "agent"

	StatusOnline  = "online"
	StatusWarning = "warning"
	StatusOffline = "offline"

	NotifyLocal    = "local"
	NotifyMaster   = "master"
	NotifyDisabled = "disabled"

	ScopeStatusRead   = "fleet:status:read"
	ScopeAppsRead     = "fleet:apps:read"
	ScopeBackupsRead  = "fleet:backups:read"
	ScopeVersionRead  = "fleet:version:read"
	ScopeHeartbeatWrite = "fleet:heartbeat:write"
	ScopeEventsWrite  = "fleet:events:write"

	WarningAfter  = 90 * time.Second
	OfflineAfter  = 180 * time.Second
	PairingTTL    = 10 * time.Minute
	RegTokenTTL   = 10 * time.Minute
)

func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func GenerateToken(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}

func GeneratePairingCode() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Crockford-ish base32 without padding, uppercased chunks.
	s := strings.ToUpper(base64.RawURLEncoding.EncodeToString(b))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	if len(s) > 12 {
		s = s[:12]
	}
	return fmt.Sprintf("%s-%s", s[:6], s[6:]), nil
}

func NewEventID() uuid.UUID {
	return uuid.New()
}

func DedupKey(nodeID uuid.UUID, eventType, resourceType, resourceID string) string {
	return fmt.Sprintf("%s|%s|%s|%s", nodeID.String(), eventType, resourceType, resourceID)
}

func ComputeStatus(lastSeen *time.Time, now time.Time) string {
	if lastSeen == nil {
		return StatusOffline
	}
	age := now.Sub(*lastSeen)
	if age <= WarningAfter {
		return StatusOnline
	}
	if age <= OfflineAfter {
		return StatusWarning
	}
	return StatusOffline
}

func HasScope(scopes []string, need string) bool {
	for _, s := range scopes {
		if s == need || s == "*" {
			return true
		}
	}
	return false
}

func MasterCapabilities() []string {
	return []string{
		"system_metrics", "barn_apps", "deployments", "postgres", "backups", "billing",
	}
}

func AgentCapabilities() []string {
	return []string{"system_metrics", "systemd_units"}
}

func BarnCapabilities() []string {
	return []string{
		"system_metrics", "barn_apps", "deployments", "postgres", "backups", "billing",
	}
}

// NormalizeConnectionType maps legacy "dockpilot" to "barn"
func NormalizeConnectionType(connType string) string {
	if connType == "dockpilot" {
		return ConnBarn
	}
	return connType
}

// IsBarnPanel reports whether the node is a full Barn panel (incl. legacy dockpilot rows).
func IsBarnPanel(connType string) bool {
	return NormalizeConnectionType(connType) == ConnBarn
}

// NormalizeInstallKind maps legacy "dockpilot" to "barn"
func NormalizeInstallKind(kind string) string {
	if kind == "dockpilot" {
		return "barn"
	}
	return kind
}
