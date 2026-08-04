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

// managedPostgresCandidates is the lookup order for the managed Postgres container.
// Panel compose uses dock-pilot-postgres — that must be last so we never exec
// managed dumps/queries into the panel DB when barn-postgres also exists.
var managedPostgresCandidates = []string{
	"barn-postgres",
	"dockpilot-postgres",
	"dock-pilot-postgres",
}

func volumeForManagedContainer(container string) string {
	switch container {
	case "dock-pilot-postgres":
		return "dock-pilot_dock_pilot_pg"
	case "dockpilot-postgres":
		return "dockpilot-postgres-data"
	default:
		return "barn-postgres-data"
	}
}

// resolvePGNames picks the managed Postgres container/volume.
// Prefer barn-postgres (canonical). Fall back to legacy managed names, and only
// then to dock-pilot-postgres (old shared panel+managed installs).
func (s *Service) resolvePGNames(ctx context.Context) (container, volume string) {
	st, err := s.docker.InspectContainer(ctx, managedPostgresCandidates...)
	if err == nil && st.Found && st.Container != "" {
		return st.Container, volumeForManagedContainer(st.Container)
	}
	return "barn-postgres", "barn-postgres-data"
}

// presentManagedContainers returns which candidate containers Docker currently knows.
func (s *Service) presentManagedContainers(ctx context.Context) []string {
	var out []string
	for _, name := range managedPostgresCandidates {
		st, err := s.docker.InspectContainer(ctx, name)
		if err != nil || !st.Found {
			continue
		}
		state := st.State
		if state == "" {
			if st.Running {
				state = "running"
			} else {
				state = "found"
			}
		}
		out = append(out, name+"="+state)
	}
	return out
}

// listClusterDatabases lists non-template DB names inside the resolved cluster.
// Used only for diagnostics when an operation fails (never logs passwords).
func (s *Service) listClusterDatabases(ctx context.Context, creds execCreds) []string {
	var stdout, stderr bytes.Buffer
	opts := s.execOpts(creds, []string{
		"psql",
		"-v", "ON_ERROR_STOP=1",
		"-U", creds.user,
		"-d", "postgres",
		"-tAc", "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY 1",
	})
	code, err := s.docker.Exec(ctx, opts, nil, &stdout, &stderr)
	if err != nil || code != 0 {
		s.logger.WarnContext(ctx, "pg list databases for diagnostics failed",
			"container", creds.container,
			"user", creds.user,
			"mode", creds.mode.String(),
			"exit", code,
			"stderr", truncateDiag(stderr.String(), 500),
			"error", errString(err),
		)
		return nil
	}
	var names []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}

func truncateDiag(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Service) logPGOp(ctx context.Context, op string, inst db.PdbInstance, creds execCreds, database string, extra ...any) {
	args := []any{
		"op", op,
		"instance_id", inst.ID.String(),
		"admin_user_setting", inst.AdminUser,
		"container", creds.container,
		"volume", volumeForManagedContainer(creds.container),
		"user", creds.user,
		"mode", creds.mode.String(),
		"database", database,
		"present_containers", s.presentManagedContainers(ctx),
	}
	args = append(args, extra...)
	s.logger.InfoContext(ctx, "managed postgres op", args...)
}

func (s *Service) failPGOp(ctx context.Context, op string, inst db.PdbInstance, creds execCreds, database, msg string) error {
	dbs := s.listClusterDatabases(ctx, creds)
	inCluster := false
	for _, n := range dbs {
		if n == database {
			inCluster = true
			break
		}
	}
	s.logger.ErrorContext(ctx, "managed postgres op failed",
		"op", op,
		"instance_id", inst.ID.String(),
		"admin_user_setting", inst.AdminUser,
		"container", creds.container,
		"volume", volumeForManagedContainer(creds.container),
		"user", creds.user,
		"mode", creds.mode.String(),
		"database", database,
		"database_in_cluster", inCluster,
		"cluster_databases", dbs,
		"present_containers", s.presentManagedContainers(ctx),
		"error", msg,
	)
	return fmt.Errorf("%s (container=%s user=%s mode=%s db=%q in_cluster=%v cluster_dbs=%v present=%v)",
		msg, creds.container, creds.user, creds.mode.String(), database, inCluster, dbs, s.presentManagedContainers(ctx))
}

