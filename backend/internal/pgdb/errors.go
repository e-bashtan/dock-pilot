package pgdb

import "errors"

var (
	ErrNotFound          = errors.New("postgres resource not found")
	ErrSlugConflict      = errors.New("postgres instance slug already exists")
	ErrAlreadyConfigured = errors.New("only one postgres instance is allowed on this host")
	ErrInvalidInput      = errors.New("invalid postgres input")
)
