package pgdb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
	"github.com/ebash/barn/backend/internal/secrets"
)

// fakePGDocker simulates docker exec against a managed Postgres container.
// It models the real failure modes we hit in production:
//   - volume initialized with a non-postgres role (POSTGRES_USER)
//   - local socket trust vs TCP password auth
//   - TCP disabled / unreachable while socket still works
type fakePGDocker struct {
	*docker.StubClient
	mu sync.Mutex

	container string
	env       map[string]string
	// passwords for roles that exist. Missing key => role does not exist.
	passwords map[string]string
	// localTrust: socket auth succeeds for any existing role (official image default).
	localTrust bool
	// allowTCP: whether -h 127.0.0.1 connections work.
	allowTCP bool

	execs []docker.ExecOptions
}

func newFakePG(container string) *fakePGDocker {
	return &fakePGDocker{
		StubClient: docker.NewStubClient(nil),
		container:  container,
		env:        map[string]string{},
		passwords:  map[string]string{},
		localTrust: true,
		allowTCP:   true,
	}
}

func (f *fakePGDocker) InspectContainer(ctx context.Context, names ...string) (docker.ContainerStatus, error) {
	for _, n := range names {
		if n == f.container {
			return docker.ContainerStatus{
				Found:     true,
				Running:   true,
				State:     "running",
				Health:    "healthy",
				Container: f.container,
			}, nil
		}
	}
	return docker.ContainerStatus{Health: "none"}, nil
}

func (f *fakePGDocker) Exec(ctx context.Context, opts docker.ExecOptions, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	f.mu.Lock()
	f.execs = append(f.execs, opts)
	f.mu.Unlock()

	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cmd := opts.Cmd
	if len(cmd) == 0 {
		return 1, fmt.Errorf("empty cmd")
	}

	switch cmd[0] {
	case "printenv":
		if len(cmd) < 2 {
			return 1, nil
		}
		f.mu.Lock()
		v := f.env[cmd[1]]
		f.mu.Unlock()
		if v == "" {
			return 1, nil
		}
		_, _ = io.WriteString(stdout, v+"\n")
		return 0, nil
	}

	bin := cmd[0]
	if bin != "psql" && bin != "pg_dump" && bin != "pg_isready" {
		return 1, fmt.Errorf("unsupported: %s", bin)
	}

	tcp := false
	user := ""
	database := "postgres"
	sql := ""
	csv := false
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case "-h":
			tcp = true
			i++
		case "-U":
			if i+1 < len(cmd) {
				user = cmd[i+1]
				i++
			}
		case "-d":
			if i+1 < len(cmd) {
				database = cmd[i+1]
				i++
			}
		case "-c":
			if i+1 < len(cmd) {
				sql = cmd[i+1]
				i++
			}
		case "--csv":
			csv = true
		}
	}
	pass := ""
	for _, e := range opts.Env {
		if strings.HasPrefix(e, "PGPASSWORD=") {
			pass = strings.TrimPrefix(e, "PGPASSWORD=")
		}
	}

	f.mu.Lock()
	rolePass, roleExists := f.passwords[user]
	localTrust := f.localTrust
	allowTCP := f.allowTCP
	f.mu.Unlock()

	if user == "" {
		_, _ = io.WriteString(stderr, "no user\n")
		return 1, nil
	}
	if !roleExists {
		_, _ = fmt.Fprintf(stderr, `psql: error: connection to server on socket "/var/run/postgresql/.s.PGSQL.5432" failed: FATAL: role %q does not exist`+"\n", user)
		return 1, nil
	}

	if bin == "pg_isready" {
		return 0, nil
	}

	authed := false
	if tcp {
		if !allowTCP {
			_, _ = io.WriteString(stderr, "could not connect to server: Connection refused\n")
			return 1, nil
		}
		authed = pass != "" && pass == rolePass
		if !authed {
			_, _ = fmt.Fprintf(stderr, "FATAL: password authentication failed for user %q\n", user)
			return 1, nil
		}
	} else {
		// Socket: trust accepts any existing role; otherwise password must match.
		if localTrust {
			authed = true
		} else {
			authed = pass != "" && pass == rolePass
		}
		if !authed {
			_, _ = fmt.Fprintf(stderr, "FATAL: password authentication failed for user %q\n", user)
			return 1, nil
		}
	}

	// Handle ALTER ROLE … PASSWORD (password sync via socket).
	if strings.Contains(strings.ToUpper(sql), "ALTER ROLE") && strings.Contains(strings.ToUpper(sql), "PASSWORD") {
		newPass := extractQuotedLiteral(sql)
		if newPass == "" {
			return 1, nil
		}
		f.mu.Lock()
		f.passwords[user] = newPass
		f.mu.Unlock()
		return 0, nil
	}

	if bin == "pg_dump" {
		_, _ = fmt.Fprintf(stdout, "-- dump of %s as %s\nCREATE TABLE ok(id int);\n", database, user)
		return 0, nil
	}

	if strings.Contains(sql, "SELECT 1") && !strings.Contains(sql, "relname") {
		if csv {
			_, _ = io.WriteString(stdout, "?column?\n1\n")
		} else {
			_, _ = io.WriteString(stdout, "1\n")
		}
		return 0, nil
	}

	if strings.Contains(sql, "relname") {
		_, _ = io.WriteString(stdout, "name,approx_rows\nusers,3\norders,10\n")
		return 0, nil
	}

	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		if csv {
			_, _ = io.WriteString(stdout, "id\n1\n")
		}
		return 0, nil
	}

	return 0, nil
}