// AdminExecCreds returns the managed Postgres container name and superuser login.
// tcp is true when callers must use -h 127.0.0.1 with PGPASSWORD; false means local socket.
func (s *Service) AdminExecCreds(ctx context.Context) (containerName, adminUser, password string, tcp bool, err error) {
	instances, err := s.queries.ListPgInstances(ctx)
	if err != nil {
		return "", "", "", false, err
	}
	if len(instances) == 0 {
		return "", "", "", false, fmt.Errorf("%w: no postgres instance configured", ErrNotFound)
	}
	inst := instances[0]
	creds, err := s.resolveExecCreds(ctx, inst)
	if err != nil {
		return "", "", "", false, err
	}
	return creds.container, creds.user, creds.password, creds.mode == connTCP, nil
}

func (s *Service) adminPassword(inst db.PdbInstance) (string, error) {
	return s.cipher.Decrypt(inst.EncryptedAdminPassword)
}

type connMode int

const (
	connSocket connMode = iota
	connTCP
)

type execCreds struct {
	container string
	user      string
	password  string
	mode      connMode
}

const execCredsTTL = 5 * time.Minute

// resolveExecCreds returns credentials for psql/pg_dump inside the managed container.
//
// Order:
//  1. Panel password over TCP (scram on 127.0.0.1) — best for all callers.
//  2. Panel password over local socket (historical managed DB path; local trust).
//     If socket works but TCP does not, sync the panel password onto the role and
//     prefer TCP afterward so panel snapshots can use PGPASSWORD.
//  3. Discover role via local socket without password, sync panel password, retry.
//  4. Container POSTGRES_PASSWORD as last resort.
//
// Candidate roles prefer POSTGRES_USER from the container (volume init user), then
// panel admin_user — this is the "role postgres does not exist" case.
func (s *Service) resolveExecCreds(ctx context.Context, inst db.PdbInstance) (execCreds, error) {
	if creds, ok := s.getCachedExecCreds(inst.ID); ok {
		s.logger.DebugContext(ctx, "managed postgres creds cache hit",
			"instance_id", inst.ID.String(),
			"container", creds.container,
			"user", creds.user,
			"mode", creds.mode.String(),
		)
		return creds, nil
	}

	cname := s.containerName(inst)
	present := s.presentManagedContainers(ctx)
	panelPass, err := s.adminPassword(inst)
	if err != nil {
		return execCreds{}, err
	}

	envUser := s.containerEnv(ctx, cname, "POSTGRES_USER")
	envPass := s.containerEnv(ctx, cname, "POSTGRES_PASSWORD")
	panelUser := strings.TrimSpace(inst.AdminUser)

	s.logger.InfoContext(ctx, "managed postgres resolve creds start",
		"instance_id", inst.ID.String(),
		"container", cname,
		"volume", volumeForManagedContainer(cname),
		"admin_user_setting", panelUser,
		"env_postgres_user", envUser,
		"env_password_set", envPass != "",
		"panel_password_set", panelPass != "",
		"present_containers", present,
	)

	// Prefer the role the volume was created with, then panel setting.
	candidates := uniqueNonEmpty(envUser, panelUser, "barn", "dockpilot", "dock_pilot", "postgres")

	accept := func(user, pass string, mode connMode) (execCreds, bool) {
		if user == "" {
			return execCreds{}, false
		}
		if !s.probeAdmin(ctx, cname, user, pass, mode) {
			return execCreds{}, false
		}
		creds := execCreds{container: cname, user: user, password: pass, mode: mode}
		s.putCachedExecCreds(inst.ID, creds)
		s.logger.InfoContext(ctx, "managed postgres creds resolved",
			"instance_id", inst.ID.String(),
			"container", cname,
			"user", user,
			"mode", mode.String(),
			"admin_user_setting", panelUser,
			"role_matches_setting", user == panelUser,
		)
		if user != panelUser && panelUser != "" {
			s.logger.WarnContext(ctx, "postgres admin role differs from panel setting",
				"configured", panelUser, "actual", user, "mode", mode.String())
		}
		return creds, true
	}

	// 1) Panel password over TCP — preferred when scram-on-host works.
	if panelPass != "" {
		for _, user := range candidates {
			if creds, ok := accept(user, panelPass, connTCP); ok {
				return creds, nil
			}
		}
	}

	// 2) Panel password over local socket — historical path for managed dumps/queries
	//    (official image uses local trust; PGPASSWORD may be ignored).
	for _, user := range candidates {
		if _, ok := accept(user, panelPass, connSocket); !ok {
			continue
		}
		// Align DB password with panel so TCP callers (panel snapshot dump) work too.
		if panelPass != "" && !s.probeAdmin(ctx, cname, user, panelPass, connTCP) {
			if err := s.setRolePasswordLocal(ctx, cname, user, panelPass); err != nil {
				s.logger.WarnContext(ctx, "sync postgres role password via socket failed",
					"user", user, "error", err)
			} else if creds, ok := accept(user, panelPass, connTCP); ok {
				return creds, nil
			}
		}
		if creds, ok := accept(user, panelPass, connSocket); ok {
			return creds, nil
		}
	}

	// 3) Password out of sync and socket trust allows discovery without password.
	for _, user := range candidates {
		if !s.probeAdmin(ctx, cname, user, "", connSocket) {
			continue
		}
		if err := s.setRolePasswordLocal(ctx, cname, user, panelPass); err != nil {
			s.logger.WarnContext(ctx, "sync postgres role password via socket failed",
				"user", user, "error", err)
			continue
		}
		s.clearCachedExecCreds(inst.ID)
		if panelPass != "" {
			if creds, ok := accept(user, panelPass, connTCP); ok {
				return creds, nil
			}
		}
		if creds, ok := accept(user, panelPass, connSocket); ok {
			return creds, nil
		}
	}

	// 4) Env password (initial boot password still valid).
	if envPass != "" && envPass != panelPass {
		for _, user := range candidates {
			if creds, ok := accept(user, envPass, connTCP); ok {
				return creds, nil
			}
			if creds, ok := accept(user, envPass, connSocket); ok {
				return creds, nil
			}
		}
	}

	return execCreds{}, fmt.Errorf(
		"no working postgres admin role in %s (configured admin_user=%q env_user=%q present=%v)",
		cname, panelUser, envUser, present,
	)
}

