package panelbackup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/docker"
	"github.com/ebash/barn/backend/internal/pgdb"
	"github.com/ebash/barn/backend/internal/s3util"
	"github.com/ebash/barn/backend/internal/secrets"
)

const bundleVersion = "1"

type Service struct {
	queries              *db.Queries
	docker               docker.Client
	cipher               *secrets.Cipher
	pgdb                 *pgdb.Service
	logger               *slog.Logger
	databaseURL          string
	panelPostgresContainer string
}

func NewService(
	queries *db.Queries,
	dockerClient docker.Client,
	cipher *secrets.Cipher,
	pgdbSvc *pgdb.Service,
	databaseURL string,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	container := strings.TrimSpace(os.Getenv("PANEL_POSTGRES_CONTAINER"))
	return &Service{
		queries:                queries,
		docker:                 dockerClient,
		cipher:                 cipher,
		pgdb:                   pgdbSvc,
		logger:                 logger,
		databaseURL:            databaseURL,
		panelPostgresContainer: container, // empty → resolve at use (barn / dockpilot)
	}
}

func (s *Service) GetSettings(ctx context.Context) (SettingsResponse, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	return toSettingsResponse(row), nil
}

func (s *Service) UpdateSettings(ctx context.Context, req UpdateSettingsRequest) (SettingsResponse, error) {
	if err := validateSettings(req); err != nil {
		return SettingsResponse{}, err
	}
	if _, err := s.getSettingsRow(ctx); err != nil {
		return SettingsResponse{}, err
	}

	prefix := strings.TrimSpace(req.S3Prefix)
	if prefix == "" {
		prefix = "barn/backups"
	}
	retention := req.RetentionCount
	if retention <= 0 {
		retention = 7
	}

	row, err := s.queries.UpdatePanelBackupSettings(ctx, db.UpdatePanelBackupSettingsParams{
		Enabled:          req.Enabled,
		Hour:             int32(req.Hour),
		Minute:           int32(req.Minute),
		Timezone:         defaultStr(req.Timezone, "UTC"),
		S3Endpoint:       strings.TrimSpace(req.S3Endpoint),
		S3Region:         defaultStr(req.S3Region, "ru-central1"),
		S3Bucket:         strings.TrimSpace(req.S3Bucket),
		S3Prefix:         prefix,
		S3ForcePathStyle: req.S3ForcePathStyle,
		RetentionCount:   int32(retention),
	})
	if err != nil {
		return SettingsResponse{}, err
	}

	if req.ClearS3Credentials {
		row, err = s.queries.ClearPanelBackupS3Keys(ctx)
		if err != nil {
			return SettingsResponse{}, err
		}
	} else if strings.TrimSpace(req.S3AccessKey) != "" && strings.TrimSpace(req.S3SecretKey) != "" {
		accessEnc, err := s.cipher.Encrypt(strings.TrimSpace(req.S3AccessKey))
		if err != nil {
			return SettingsResponse{}, err
		}
		secretEnc, err := s.cipher.Encrypt(strings.TrimSpace(req.S3SecretKey))
		if err != nil {
			return SettingsResponse{}, err
		}
		row, err = s.queries.UpdatePanelBackupS3Keys(ctx, db.UpdatePanelBackupS3KeysParams{
			EncryptedS3AccessKey: accessEnc,
			EncryptedS3SecretKey: secretEnc,
		})
		if err != nil {
			return SettingsResponse{}, err
		}
	}

	return toSettingsResponse(row), nil
}

func (s *Service) ListFullBackups(ctx context.Context) ([]FullBackupInfo, error) {
	cfg, prefix, err := s.s3Config(ctx)
	if err != nil {
		return nil, err
	}
	listPrefix := path.Join(strings.Trim(prefix, "/"), "full")
	objs, err := s3util.List(ctx, cfg, listPrefix, 100, ".tar.gz")
	if err != nil {
		return nil, err
	}
	out := make([]FullBackupInfo, 0, len(objs))
	for _, o := range objs {
		out = append(out, FullBackupInfo{
			S3Key:     o.Key,
			SizeBytes: o.Size,
			CreatedAt: o.LastModified,
		})
	}
	return out, nil
}

