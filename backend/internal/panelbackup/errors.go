package panelbackup

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotConfigured = errors.New("panel backup not configured")
	ErrMigration     = errors.New("run database migrations (panel_backup_settings missing)")
)