func (m connMode) String() string {
	if m == connTCP {
		return "tcp"
	}
	return "socket"
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

func (s *Service) probeAdmin(ctx context.Context, cname, user, password string, mode connMode) bool {
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
	switch mode {
	case connTCP:
		opts.Cmd = append([]string{"psql", "-h", "127.0.0.1"}, opts.Cmd[1:]...)
		if password != "" {
			opts.Env = []string{"PGPASSWORD=" + password}
		}
	case connSocket:
		if password != "" {
			// Historical path: default exec user + PGPASSWORD (local trust ignores it).
			opts.Env = []string{"PGPASSWORD=" + password}
		} else {
			// Role discovery / password sync bootstrap as image OS user.
			opts.User = "postgres"
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
	if creds.mode == connTCP && len(cmd) > 0 {
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
	s.logPGOp(ctx, "psql", inst, creds, database, "sql_preview", truncateDiag(sql, 120))
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
		return s.failPGOp(ctx, "psql", inst, creds, database, err.Error())
	}
	if code != 0 {
		s.clearCachedExecCreds(inst.ID)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("psql exit %d", code)
		}
		return s.failPGOp(ctx, "psql", inst, creds, database, msg)
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
	// pg_isready only checks the server; prefer socket like the rest of bootstrap.
	creds := execCreds{container: s.containerName(inst), user: user, password: password, mode: connSocket}
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
		if !s.probeAdmin(ctx, cname, user, "", connSocket) {
			continue
		}
		if err := s.setRolePasswordLocal(ctx, cname, user, password); err != nil {
			lastErr = err
			continue
		}
		if s.probeAdmin(ctx, cname, user, password, connSocket) || s.probeAdmin(ctx, cname, user, password, connTCP) {
			return nil
		}
		lastErr = fmt.Errorf("password set for %s but login still failed", user)
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
	s.logPGOp(ctx, "pg_dump", inst, creds, dbName)
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
		return s.failPGOp(ctx, "pg_dump", inst, creds, dbName, err.Error())
	}
	if code != 0 {
		s.clearCachedExecCreds(inst.ID)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("pg_dump exit %d", code)
		}
		return s.failPGOp(ctx, "pg_dump", inst, creds, dbName, msg)
	}
	s.logger.InfoContext(ctx, "managed postgres op ok",
		"op", "pg_dump",
		"container", creds.container,
		"user", creds.user,
		"mode", creds.mode.String(),
		"database", dbName,
	)
	return nil
}

func (s *Service) restoreDatabase(ctx context.Context, inst db.PdbInstance, dbName string, r io.Reader) error {
	creds, err := s.resolveExecCreds(ctx, inst)
	if err != nil {
		return err
	}
	s.logPGOp(ctx, "pg_restore_psql", inst, creds, dbName)
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
		return s.failPGOp(ctx, "pg_restore_psql", inst, creds, dbName, err.Error())
	}
	if code != 0 {
		s.clearCachedExecCreds(inst.ID)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("psql restore exit %d", code)
		}
		return s.failPGOp(ctx, "pg_restore_psql", inst, creds, dbName, msg)
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