func (s *Service) CreateFullBackup(ctx context.Context) (FullBackupInfo, error) {
	cfg, prefix, err := s.s3Config(ctx)
	if err != nil {
		return FullBackupInfo{}, err
	}

	info, err := s.buildAndUpload(ctx, cfg, prefix)
	status := "ok"
	if err != nil {
		status = err.Error()
	}
	_, _ = s.queries.UpdatePanelBackupRun(ctx, db.UpdatePanelBackupRunParams{
		LastRunAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		LastStatus: truncate(status, 500),
	})
	if err != nil {
		return FullBackupInfo{}, err
	}

	row, _ := s.getSettingsRow(ctx)
	_ = s.applyRetention(ctx, cfg, prefix, int(row.RetentionCount))
	return info, nil
}

func (s *Service) RestoreFullBackup(ctx context.Context, req RestoreFullRequest) error {
	return s.RestoreFullBackupWithLog(ctx, req, nil)
}

func (s *Service) RestoreFullBackupWithLog(ctx context.Context, req RestoreFullRequest, logFn func(level, message string)) error {
	log := func(level, message string) {
		if logFn != nil {
			logFn(level, message)
		}
	}

	key := strings.TrimSpace(req.S3Key)
	if key == "" {
		return fmt.Errorf("%w: s3_key is required", ErrInvalidInput)
	}
	cfg, _, err := s.s3Config(ctx)
	if err != nil {
		return err
	}
	log("info", "Downloading "+key)
	body, err := s3util.Download(ctx, cfg, key)
	if err != nil {
		return err
	}
	defer body.Close()

	gz, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("%w: invalid gzip bundle: %v", ErrInvalidInput, err)
	}
	defer gz.Close()

	log("info", "Extracting bundle")
	tr := tar.NewReader(gz)
	var panelSQL []byte
	managed := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, 512<<20))
		if err != nil {
			return err
		}
		name := hdr.Name
		switch {
		case name == "panel/barn.sql.gz" || strings.HasSuffix(name, "/panel/barn.sql.gz") ||
			name == "panel/dockpilot.sql.gz" || strings.HasSuffix(name, "/panel/dockpilot.sql.gz"):
			panelSQL, err = gunzipBytes(data)
			if err != nil {
				return err
			}
			log("info", fmt.Sprintf("Found panel dump (%d bytes)", len(panelSQL)))
		case strings.HasPrefix(name, "managed/") && strings.HasSuffix(name, ".sql.gz"):
			base := path.Base(name)
			dbName := strings.TrimSuffix(base, ".sql.gz")
			plain, err := gunzipBytes(data)
			if err != nil {
				return err
			}
			managed[dbName] = plain
			log("info", fmt.Sprintf("Found managed dump %s (%d bytes)", dbName, len(plain)))
		case name == "secrets.env" || name == "manifest.json":
			log("info", "Skipping "+path.Base(name)+" (online restore keeps current .env)")
			continue
		}
	}

	if len(panelSQL) == 0 {
		return fmt.Errorf("%w: panel dump missing from bundle", ErrInvalidInput)
	}
	log("info", "Restoring panel database…")
	if err := s.restorePanelSQL(ctx, bytes.NewReader(panelSQL)); err != nil {
		return fmt.Errorf("restore panel database: %w", err)
	}
	log("info", "Panel database restored")

	if len(managed) > 0 {
		log("info", fmt.Sprintf(
			"Skipping %d managed database dump(s) in bundle — restore those from per-database backups",
			len(managed),
		))
	}
	log("info", "Full restore completed — redeploy sites from git if needed")
	return nil
}

func (s *Service) RunDue(ctx context.Context) error {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		if errors.Is(err, ErrMigration) || errors.Is(err, ErrNotConfigured) {
			return nil
		}
		return err
	}
	if !row.Enabled {
		return nil
	}
	if !scheduleDue(row, time.Now()) {
		return nil
	}
	_, err = s.CreateFullBackup(ctx)
	return err
}

