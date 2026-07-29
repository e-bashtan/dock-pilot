package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	deploysvc "github.com/ebash/dock-pilot/backend/internal/deployments"
	"github.com/ebash/dock-pilot/backend/internal/billing"
	"github.com/ebash/dock-pilot/backend/internal/fleet"
	notifpkg "github.com/ebash/dock-pilot/backend/internal/notifications"
	"github.com/ebash/dock-pilot/backend/internal/panelbackup"
	"github.com/ebash/dock-pilot/backend/internal/pgdb"
	secretpkg "github.com/ebash/dock-pilot/backend/internal/secrets"
	sitesvc "github.com/ebash/dock-pilot/backend/internal/sites"
	"github.com/ebash/dock-pilot/backend/internal/system"
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
		errors.Is(err, fleet.ErrNotFound):
		status = http.StatusNotFound
		msg = err.Error()
	case errors.Is(err, sitesvc.ErrSlugConflict),
		errors.Is(err, pgdb.ErrSlugConflict),
		errors.Is(err, pgdb.ErrAlreadyConfigured),
		errors.Is(err, system.ErrUpgradeBusy),
		errors.Is(err, fleet.ErrConflict),
		errors.Is(err, fleet.ErrHasRemotes),
		errors.Is(err, fleet.ErrAlreadyPaired),
		errors.Is(err, fleet.ErrCannotNest):
		status = http.StatusConflict
		msg = err.Error()
	case errors.Is(err, fleet.ErrUnauthorized):
		status = http.StatusUnauthorized
		msg = err.Error()
	case errors.Is(err, fleet.ErrForbidden),
		errors.Is(err, fleet.ErrScope):
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
		errors.Is(err, fleet.ErrInvalidInput),
		errors.Is(err, fleet.ErrMode),
		errors.Is(err, fleet.ErrNotConfigured),
		errors.Is(err, fleet.ErrMigration),
		errors.Is(err, fleet.ErrPairingExpired):
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
