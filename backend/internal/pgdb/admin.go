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
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
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
	c, _ := s.resolvePGNames(context.Background())
	return c
}

func (s *Service) volumeName(inst db.PdbInstance) string {
	_ = inst
	_, v := s.resolvePGNames(context.Background())
	return v
}

// resolvePGNames prefers an already-running legacy postgres container/volume.
func (s *Service) resolvePGNames(ctx context.Context) (container, volume string) {
	st, err := s.docker.InspectContainer(ctx, "dock-pilot-postgres", "dockpilot-postgres", "barn-postgres")
	if err == nil && st.Found {
		switch st.Container {
		case "dock-pilot-postgres":
			return "dock-pilot-postgres", "dock-pilot_dock_pilot_pg"
		case "dockpilot-postgres":
			return "dockpilot-postgres", "dockpilot-postgres-data"
		default:
			return "barn-postgres", "barn-postgres-data"
		}
	}
	return "barn-postgres", "barn-postgres-data"
}

// AdminExecCreds returns the managed Postgres container name and superuser password.
// Used by panel backup when DATABASE_URL still has a legacy user (e.g. dockpilot).
func (s *Service) AdminExecCreds(ctx context.Context) (containerName, adminUser, password string, err error) {
	instances, err := s.queries.ListPgInstances(ctx)
	if err != nil {
		return "", "", "", err
	}
	if len(instances) == 0 {
		return "", "", "", fmt.Errorf("%w: no postgres instance configured", ErrNotFound)
	}
	inst := instances[0]
	creds, err := s.resolveExecCreds(ctx, inst)
	if err != nil {
		return "", "", "", err
	}
	return creds.container, creds.user, creds.password, nil
}

func (s *Service) adminPassword(inst db.PdbInstance) (string, error) {
	return s.cipher.Decrypt(inst.EncryptedAdminPassword)
}

type execCreds struct {
	container string
	user      string
	password  string
}

const execCredsTTL = 5 * time.Minute

// resolveExecCreds returns TCP credentials for the managed Postgres container.
// Prefer panel settings (admin_user + password). If the data volume was created
// with a different POSTGRES_USER, discover that role, sync the panel password
// onto it, then always connect via 127.0.0.1 + PGPASSWORD (same as DB create).
func (s *Service) resolveExecCreds(ctx context.Context, inst db.PdbInstance) (execCreds, error) {
	if creds, ok := s.getCachedExecCreds(inst.ID); ok {
		return creds, nil
	}

	cname := s.containerName(inst)
	panelPass, err := s.adminPassword(inst)
	if err != nil {
		return execCreds{}, err
	}

	envUser := s.containerEnv(ctx, cname, "POSTGRES_USER")
	envPass := s.containerEnv(ctx, cname, "POSTGRES_PASSWORD")
	panelUser := strings.TrimSpace(inst.AdminUser)

	// Same order the container is started with: POSTGRES_USER, then panel admin.
	candidates := uniqueNonEmpty(envUser, panelUser, "barn", "dockpilot", "dock_pilot", "postgres")

	tryTCP := func(user, pass string) (execCreds, bool) {
		if user == "" || pass == "" {
			return execCreds{}, false
		}
		if !s.probeAdmin(ctx, cname, user, pass, false) {
			return execCreds{}, false
		}
		creds := execCreds{container: cname, user: user, password: pass}
		s.putCachedExecCreds(inst.ID, creds)
		if user != panelUser {
			s.logger.WarnContext(ctx, "postgres admin role differs from panel setting",
				"configured", panelUser, "actual", user)
		}
		return creds, true
	}

	// 1) Panel password (settings used to create DBs / sync on deploy).
	for _, user := range candidates {
		if creds, ok := tryTCP(user, panelPass); ok {
			return creds, nil
		}
	}

	// 2) Password out of sync with volume — set panel password via local trust, then TCP.
	for _, user := range candidates {
		if !s.probeAdmin(ctx, cname, user, "", true) {
			continue
		}
		if err := s.setRolePasswordLocal(ctx, cname, user, panelPass); err != nil {
			s.logger.WarnContext(ctx, "sync postgres role password via socket failed",
				"user", user, "error", err)
			continue
		}
		if creds, ok := tryTCP(user, panelPass); ok {
			return creds, nil
		}
	}

	// 3) Last resort: container env password (initial image boot).
	if envPass != "" && envPass != panelPass {
		for _, user := range candidates {
			if creds, ok := tryTCP(user, envPass); ok {
				return creds, nil
			}
		}
	}

	return execCreds{}, fmt.Errorf(
		"no working postgres admin role in %s (configured admin_user=%q env_user=%q)",
		cname, panelUser, envUser,
	)
}