func (s *Service) buildAndUpload(ctx context.Context, cfg s3util.Config, prefix string) (FullBackupInfo, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	sites, _ := s.queries.ListSites(ctx)
	slugs := make([]string, 0, len(sites))
	for _, site := range sites {
		slugs = append(slugs, site.Slug)
	}
	panelDB := "barn"
	if conn, err := s.resolvePanelDumpConn(ctx); err == nil {
		panelDB = conn.dbName
	} else if u, err := url.Parse(s.databaseURL); err == nil && strings.Trim(u.Path, "/") != "" {
		panelDB = strings.Trim(u.Path, "/")
	}

	manifest := Manifest{
		Version:     bundleVersion,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Hostname:    hostname(),
		SiteSlugs:   slugs,
		ManagedDBs:  nil, // managed DBs are backed up separately
		PanelDBName: panelDB,
	}
	manJSON, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeTarFile(tw, "manifest.json", manJSON); err != nil {
		return FullBackupInfo{}, err
	}
	if err := writeTarFile(tw, "secrets.env", []byte(buildSecretsEnv())); err != nil {
		return FullBackupInfo{}, err
	}

	var panelBuf bytes.Buffer
	if err := s.dumpPanel(ctx, &panelBuf); err != nil {
		return FullBackupInfo{}, fmt.Errorf("dump panel: %w", err)
	}
	panelGz, err := gzipBytes(panelBuf.Bytes())
	if err != nil {
		return FullBackupInfo{}, err
	}
	if err := writeTarFile(tw, "panel/barn.sql.gz", panelGz); err != nil {
		return FullBackupInfo{}, err
	}

	if err := tw.Close(); err != nil {
		return FullBackupInfo{}, err
	}
	if err := gw.Close(); err != nil {
		return FullBackupInfo{}, err
	}

	now := time.Now().UTC()
	key := path.Join(strings.Trim(prefix, "/"), "full", fmt.Sprintf("barn-%s.tar.gz", now.Format("20060102-150405")))
	size, err := s3util.Upload(ctx, cfg, key, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return FullBackupInfo{}, err
	}
	return FullBackupInfo{S3Key: key, SizeBytes: size, CreatedAt: now}, nil
}

func (s *Service) dumpPanel(ctx context.Context, w io.Writer) error {
	conn, err := s.resolvePanelDumpConn(ctx)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	var code int
	if conn.useHostNet {
		code, err = s.runPgClientHostNet(ctx, conn, nil, w, &stderr, []string{
			"pg_dump",
			"-h", conn.host,
			"-p", conn.port,
			"-U", conn.user,
			"-d", conn.dbName,
			"--no-owner",
			"--no-acl",
		})
	} else {
		code, err = s.docker.Exec(ctx, docker.ExecOptions{
			ContainerName: conn.container,
			Cmd: []string{
				"pg_dump",
				"-h", "127.0.0.1",
				"-p", "5432",
				"-U", conn.user,
				"-d", conn.dbName,
				"--no-owner",
				"--no-acl",
			},
			Env: []string{"PGPASSWORD=" + conn.password},
		}, nil, w, &stderr)
	}
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
	s.logger.InfoContext(ctx, "panel dump ok",
		"database", conn.dbName, "user", conn.user, "host_net", conn.useHostNet)
	return nil
}

