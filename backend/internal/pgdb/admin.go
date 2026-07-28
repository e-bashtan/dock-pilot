package pgdb

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/docker"
)

var identPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func quoteIdent(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !identPattern.MatchString(name) {
		return "", fmt.Errorf("%w: invalid identifier %q", ErrInvalidInput, name)
	}
	return `"` + name + `"`, nil
}

func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func (s *Service) containerName(inst db.PdbInstance) string {
	_ = inst
	return "dockpilot-postgres"
}

func (s *Service) volumeName(inst db.PdbInstance) string {
	_ = inst
	return "dockpilot-postgres-data"
}

func (s *Service) adminPassword(inst db.PdbInstance) (string, error) {
	return s.cipher.Decrypt(inst.EncryptedAdminPassword)
}

func (s *Service) execSQL(ctx context.Context, inst db.PdbInstance, database, sql string) error {
	password, err := s.adminPassword(inst)
	if err != nil {
		return fmt.Errorf("decrypt admin password: %w", err)
	}
	var stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: s.containerName(inst),
		Cmd: []string{
			"psql",
			"-v", "ON_ERROR_STOP=1",
			"-U", inst.AdminUser,
			"-d", database,
			"-c", sql,
		},
		Env: []string{"PGPASSWORD=" + password},
	}, nil, io.Discard, &stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("psql exit %d", code)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (s *Service) waitReady(ctx context.Context, inst db.PdbInstance) error {
	password, err := s.adminPassword(inst)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: s.containerName(inst),
		Cmd:           []string{"pg_isready", "-U", inst.AdminUser},
		Env:           []string{"PGPASSWORD=" + password},
	}, nil, io.Discard, &stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "postgres is not ready"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (s *Service) dumpDatabase(ctx context.Context, inst db.PdbInstance, dbName string, w io.Writer) error {
	password, err := s.adminPassword(inst)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: s.containerName(inst),
		Cmd: []string{
			"pg_dump",
			"-U", inst.AdminUser,
			"-d", dbName,
			"--no-owner",
			"--no-acl",
		},
		Env: []string{"PGPASSWORD=" + password},
	}, nil, w, &stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("pg_dump exit %d", code)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (s *Service) restoreDatabase(ctx context.Context, inst db.PdbInstance, dbName string, r io.Reader) error {
	password, err := s.adminPassword(inst)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: s.containerName(inst),
		Cmd: []string{
			"psql",
			"-v", "ON_ERROR_STOP=1",
			"-U", inst.AdminUser,
			"-d", dbName,
		},
		Env: []string{"PGPASSWORD=" + password},
	}, r, io.Discard, &stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("psql restore exit %d", code)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// openSQLDump returns a reader for a plain SQL dump, transparently gunzipping if needed.
func openSQLDump(r io.Reader) (io.Reader, io.Closer, error) {
	br := bufio.NewReader(r)
	magic, err := br.Peek(2)
	if err != nil && err != io.EOF {
		return nil, nil, err
	}
	if len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid gzip dump: %v", ErrInvalidInput, err)
		}
		return gz, gz, nil
	}
	return br, nil, nil
}

func validateDBName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: database name is required", ErrInvalidInput)
	}
	if !identPattern.MatchString(name) {
		return fmt.Errorf("%w: database name must be alphanumeric/underscore", ErrInvalidInput)
	}
	if len(name) > 63 {
		return fmt.Errorf("%w: database name too long", ErrInvalidInput)
	}
	return nil
}

func validateRoleName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: role name is required", ErrInvalidInput)
	}
	if !identPattern.MatchString(name) {
		return fmt.Errorf("%w: role name must be alphanumeric/underscore", ErrInvalidInput)
	}
	lower := strings.ToLower(name)
	if lower == "postgres" || lower == "pg_signal_backend" {
		return fmt.Errorf("%w: reserved role name", ErrInvalidInput)
	}
	return nil
}

func generatePassword(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if n < 16 {
		n = 16
	}
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(b), nil
}

func slugify(name string) string {
	s := strings.ToLower(name)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "postgres"
	}
	if len(out) > 48 {
		out = out[:48]
		out = strings.Trim(out, "-")
	}
	return out
}
