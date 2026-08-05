package servers

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrForbidden      = errors.New("forbidden")
	ErrConflict       = errors.New("conflict")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrNotConfigured  = errors.New("servers mode not configured")
	ErrMigration      = errors.New("servers schema incomplete — run: docker compose run --rm migrate (need through 00023_rename_fleet_to_servers)")
	ErrMode           = errors.New("invalid servers mode")
	ErrHasRemotes     = errors.New("disconnect remote servers before disabling master")
	ErrAlreadyPaired  = errors.New("already paired to a master")
	ErrCannotNest     = errors.New("cannot nest masters")
	ErrPairingExpired = errors.New("pairing code expired or invalid")
	ErrScope          = errors.New("insufficient node token scope")
)