func (s *Service) restorePanelSQL(ctx context.Context, r io.Reader) error {
	conn, err := s.resolvePanelDumpConn(ctx)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	var code int
	if conn.useHostNet {
		code, err = s.runPgClientHostNet(ctx, conn, r, io.Discard, &stderr, []string{
			"psql",
			"-h", conn.host,
			"-p", conn.port,
			"-v", "ON_ERROR_STOP=1",
			"-U", conn.user,
			"-d", conn.dbName,
		})
	} else {
		code, err = s.docker.Exec(ctx, docker.ExecOptions{
			ContainerName: conn.container,
			Cmd: []string{
				"psql",
				"-h", "127.0.0.1",
				"-p", "5432",
				"-v", "ON_ERROR_STOP=1",
				"-U", conn.user,
				"-d", conn.dbName,
			},
			Env: []string{"PGPASSWORD=" + conn.password},
		}, r, io.Discard, &stderr)
	}
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

// runPgClientHostNet runs pg_dump/psql in a one-shot container on the host network,
// so it hits the same host:port the API uses (system Postgres or published docker port).
func (s *Service) runPgClientHostNet(ctx context.Context, conn panelDumpConn, stdin io.Reader, stdout, stderr io.Writer, args []string) (int, error) {
	image := s.pgClientImage(ctx, conn.container)
	cmdArgs := []string{
		"run", "--rm", "-i", "--network", "host",
		"-e", "PGPASSWORD=" + conn.password,
		image,
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	return -1, err
}

func (s *Service) pgClientImage(ctx context.Context, container string) string {
	if container != "" {
		// Prefer the image already running as panel/managed Postgres.
		out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.Config.Image}}", container).Output()
		if err == nil {
			if img := strings.TrimSpace(string(out)); img != "" {
				return img
			}
		}
	}
	for _, img := range []string{"barn-postgres:latest", "dockpilot-postgres:latest", "postgres:16"} {
		if err := exec.CommandContext(ctx, "docker", "image", "inspect", img).Run(); err == nil {
			return img
		}
	}
	return "postgres:16"
}

// resolvePanelPostgresContainer picks the running panel Postgres container.
// Prefer PANEL_POSTGRES_CONTAINER, then dockpilot-postgres (legacy), then barn-postgres.
func (s *Service) resolvePanelPostgresContainer(ctx context.Context) (string, error) {
	if name := strings.TrimSpace(s.panelPostgresContainer); name != "" {
		st, err := s.docker.InspectContainer(ctx, name)
		if err != nil {
			return "", err
		}
		if !st.Found {
			return "", fmt.Errorf("PANEL_POSTGRES_CONTAINER=%s not found", name)
		}
		if !st.Running {
			return "", fmt.Errorf("postgres container %s is not running", name)
		}
		return st.Container, nil
	}

	st, err := s.docker.InspectContainer(ctx, "dockpilot-postgres", "barn-postgres")
	if err != nil {
		return "", err
	}
	if !st.Found {
		return "", fmt.Errorf("postgres container not found (tried dockpilot-postgres, barn-postgres) — start the panel database")
	}
	if !st.Running {
		return "", fmt.Errorf("postgres container %s is not running", st.Container)
	}
	return st.Container, nil
}

func (s *Service) applyRetention(ctx context.Context, cfg s3util.Config, prefix string, keep int) error {
	if keep <= 0 {
		return nil
	}
	listPrefix := path.Join(strings.Trim(prefix, "/"), "full")
	objs, err := s3util.List(ctx, cfg, listPrefix, 500, ".tar.gz")
	if err != nil {
		return err
	}
	if len(objs) <= keep {
		return nil
	}
	for _, old := range objs[keep:] {
		_ = s3util.Delete(ctx, cfg, old.Key)
	}
	return nil
}

func (s *Service) s3Config(ctx context.Context) (s3util.Config, string, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return s3util.Config{}, "", err
	}
	if len(row.EncryptedS3AccessKey) == 0 || len(row.EncryptedS3SecretKey) == 0 {
		return s3util.Config{}, "", ErrNotConfigured
	}
	if strings.TrimSpace(row.S3Bucket) == "" {
		return s3util.Config{}, "", ErrNotConfigured
	}
	access, err := s.cipher.Decrypt(row.EncryptedS3AccessKey)
	if err != nil {
		return s3util.Config{}, "", err
	}
	secret, err := s.cipher.Decrypt(row.EncryptedS3SecretKey)
	if err != nil {
		return s3util.Config{}, "", err
	}
	prefix := row.S3Prefix
	if prefix == "" {
		prefix = "barn/backups"
	}
	return s3util.Config{
		Endpoint:       row.S3Endpoint,
		Region:         row.S3Region,
		Bucket:         row.S3Bucket,
		AccessKey:      access,
		SecretKey:      secret,
		ForcePathStyle: row.S3ForcePathStyle,
	}, prefix, nil
}