func extractQuotedLiteral(sql string) string {
	// ALTER ROLE "x" WITH PASSWORD 'secret'
	i := strings.Index(sql, "'")
	if i < 0 {
		return ""
	}
	rest := sql[i+1:]
	j := strings.Index(rest, "'")
	if j < 0 {
		return ""
	}
	return strings.ReplaceAll(rest[:j], "''", "'")
}

func testCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	c, err := secrets.NewCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testInstance(t *testing.T, cipher *secrets.Cipher, adminUser, password string) db.PdbInstance {
	t.Helper()
	enc, err := cipher.Encrypt(password)
	if err != nil {
		t.Fatal(err)
	}
	return db.PdbInstance{
		ID:                     uuid.New(),
		Name:                   "Postgres",
		Slug:                   "postgres",
		AdminUser:              adminUser,
		EncryptedAdminPassword: enc,
		Status:                 "active",
	}
}

func testService(dockerClient docker.Client, cipher *secrets.Cipher) *Service {
	return NewService(nil, dockerClient, cipher, nil)
}

func TestResolveExecCreds_panelSettingsOverSocket(t *testing.T) {
	// Regression: original path used socket + panel admin_user/password (trust).
	// Forcing TCP-only broke installs where TCP auth fails but socket works.
	pg := newFakePG("barn-postgres")
	pg.env["POSTGRES_USER"] = "barn"
	pg.env["POSTGRES_PASSWORD"] = "panel-secret"
	pg.passwords["barn"] = "panel-secret"
	pg.allowTCP = false // TCP broken / not listening
	pg.localTrust = true

	cipher := testCipher(t)
	svc := testService(pg, cipher)
	inst := testInstance(t, cipher, "barn", "panel-secret")

	creds, err := svc.resolveExecCreds(context.Background(), inst)
	if err != nil {
		t.Fatalf("resolveExecCreds: %v", err)
	}
	if creds.user != "barn" {
		t.Fatalf("user=%q want barn", creds.user)
	}
	if creds.mode != connSocket {
		t.Fatalf("mode=%v want socket (TCP disabled)", creds.mode)
	}

	var dump bytes.Buffer
	if err := svc.dumpDatabase(context.Background(), inst, "appdb", &dump); err != nil {
		t.Fatalf("dumpDatabase: %v", err)
	}
	if !strings.Contains(dump.String(), "dump of appdb") {
		t.Fatalf("unexpected dump: %q", dump.String())
	}

	// Ensure dump did not force TCP.
	pg.mu.Lock()
	defer pg.mu.Unlock()
	for _, e := range pg.execs {
		if len(e.Cmd) > 0 && e.Cmd[0] == "pg_dump" {
			for _, a := range e.Cmd {
				if a == "-h" {
					t.Fatalf("pg_dump used TCP despite socket creds: %v", e.Cmd)
				}
			}
			if e.Cmd[0] == "pg_dump" && !containsArg(e.Cmd, "-U", "barn") {
				t.Fatalf("pg_dump user wrong: %v", e.Cmd)
			}
		}
	}
}

func TestResolveExecCreds_legacyVolumeMissingPostgresRole(t *testing.T) {
	// Panel still has admin_user=postgres, but volume was init with POSTGRES_USER=barn.
	pg := newFakePG("barn-postgres")
	pg.env["POSTGRES_USER"] = "barn"
	pg.env["POSTGRES_PASSWORD"] = "old-env-pass"
	pg.passwords["barn"] = "panel-secret" // already synced earlier
	// no "postgres" role
	pg.allowTCP = true
	pg.localTrust = true

	cipher := testCipher(t)
	svc := testService(pg, cipher)
	inst := testInstance(t, cipher, "postgres", "panel-secret")

	creds, err := svc.resolveExecCreds(context.Background(), inst)
	if err != nil {
		t.Fatalf("resolveExecCreds: %v", err)
	}
	if creds.user != "barn" {
		t.Fatalf("user=%q want barn (not missing postgres role)", creds.user)
	}

	var dump bytes.Buffer
	if err := svc.dumpDatabase(context.Background(), inst, "coachman", &dump); err != nil {
		t.Fatalf("dumpDatabase: %v", err)
	}
}

