package billing

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrMigration     = errors.New("billing tables missing — run migrations")
	ErrNotConfigured = errors.New("billing account not configured")
)

func wrapInvalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, msg)
}