func (s *Service) getSettingsRow(ctx context.Context) (db.PanelBackupSetting, error) {
	row, err := s.queries.GetPanelBackupSettings(ctx)
	if err == nil {
		return row, nil
	}
	if mapped := mapDBErr(err); mapped != nil {
		return db.PanelBackupSetting{}, mapped
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.PanelBackupSetting{}, err
	}
	row, err = s.queries.EnsurePanelBackupSettings(ctx)
	if err != nil {
		if mapped := mapDBErr(err); mapped != nil {
			return db.PanelBackupSetting{}, mapped
		}
		return db.PanelBackupSetting{}, err
	}
	return row, nil
}

func mapDBErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return ErrMigration
	}
	return nil
}

func toSettingsResponse(row db.PanelBackupSetting) SettingsResponse {
	return SettingsResponse{
		Enabled:          row.Enabled,
		Hour:             int(row.Hour),
		Minute:           int(row.Minute),
		Timezone:         row.Timezone,
		S3Endpoint:       row.S3Endpoint,
		S3Region:         row.S3Region,
		S3Bucket:         row.S3Bucket,
		S3Prefix:         row.S3Prefix,
		S3ForcePathStyle: row.S3ForcePathStyle,
		S3CredentialsSet: len(row.EncryptedS3AccessKey) > 0 && len(row.EncryptedS3SecretKey) > 0,
		RetentionCount:   int(row.RetentionCount),
		LastRunAt:        optionalTime(row.LastRunAt),
		LastStatus:       row.LastStatus,
		UpdatedAt:        row.UpdatedAt,
	}
}

func optionalTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func validateSettings(req UpdateSettingsRequest) error {
	if req.Hour < 0 || req.Hour > 23 || req.Minute < 0 || req.Minute > 59 {
		return fmt.Errorf("%w: invalid schedule time", ErrInvalidInput)
	}
	tz := strings.TrimSpace(req.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalidInput)
	}
	if req.Enabled && strings.TrimSpace(req.S3Bucket) == "" {
		return fmt.Errorf("%w: s3_bucket required when enabled", ErrInvalidInput)
	}
	return nil
}

func scheduleDue(row db.PanelBackupSetting, now time.Time) bool {
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	if local.Hour() != int(row.Hour) || local.Minute() != int(row.Minute) {
		return false
	}
	if !row.LastRunAt.Valid {
		return true
	}
	last := row.LastRunAt.Time.In(loc)
	return last.Year() != local.Year() || last.YearDay() != local.YearDay()
}

func parseDatabaseURL(raw string) (user, password, dbName string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: invalid DATABASE_URL", ErrInvalidInput)
	}
	user = u.User.Username()
	password, _ = u.User.Password()
	dbName = strings.Trim(u.Path, "/")
	if user == "" || dbName == "" {
		return "", "", "", fmt.Errorf("%w: DATABASE_URL missing user or database", ErrInvalidInput)
	}
	return user, password, dbName, nil
}

func databaseURLEndpoint(raw string) (host, port string) {
	u, err := url.Parse(raw)
	if err != nil {
		return "127.0.0.1", "5432"
	}
	host = u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port = u.Port()
	if port == "" {
		port = "5432"
	}
	return host, port
}

// rewriteDatabaseURLPath swaps the database name in a postgres URL.
func rewriteDatabaseURLPath(raw, dbName string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.Path = "/" + strings.Trim(dbName, "/")
	return u.String(), nil
}

type panelDumpConn struct {
	container  string
	user       string
	password   string
	dbName     string
	host       string
	port       string
	useHostNet bool // dump/restore via docker run --network host (same endpoint as API)
}