func TestResolveExecCreds_syncPasswordWhenOutOfSync(t *testing.T) {
	pg := newFakePG("barn-postgres")
	pg.env["POSTGRES_USER"] = "barn"
	pg.passwords["barn"] = "volume-old-password"
	pg.allowTCP = true
	pg.localTrust = true

	cipher := testCipher(t)
	svc := testService(pg, cipher)
	inst := testInstance(t, cipher, "barn", "panel-secret")

	creds, err := svc.resolveExecCreds(context.Background(), inst)
	if err != nil {
		t.Fatalf("resolveExecCreds: %v", err)
	}
	if creds.user != "barn" || creds.password != "panel-secret" {
		t.Fatalf("creds=%+v", creds)
	}
	if creds.mode != connTCP {
		t.Fatalf("mode=%v want tcp after password sync", creds.mode)
	}
	pg.mu.Lock()
	if pg.passwords["barn"] != "panel-secret" {
		t.Fatalf("password not synced: %q", pg.passwords["barn"])
	}
	pg.mu.Unlock()
}

func TestQuerySQL_listsTablesWithResolvedCreds(t *testing.T) {
	pg := newFakePG("dock-pilot-postgres")
	pg.env["POSTGRES_USER"] = "dockpilot"
	pg.passwords["dockpilot"] = "secret"
	pg.allowTCP = false
	pg.localTrust = true

	cipher := testCipher(t)
	svc := testService(pg, cipher)
	inst := testInstance(t, cipher, "dockpilot", "secret")

	res, err := svc.querySQL(context.Background(), inst, "mydb", `
SELECT c.relname AS name,
       COALESCE(s.n_live_tup, 0)::bigint AS approx_rows
FROM pg_catalog.pg_class c`)
	if err != nil {
		t.Fatalf("querySQL: %v", err)
	}
	if len(res.Columns) != 2 || res.Columns[0] != "name" {
		t.Fatalf("columns=%v", res.Columns)
	}
	if len(res.Rows) != 2 || res.Rows[0][0] != "users" {
		t.Fatalf("rows=%v", res.Rows)
	}
}

func TestDumpDatabase_neverUsesMissingPostgresRole(t *testing.T) {
	pg := newFakePG("barn-postgres")
	pg.env["POSTGRES_USER"] = "barn"
	pg.passwords["barn"] = "x"
	pg.allowTCP = true
	pg.localTrust = false // require password on socket too

	cipher := testCipher(t)
	svc := testService(pg, cipher)
	inst := testInstance(t, cipher, "postgres", "x")

	var dump bytes.Buffer
	if err := svc.dumpDatabase(context.Background(), inst, "db1", &dump); err != nil {
		t.Fatalf("dumpDatabase: %v", err)
	}

	pg.mu.Lock()
	defer pg.mu.Unlock()
	for _, e := range pg.execs {
		if len(e.Cmd) == 0 || e.Cmd[0] != "pg_dump" {
			continue
		}
		if containsArg(e.Cmd, "-U", "postgres") {
			t.Fatalf("pg_dump used missing role postgres: %v", e.Cmd)
		}
		if !containsArg(e.Cmd, "-U", "barn") {
			t.Fatalf("pg_dump expected -U barn: %v", e.Cmd)
		}
	}
}

func TestExecOpts_socketVsTCP(t *testing.T) {
	svc := testService(docker.NewStubClient(nil), testCipher(t))

	sock := svc.execOpts(execCreds{container: "c", user: "barn", password: "p", mode: connSocket}, []string{"pg_dump", "-U", "barn", "-d", "db"})
	for _, a := range sock.Cmd {
		if a == "-h" {
			t.Fatalf("socket mode must not inject -h: %v", sock.Cmd)
		}
	}

	tcp := svc.execOpts(execCreds{container: "c", user: "barn", password: "p", mode: connTCP}, []string{"psql", "-U", "barn", "-d", "db", "-c", "SELECT 1"})
	if !containsArg(tcp.Cmd, "-h", "127.0.0.1") {
		t.Fatalf("tcp mode must inject -h 127.0.0.1: %v", tcp.Cmd)
	}
}

func TestParseCSVResult(t *testing.T) {
	res, err := parseCSVResult([]byte("a,b\n1,2\n3,4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Columns) != 2 || len(res.Rows) != 2 {
		t.Fatalf("%+v", res)
	}
}

func containsArg(cmd []string, flag, value string) bool {
	for i := 0; i+1 < len(cmd); i++ {
		if cmd[i] == flag && cmd[i+1] == value {
			return true
		}
	}
	return false
}
