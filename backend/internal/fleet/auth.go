package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/db"
)

type ctxKey int

const (
	ctxNodeID ctxKey = iota + 1
	ctxScopes
)

func NodeIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxNodeID).(uuid.UUID)
	return id, ok
}

func ScopesFromContext(ctx context.Context) []string {
	s, _ := ctx.Value(ctxScopes).([]string)
	return s
}

func (s *Service) FleetTokenAuth(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := extractBearer(r)
			if raw == "" {
				writeFleetUnauthorized(w)
				return
			}
			cred, err := s.q.GetInboundCredentialByHash(r.Context(), HashToken(raw))
			if err != nil {
				writeFleetUnauthorized(w)
				return
			}
			if !tokenHashEqual(cred.TokenHash, HashToken(raw)) {
				writeFleetUnauthorized(w)
				return
			}
			if requiredScope != "" && !HasScope(cred.Scopes, requiredScope) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"forbidden"}`))
				return
			}
			node, err := s.q.GetFleetNode(r.Context(), cred.NodeID)
			if err != nil || node.DeletedAt.Valid {
				writeFleetUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxNodeID, cred.NodeID)
			ctx = context.WithValue(ctx, ctxScopes, append([]string(nil), cred.Scopes...))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeFleetUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return strings.TrimSpace(r.Header.Get("X-Fleet-Token"))
}

// storeNodeInboundToken saves hash of the token Master will use to call this node.
func (s *Service) storeNodeInboundToken(ctx context.Context, nodeToken string) error {
	settings, err := s.ensureSettings(ctx)
	if err != nil {
		return err
	}
	local, err := s.q.GetFleetNodeByUID(ctx, settings.NodeUid)
	if err != nil {
		caps, _ := json.Marshal(BarnCapabilities())
		now := time.Now().UTC()
		local, err = s.q.CreateFleetNode(ctx, db.CreateFleetNodeParams{
			NodeUid:        settings.NodeUid,
			Name:           settings.NodeName,
			Role:           RoleNode,
			ConnectionType: ConnLocal,
			BaseUrl:        settings.PublicUrl,
			Status:         StatusOnline,
			Capabilities:   caps,
			Version:        s.appVersion,
			AgentVersion:   "",
			LastSeenAt:     pgTimestamptz(now),
			PairedAt:       pgTimestamptz(now),
			Metadata:       []byte("{}"),
		})
		if err != nil {
			return mapErr(err)
		}
	}
	_, err = s.q.CreateFleetCredential(ctx, db.CreateFleetCredentialParams{
		NodeID:         local.ID,
		Direction:      "inbound",
		Purpose:        "master_to_node",
		Scopes:         []string{ScopeStatusRead, ScopeAppsRead, ScopeBackupsRead, ScopeVersionRead},
		TokenHash:      HashToken(nodeToken),
		EncryptedToken: nil,
	})
	return mapErr(err)
}