// resolvePanelDumpConn finds working credentials + database for panel pg_dump.
// Prefers the live DATABASE_URL (what the API already uses), then discovers a
// non-managed DB with panel schema. Managed application databases are never
// selected during discovery — they are backed up separately.
func (s *Service) resolvePanelDumpConn(ctx context.Context) (panelDumpConn, error) {
	managed := s.managedDBSet(ctx)
	host, port := databaseURLEndpoint(s.databaseURL)

	// 1) Live API connection — authoritative.
	if live, liveErr := s.probeLiveDatabaseURL(ctx); liveErr == nil {
		s.logger.InfoContext(ctx, "panel dump connection resolved",
			"source", "DATABASE_URL_live", "user", live.user, "database", live.dbName,
			"host", host, "port", port)
		live.host = host
		live.port = port
		live.useHostNet = true
		if c, cerr := s.resolvePanelPostgresContainer(ctx); cerr == nil {
			live.container = c
		}
		return live, nil
	} else {
		s.logger.WarnContext(ctx, "DATABASE_URL live probe failed", "error", liveErr)
	}

	// 2) Discover via pgx on the same host:port (works even if DB name in URL is stale).
	if found, discErr := s.discoverPanelDBViaPgx(ctx, managed); discErr == nil {
		s.logger.InfoContext(ctx, "panel dump connection resolved",
			"source", "pgx_discovery", "user", found.user, "database", found.dbName,
			"host", host, "port", port)
		found.host = host
		found.port = port
		found.useHostNet = true
		if c, cerr := s.resolvePanelPostgresContainer(ctx); cerr == nil {
			found.container = c
		}
		return found, nil
	} else {
		s.logger.WarnContext(ctx, "pgx panel DB discovery failed", "error", discErr)
	}

	// 3) Fall back: docker exec into panel/managed postgres container.
	container, err := s.resolvePanelPostgresContainer(ctx)
	if err != nil {
		return panelDumpConn{}, err
	}

	type cred struct {
		user, pass, preferredDB, source string
	}
	var tries []cred

	if u, p, d, urlErr := parseDatabaseURL(s.databaseURL); urlErr == nil {
		tries = append(tries, cred{u, p, d, "DATABASE_URL"})
	}
	if u := strings.TrimSpace(os.Getenv("POSTGRES_USER")); u != "" {
		if p := os.Getenv("POSTGRES_PASSWORD"); p != "" {
			d := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
			if d == "" {
				d = "barn"
			}
			tries = append(tries, cred{u, p, d, "POSTGRES_*"})
		}
	}
	if s.pgdb != nil {
		if _, adminUser, adminPass, adminErr := s.pgdb.AdminExecCreds(ctx); adminErr == nil && adminUser != "" {
			preferred := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
			if preferred == "" {
				preferred = "barn"
			}
			tries = append(tries, cred{adminUser, adminPass, preferred, "pdb_admin"})
		}
	}

	var lastErr error
	for _, t := range tries {
		dbName, probeErr := s.resolvePanelDatabaseName(ctx, container, t.user, t.pass, t.preferredDB, managed)
		if probeErr != nil {
			lastErr = fmt.Errorf("%s: %w", t.source, probeErr)
			s.logger.WarnContext(ctx, "panel dump probe failed", "source", t.source, "error", probeErr)
			continue
		}
		s.logger.InfoContext(ctx, "panel dump connection resolved",
			"source", t.source, "user", t.user, "database", dbName)
		return panelDumpConn{
			container: container,
			user:      t.user,
			password:  t.pass,
			dbName:    dbName,
			host:      "127.0.0.1",
			port:      "5432",
		}, nil
	}
	if lastErr != nil {
		return panelDumpConn{}, lastErr
	}
	return panelDumpConn{}, fmt.Errorf("no credentials available for panel dump")
}

func (s *Service) managedDBSet(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	if s.pgdb == nil {
		return out
	}
	names, err := s.pgdb.ListManagedDatabaseNames(ctx)
	if err != nil {
		return out
	}
	for _, n := range names {
		out[strings.TrimSpace(n)] = true
	}
	return out
}

