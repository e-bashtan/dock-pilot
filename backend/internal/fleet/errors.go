package fleet

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrForbidden      = errors.New("forbidden")
	ErrConflict       = errors.New("conflict")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrNotConfigured  = errors.New("fleet not configured")
	ErrMigration      = errors.New("fleet tables missing — run migrations")
	ErrMode           = errors.New("invalid fleet mode")
	ErrHasRemotes     = errors.New("disconnect remote servers before disabling master")
	ErrAlreadyPaired  = errors.New("already paired to a master")
	ErrCannotNest     = errors.New("cannot nest masters")
	ErrPairingExpired = errors.New("pairing code expired or invalid")
	ErrScope          = errors.New("insufficient fleet token scope")
)
