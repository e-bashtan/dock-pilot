package pgdb

import "errors"

var (
	ErrNotFound     = errors.New("postgres resource not found")
	ErrSlugConflict = errors.New("postgres instance slug already exists")
	ErrInvalidInput = errors.New("invalid postgres input")
)