func (s *Service) probeLiveDatabaseURL(ctx context.Context) (panelDumpConn, error) {
	_, password, _, err := parseDatabaseURL(s.databaseURL)
	if err != nil {
		return panelDumpConn{}, err
	}
	conn, err := pgx.Connect(ctx, s.databaseURL)
	if err != nil {
		return panelDumpConn{}, err
	}
	defer conn.Close(ctx)

	var dbName, user string
	if err := conn.QueryRow(ctx, "SELECT current_database(), current_user").Scan(&dbName, &user); err != nil {
		return panelDumpConn{}, err
	}
	var hasPanel bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name IN ('sites', 'goose_db_version', 'pdb_instances')
		)`).Scan(&hasPanel); err != nil {
		return panelDumpConn{}, err
	}
	if !hasPanel {
		return panelDumpConn{}, fmt.Errorf("database %q has no panel schema (sites/goose_db_version)", dbName)
	}
	return panelDumpConn{user: user, password: password, dbName: dbName}, nil
}

// discoverPanelDBViaPgx lists DBs on the API's Postgres endpoint and picks one
// with panel schema, skipping managed application databases.
func (s *Service) discoverPanelDBViaPgx(ctx context.Context, managed map[string]bool) (panelDumpConn, error) {
	user, password, _, err := parseDatabaseURL(s.databaseURL)
	if err != nil {
		return panelDumpConn{}, err
	}

	var listed []string
	var lastErr error
	for _, maint := range []string{"postgres", "barn", "dockpilot", user} {
		maintURL, rwErr := rewriteDatabaseURLPath(s.databaseURL, maint)
		if rwErr != nil {
			lastErr = rwErr
			continue
		}
		conn, connErr := pgx.Connect(ctx, maintURL)
		if connErr != nil {
			lastErr = connErr
			continue
		}
		rows, qErr := conn.Query(ctx, `SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`)
		if qErr != nil {
			conn.Close(ctx)
			lastErr = qErr
			continue
		}
		for rows.Next() {
			var name string
			if scanErr := rows.Scan(&name); scanErr == nil && name != "" {
				listed = append(listed, name)
			}
		}
		rows.Close()
		conn.Close(ctx)
		if len(listed) > 0 {
			lastErr = nil
			break
		}
	}
	if len(listed) == 0 {
		if lastErr != nil {
			return panelDumpConn{}, lastErr
		}
		return panelDumpConn{}, fmt.Errorf("could not list databases via DATABASE_URL")
	}

	preferred := []string{}
	if d := strings.TrimSpace(os.Getenv("POSTGRES_DB")); d != "" {
		preferred = append(preferred, d)
	}
	preferred = append(preferred, "barn", "dockpilot", "postgres")
	seen := map[string]struct{}{}
	var order []string
	for _, name := range append(preferred, listed...) {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		order = append(order, name)
	}

	for _, name := range order {
		if managed[name] {
			continue
		}
		dbURL, rwErr := rewriteDatabaseURLPath(s.databaseURL, name)
		if rwErr != nil {
			continue
		}
		conn, connErr := pgx.Connect(ctx, dbURL)
		if connErr != nil {
			continue
		}
		var hasPanel bool
		_ = conn.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name IN ('sites', 'goose_db_version', 'pdb_instances')
			)`).Scan(&hasPanel)
		var curUser string
		_ = conn.QueryRow(ctx, `SELECT current_user`).Scan(&curUser)
		conn.Close(ctx)
		if !hasPanel {
			continue
		}
		if curUser == "" {
			curUser = user
		}
		return panelDumpConn{user: curUser, password: password, dbName: name}, nil
	}

	return panelDumpConn{}, fmt.Errorf("panel database not found via pgx; listed=[%s]", strings.Join(listed, ","))
}