func (s *Service) getCachedExecCreds(id uuid.UUID) (execCreds, bool) {
	s.credMu.Lock()
	defer s.credMu.Unlock()
	item, ok := s.credCache[id]
	if !ok || time.Now().After(item.until) {
		return execCreds{}, false
	}
	return item.creds, true
}

func (s *Service) putCachedExecCreds(id uuid.UUID, creds execCreds) {
	s.credMu.Lock()
	defer s.credMu.Unlock()
	if s.credCache == nil {
		s.credCache = map[uuid.UUID]cachedExecCreds{}
	}
	s.credCache[id] = cachedExecCreds{creds: creds, until: time.Now().Add(execCredsTTL)}
}

func (s *Service) clearCachedExecCreds(id uuid.UUID) {
	s.credMu.Lock()
	defer s.credMu.Unlock()
	delete(s.credCache, id)
}

func uniqueNonEmpty(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// setRolePasswordLocal uses the image OS user over the local socket (trust/peer)
// to align the DB role password with panel settings.
func (s *Service) setRolePasswordLocal(ctx context.Context, cname, user, password string) error {
	userIdent, err := quoteIdent(user)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s", userIdent, quoteLiteral(password))
	var stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: cname,
		User:          "postgres",
		Cmd: []string{
			"psql",
			"-v", "ON_ERROR_STOP=1",
			"-U", user,
			"-d", "postgres",
			"-c", sql,
		},
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

func (s *Service) containerEnv(ctx context.Context, cname, key string) string {
	var stdout bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: cname,
		Cmd:           []string{"printenv", key},
	}, nil, &stdout, io.Discard)
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

func (s *Service) probeAdmin(ctx context.Context, cname, user, password string, localSocket bool) bool {
	opts := docker.ExecOptions{
		ContainerName: cname,
		Cmd: []string{
			"psql",
			"-v", "ON_ERROR_STOP=1",
			"-U", user,
			"-d", "postgres",
			"-c", "SELECT 1",
		},
	}
	if localSocket {
		// Image OS user — local trust/peer, used only to bootstrap password sync.
		opts.User = "postgres"
	} else {
		opts.Cmd = append([]string{"psql", "-h", "127.0.0.1"}, opts.Cmd[1:]...)
		if password != "" {
			opts.Env = []string{"PGPASSWORD=" + password}
		}
	}
	code, err := s.docker.Exec(ctx, opts, nil, io.Discard, io.Discard)
	return err == nil && code == 0
}

func (s *Service) execOpts(creds execCreds, cmd []string) docker.ExecOptions {
	opts := docker.ExecOptions{
		ContainerName: creds.container,
		Cmd:           cmd,
	}
	// Always TCP so PGPASSWORD is honored (socket peer ignores it and breaks on wrong -U).
	if len(cmd) > 0 {
		switch cmd[0] {
		case "psql", "pg_dump", "pg_isready", "pg_restore":
			opts.Cmd = append([]string{cmd[0], "-h", "127.0.0.1"}, cmd[1:]...)
		}
	}
	if creds.password != "" {
		opts.Env = []string{"PGPASSWORD=" + creds.password}
	}
	return opts
}

func (s *Service) execSQL(ctx context.Context, inst db.PdbInstance, database, sql string) error {
	creds, err := s.resolveExecCreds(ctx, inst)
	if err != nil {
		return fmt.Errorf("resolve admin: %w", err)
	}
	var stderr bytes.Buffer
	opts := s.execOpts(creds, []string{
		"psql",
		"-v", "ON_ERROR_STOP=1",
		"-U", creds.user,
		"-d", database,
		"-c", sql,
	})
	code, err := s.docker.Exec(ctx, opts, nil, io.Discard, &stderr)
	if err != nil {
		s.clearCachedExecCreds(inst.ID)
		return err
	}
	if code != 0 {
		s.clearCachedExecCreds(inst.ID)
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
	user := strings.TrimSpace(inst.AdminUser)
	if user == "" {
		user = "postgres"
	}
	if envUser := s.containerEnv(ctx, s.containerName(inst), "POSTGRES_USER"); envUser != "" {
		user = envUser
	}
	creds := execCreds{container: s.containerName(inst), user: user, password: password}
	var stderr bytes.Buffer
	opts := s.execOpts(creds, []string{"pg_isready", "-U", creds.user})
	code, err := s.docker.Exec(ctx, opts, nil, io.Discard, &stderr)
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

// syncAdminPassword sets the DB role password to match the panel (local socket / trust).
// Needed when the data volume already existed — POSTGRES_PASSWORD is ignored on reuse.
func (s *Service) syncAdminPassword(ctx context.Context, inst db.PdbInstance, password string) error {
	s.clearCachedExecCreds(inst.ID)
	cname := s.containerName(inst)
	envUser := s.containerEnv(ctx, cname, "POSTGRES_USER")
	candidates := uniqueNonEmpty(envUser, inst.AdminUser, "barn", "dockpilot", "dock_pilot", "postgres")

	var lastErr error
	for _, user := range candidates {
		if !s.probeAdmin(ctx, cname, user, "", true) {
			continue
		}
		if err := s.setRolePasswordLocal(ctx, cname, user, password); err != nil {
			lastErr = err
			continue
		}
		if s.probeAdmin(ctx, cname, user, password, false) {
			return nil
		}
		lastErr = fmt.Errorf("password set for %s but TCP login failed", user)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("could not sync admin password for any role in %s", cname)
}

func (s *Service) dumpDatabase(ctx context.Context, inst db.PdbInstance, dbName string, w io.Writer) error {
	creds, err := s.resolveExecCreds(ctx, inst)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	opts := s.execOpts(creds, []string{
		"pg_dump",
		"-U", creds.user,
		"-d", dbName,
		"--no-owner",
		"--no-acl",
	})
	code, err := s.docker.Exec(ctx, opts, nil, w, &stderr)
	if err != nil {
		s.clearCachedExecCreds(inst.ID)
		return err
	}
	if code != 0 {
		s.clearCachedExecCreds(inst.ID)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("pg_dump exit %d", code)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (s *Service) restoreDatabase(ctx context.Context, inst db.PdbInstance, dbName string, r io.Reader) error {
	creds, err := s.resolveExecCreds(ctx, inst)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	opts := s.execOpts(creds, []string{
		"psql",
		"-v", "ON_ERROR_STOP=1",
		"-U", creds.user,
		"-d", dbName,
	})
	code, err := s.docker.Exec(ctx, opts, filterRestoreSQL(r), io.Discard, &stderr)
	if err != nil {
		s.clearCachedExecCreds(inst.ID)
		return err
	}
	if code != 0 {
		s.clearCachedExecCreds(inst.ID)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("psql restore exit %d", code)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// Newer pg_dump (PG17+) emits SET lines that older servers reject under ON_ERROR_STOP.
var restoreSkipLine = regexp.MustCompile(`(?i)^\s*(SET\s+transaction_timeout\b|SELECT\s+pg_catalog\.set_config\(\s*'transaction_timeout')`)

// filterRestoreSQL drops session settings unknown to older Postgres (e.g. 16).
func filterRestoreSQL(r io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if restoreSkipLine.MatchString(line) {
				continue
			}
			if _, err := io.WriteString(pw, line+"\n"); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		if err := sc.Err(); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	return pr
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
