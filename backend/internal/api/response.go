package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ebash/barn/backend/internal/billing"
	deploysvc "github.com/ebash/barn/backend/internal/deployments"
	notifpkg "github.com/ebash/barn/backend/internal/notifications"
	"github.com/ebash/barn/backend/internal/panelbackup"
	"github.com/ebash/barn/backend/internal/pgdb"
	secretpkg "github.com/ebash/barn/backend/internal/secrets"
	"github.com/ebash/barn/backend/internal/servers"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
	"github.com/ebash/barn/backend/internal/system"
)

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	msg := "internal server error"

	switch {
	case errors.Is(err, sitesvc.ErrNotFound),
		errors.Is(err, deploysvc.ErrNotFound),
		errors.Is(err, secretpkg.ErrNotFound),
		errors.Is(err, notifpkg.ErrNotFound),
		errors.Is(err, pgdb.ErrNotFound),
		errors.Is(err, billing.ErrNotFound),
		errors.Is(err, servers.ErrNotFound):
		status = http.StatusNotFound
		msg = err.Error()
	case errors.Is(err, sitesvc.ErrSlugConflict),
		errors.Is(err, pgdb.ErrSlugConflict),
		errors.Is(err, pgdb.ErrAlreadyConfigured),
		errors.Is(err, system.ErrUpgradeBusy),
		errors.Is(err, servers.ErrConflict),
		errors.Is(err, servers.ErrHasRemotes),
		errors.Is(err, servers.ErrAlreadyPaired),
		errors.Is(err, servers.ErrCannotNest):
		status = http.StatusConflict
		msg = err.Error()
	case errors.Is(err, servers.ErrUnauthorized):
		status = http.StatusUnauthorized
		msg = err.Error()
	case errors.Is(err, servers.ErrForbidden),
		errors.Is(err, servers.ErrScope):
		status = http.StatusForbidden
		msg = err.Error()
	case errors.Is(err, sitesvc.ErrInvalidInput),
		errors.Is(err, secretpkg.ErrInvalidInput),
		errors.Is(err, notifpkg.ErrInvalidInput),
		errors.Is(err, notifpkg.ErrNotConfigured),
		errors.Is(err, notifpkg.ErrMigration),
		errors.Is(err, pgdb.ErrInvalidInput),
		errors.Is(err, panelbackup.ErrInvalidInput),
		errors.Is(err, panelbackup.ErrNotConfigured),
		errors.Is(err, panelbackup.ErrMigration),
		errors.Is(err, billing.ErrInvalidInput),
		errors.Is(err, billing.ErrMigration),
		errors.Is(err, billing.ErrNotConfigured),
		errors.Is(err, system.ErrUpgradeNotAvail),
		errors.Is(err, system.ErrUpgradeStartFail),
		errors.Is(err, servers.ErrInvalidInput),
		errors.Is(err, servers.ErrMode),
		errors.Is(err, servers.ErrNotConfigured),
		errors.Is(err, servers.ErrMigration),
		errors.Is(err, servers.ErrPairingExpired):
		status = http.StatusBadRequest
		msg = err.Error()
	default:
		if err != nil && err.Error() != "" {
			// Authenticated API — surface actionable errors (Telegram, decrypt, DB hints).
			msg = err.Error()
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "telegram") ||
				strings.Contains(lower, "decrypt") ||
				strings.Contains(lower, "migration") {
				status = http.StatusBadRequest
			}
		}
	}

	writeJSON(w, status, errorBody{Error: msg})
}