// resolvePanelDatabaseName picks the panel DB among preferred names / discovery.
// Skips managed application databases — those are backed up separately.
func (s *Service) resolvePanelDatabaseName(ctx context.Context, container, user, password, preferred string, managed map[string]bool) (string, error) {
	seen := map[string]struct{}{}
	var candidates []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || managed[name] {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		candidates = append(candidates, name)
	}
	add(preferred)
	add(os.Getenv("POSTGRES_DB"))
	add("barn")
	add("dockpilot")
	add("postgres")

	var errs []string
	for _, name := range candidates {
		ok, detail := s.probeDatabase(ctx, container, user, password, name)
		if !ok {
			if detail != "" {
				errs = append(errs, name+": "+detail)
			}
			continue
		}
		if s.databaseHasPanelSchema(ctx, container, user, password, name) {
			return name, nil
		}
		errs = append(errs, name+": exists but no panel schema")
	}

	listed, listErr := s.listDatabases(ctx, container, user, password)
	if listErr != nil {
		errs = append(errs, "list: "+listErr.Error())
	} else {
		for _, name := range listed {
			if managed[name] {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			if s.databaseHasPanelSchema(ctx, container, user, password, name) {
				s.logger.InfoContext(ctx, "panel database discovered via schema", "database", name)
				return name, nil
			}
		}
		errs = append(errs, "listed=["+strings.Join(listed, ",")+"]")
	}

	return "", fmt.Errorf("panel database not found (%s)", strings.Join(errs, "; "))
}

func (s *Service) probeDatabase(ctx context.Context, container, user, password, dbName string) (ok bool, detail string) {
	var stdout, stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: container,
		Cmd: []string{
			"psql",
			"-h", "127.0.0.1",
			"-p", "5432",
			"-U", user,
			"-d", dbName,
			"-tAc", "SELECT 1",
		},
		Env: []string{"PGPASSWORD=" + password},
	}, nil, &stdout, &stderr)
	if err != nil {
		return false, err.Error()
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("exit %d", code)
		}
		return false, msg
	}
	return strings.TrimSpace(stdout.String()) == "1", ""
}

func (s *Service) listDatabases(ctx context.Context, container, user, password string) ([]string, error) {
	// Connect via maintenance DB candidates.
	for _, maintenance := range []string{"postgres", "barn", "dockpilot", user} {
		var stdout, stderr bytes.Buffer
		code, err := s.docker.Exec(ctx, docker.ExecOptions{
			ContainerName: container,
			Cmd: []string{
				"psql",
				"-h", "127.0.0.1",
				"-p", "5432",
				"-U", user,
				"-d", maintenance,
				"-tAc", "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname",
			},
			Env: []string{"PGPASSWORD=" + password},
		}, nil, &stdout, &stderr)
		if err != nil || code != 0 {
			continue
		}
		var out []string
		for _, line := range strings.Split(stdout.String(), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, fmt.Errorf("could not list databases as %s", user)
}

func (s *Service) databaseHasPanelSchema(ctx context.Context, container, user, password, dbName string) bool {
	var stdout, stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: container,
		Cmd: []string{
			"psql",
			"-h", "127.0.0.1",
			"-p", "5432",
			"-U", user,
			"-d", dbName,
			"-tAc", "SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('sites','goose_db_version','pdb_instances') LIMIT 1",
		},
		Env: []string{"PGPASSWORD=" + password},
	}, nil, &stdout, &stderr)
	return err == nil && code == 0 && strings.TrimSpace(stdout.String()) == "1"
}

func buildSecretsEnv() string {
	keys := []string{
		"SECRETS_ENCRYPTION_KEY",
		"API_TOKEN",
		"POSTGRES_PASSWORD",
		"POSTGRES_USER",
		"POSTGRES_DB",
		"DATABASE_URL",
	}
	var b strings.Builder
	b.WriteString("# Barn restore secrets — keep private\n")
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			fmt.Fprintf(&b, "%s=%s\n", k, v)
		}
	}
	return b.String()
}

func writeTarFile(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o600,
		Size:    int64(len(data)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func gzipBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(in); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzipBytes(in []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	return io.ReadAll(gr)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func defaultStr(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
