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
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
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

	// Create operation record
	op, _ := s.queries.CreateBackupOperation(ctx, db.CreateBackupOperationParams{
		Kind:         "panel_snapshot",
		Status:       "running",
		DatabaseName: "panel",
		InstanceID:   pgtype.UUID{},
		ScheduleID:   pgtype.UUID{},
		S3Key:        "",
		SizeBytes:    0,
		Message:      "",
	})

	info, err := s.buildAndUpload(ctx, cfg, prefix)
	status, lastErr := "ok", ""
	if err != nil {
		status = "failed"
		lastErr = truncate(err.Error(), 2000)
	}
	_, _ = s.queries.UpdatePanelBackupRun(ctx, db.UpdatePanelBackupRunParams{
		LastRunAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		LastStatus: status,
		LastError:  lastErr,
	})

	// Finish operation record
	if op.ID != (uuid.UUID{}) {
		// backup_operations.status CHECK allows only: running | ok | failed
		opStatus := "ok"
		opMsg := ""
		opKey := ""
		opSize := int64(0)
		if err != nil {
			opStatus = "failed"
			opMsg = truncate(err.Error(), 2000)
		} else {
			opKey = info.S3Key
			opSize = info.SizeBytes
		}
		if _, finErr := s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
			ID:        op.ID,
			Status:    opStatus,
			Message:   opMsg,
			S3Key:     opKey,
			SizeBytes: opSize,
		}); finErr != nil {
			s.logger.WarnContext(ctx, "panel backup: finish operation failed",
				"operation_id", op.ID, "error", finErr)
		}
	}

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

	// Create operation record
	op, _ := s.queries.CreateBackupOperation(ctx, db.CreateBackupOperationParams{
		Kind:         "panel_restore",
		Status:       "running",
		DatabaseName: "panel",
		InstanceID:   pgtype.UUID{},
		ScheduleID:   pgtype.UUID{},
		S3Key:        key,
		SizeBytes:    0,
		Message:      "",
	})

	cfg, _, err := s.s3Config(ctx)
	if err != nil {
		if op.ID != (uuid.UUID{}) {
			_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
				ID:        op.ID,
				Status:    "failed",
				Message:   truncate(err.Error(), 2000),
				S3Key:     key,
				SizeBytes: 0,
			})
		}
		return err
	}
	log("info", "Downloading "+key)
	body, err := s3util.Download(ctx, cfg, key)
	if err != nil {
		if op.ID != (uuid.UUID{}) {
			_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
				ID:        op.ID,
				Status:    "failed",
				Message:   truncate(err.Error(), 2000),
				S3Key:     key,
				SizeBytes: 0,
			})
		}
		return err
	}
	defer body.Close()

	gz, err := gzip.NewReader(body)
	if err != nil {
		if op.ID != (uuid.UUID{}) {
			_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
				ID:        op.ID,
				Status:    "failed",
				Message:   truncate(fmt.Sprintf("invalid gzip bundle: %v", err), 2000),
				S3Key:     key,
				SizeBytes: 0,
			})
		}
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
			if op.ID != (uuid.UUID{}) {
				_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
					ID:        op.ID,
					Status:    "failed",
					Message:   truncate(err.Error(), 2000),
					S3Key:     key,
					SizeBytes: 0,
				})
			}
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, 512<<20))
		if err != nil {
			if op.ID != (uuid.UUID{}) {
				_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
					ID:        op.ID,
					Status:    "failed",
					Message:   truncate(err.Error(), 2000),
					S3Key:     key,
					SizeBytes: 0,
				})
			}
			return err
		}
		name := hdr.Name
		switch {
		case name == "panel/barn.sql.gz" || strings.HasSuffix(name, "/panel/barn.sql.gz") ||
			name == "panel/dockpilot.sql.gz" || strings.HasSuffix(name, "/panel/dockpilot.sql.gz"):
			panelSQL, err = gunzipBytes(data)
			if err != nil {
				if op.ID != (uuid.UUID{}) {
					_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
						ID:        op.ID,
						Status:    "failed",
						Message:   truncate(err.Error(), 2000),
						S3Key:     key,
						SizeBytes: 0,
					})
				}
				return err
			}
			log("info", fmt.Sprintf("Found panel dump (%d bytes)", len(panelSQL)))
		case strings.HasPrefix(name, "managed/") && strings.HasSuffix(name, ".sql.gz"):
			base := path.Base(name)
			dbName := strings.TrimSuffix(base, ".sql.gz")
			plain, err := gunzipBytes(data)
			if err != nil {
				if op.ID != (uuid.UUID{}) {
					_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
						ID:        op.ID,
						Status:    "failed",
						Message:   truncate(err.Error(), 2000),
						S3Key:     key,
						SizeBytes: 0,
					})
				}
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
		if op.ID != (uuid.UUID{}) {
			_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
				ID:        op.ID,
				Status:    "failed",
				Message:   "panel dump missing from bundle",
				S3Key:     key,
				SizeBytes: 0,
			})
		}
		return fmt.Errorf("%w: panel dump missing from bundle", ErrInvalidInput)
	}
	log("info", "Restoring panel database…")
	if err := s.restorePanelSQL(ctx, bytes.NewReader(panelSQL)); err != nil {
		if op.ID != (uuid.UUID{}) {
			_, _ = s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
				ID:        op.ID,
				Status:    "failed",
				Message:   truncate(fmt.Sprintf("restore panel database: %v", err), 2000),
				S3Key:     key,
				SizeBytes: 0,
			})
		}
		return fmt.Errorf("restore panel database: %w", err)
	}
	log("info", "Panel database restored")

	if len(managed) > 0 {
		log("info", fmt.Sprintf(
			"Skipping %d managed database dump(s) in bundle — restore those from per-database backups",
			len(managed),
		))
	}

	// Finish operation record
	if op.ID != (uuid.UUID{}) {
		if _, finErr := s.queries.FinishBackupOperation(ctx, db.FinishBackupOperationParams{
			ID:        op.ID,
			Status:    "ok",
			Message:   "Restore completed",
			S3Key:     key,
			SizeBytes: 0,
		}); finErr != nil {
			s.logger.WarnContext(ctx, "panel restore: finish operation failed",
				"operation_id", op.ID, "error", finErr)
		}
	}

	log("info", "Full restore completed — redeploy sites from git if needed")
	return nil
}

func (s *Service) TestS3(ctx context.Context, req TestS3Request) (TestS3Response, error) {
	var cfg s3util.Config

	access := strings.TrimSpace(req.S3AccessKey)
	secret := strings.TrimSpace(req.S3SecretKey)

	if access == "" || secret == "" {
		// Use saved panel credentials
		existingCfg, _, err := s.s3Config(ctx)
		if err != nil {
			return TestS3Response{OK: false, Message: "No saved S3 credentials and none provided"}, nil
		}
		cfg = existingCfg
		// Override with request fields if provided
		if strings.TrimSpace(req.S3Bucket) != "" {
			cfg.Bucket = strings.TrimSpace(req.S3Bucket)
		}
		if strings.TrimSpace(req.S3Endpoint) != "" {
			cfg.Endpoint = strings.TrimSpace(req.S3Endpoint)
		}
		if strings.TrimSpace(req.S3Region) != "" {
			cfg.Region = strings.TrimSpace(req.S3Region)
		}
		cfg.ForcePathStyle = req.S3ForcePathStyle
	} else {
		// Use request credentials
		bucket := strings.TrimSpace(req.S3Bucket)
		if bucket == "" {
			return TestS3Response{OK: false, Message: "s3_bucket is required"}, nil
		}
		region := strings.TrimSpace(req.S3Region)
		if region == "" {
			region = "ru-central1"
		}
		cfg = s3util.Config{
			Endpoint:       strings.TrimSpace(req.S3Endpoint),
			Region:         region,
			Bucket:         bucket,
			AccessKey:      access,
			SecretKey:      secret,
			ForcePathStyle: req.S3ForcePathStyle,
		}
	}

	if err := s3util.Ping(ctx, cfg); err != nil {
		// Don't leak secrets in error message
		msg := err.Error()
		msg = strings.ReplaceAll(msg, cfg.AccessKey, "***")
		msg = strings.ReplaceAll(msg, cfg.SecretKey, "***")
		return TestS3Response{OK: false, Message: msg}, nil
	}

	return TestS3Response{OK: true, Message: "Connection successful"}, nil
}

func (s *Service) ListOperations(ctx context.Context, limit int32) ([]OperationResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.queries.ListBackupOperations(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]OperationResponse, 0, len(rows))
	for _, row := range rows {
		var instanceID, scheduleID *string
		if row.InstanceID.Valid {
			id := uuid.UUID(row.InstanceID.Bytes).String()
			instanceID = &id
		}
		if row.ScheduleID.Valid {
			id := uuid.UUID(row.ScheduleID.Bytes).String()
			scheduleID = &id
		}
		var finishedAt *time.Time
		if row.FinishedAt.Valid {
			t := row.FinishedAt.Time.UTC()
			finishedAt = &t
		}
		out = append(out, OperationResponse{
			ID:           row.ID.String(),
			Kind:         row.Kind,
			Status:       row.Status,
			DatabaseName: row.DatabaseName,
			InstanceID:   instanceID,
			ScheduleID:   scheduleID,
			S3Key:        row.S3Key,
			SizeBytes:    row.SizeBytes,
			Message:      row.Message,
			StartedAt:    row.StartedAt.UTC(),
			FinishedAt:   finishedAt,
		})
	}
	return out, nil
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
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: conn.container,
		Cmd: append(pgClientPrefix("pg_dump", conn.tcp),
			"-U", conn.user,
			"-d", conn.dbName,
			"--no-owner",
			"--no-acl",
		),
		Env: []string{"PGPASSWORD=" + conn.password},
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
	s.logger.InfoContext(ctx, "panel dump ok", "database", conn.dbName, "user", conn.user, "container", conn.container)
	return nil
}

func (s *Service) restorePanelSQL(ctx context.Context, r io.Reader) error {
	conn, err := s.resolvePanelDumpConn(ctx)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: conn.container,
		Cmd: append(pgClientPrefix("psql", conn.tcp),
			"-v", "ON_ERROR_STOP=1",
			"-U", conn.user,
			"-d", conn.dbName,
		),
		Env: []string{"PGPASSWORD=" + conn.password},
	}, r, io.Discard, &stderr)
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

// pgClientPrefix returns [bin] or [bin, -h, 127.0.0.1].
func pgClientPrefix(bin string, tcp bool) []string {
	if tcp {
		return []string{bin, "-h", "127.0.0.1"}
	}
	return []string{bin}
}

// resolvePanelPostgresContainer picks the running panel Postgres container.
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

	st, err := s.docker.InspectContainer(ctx, "dock-pilot-postgres", "dockpilot-postgres", "barn-postgres")
	if err != nil {
		return "", err
	}
	if !st.Found {
		return "", fmt.Errorf("postgres container not found (tried dock-pilot-postgres, dockpilot-postgres, barn-postgres)")
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
		LastError:        row.LastError,
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

type panelDumpConn struct {
	container string
	user      string
	password  string
	dbName    string
	tcp       bool
}

// resolvePanelDumpConn uses the same DATABASE_URL the API already uses, then
// dumps that database via docker exec (pdb admin, same as managed DB dumps).
func (s *Service) resolvePanelDumpConn(ctx context.Context) (panelDumpConn, error) {
	container, err := s.resolvePanelPostgresContainer(ctx)
	if err != nil {
		// Fall back to managed postgres container name.
		if s.pgdb != nil {
			if c, _, _, _, adminErr := s.pgdb.AdminExecCreds(ctx); adminErr == nil && c != "" {
				container = c
			} else {
				return panelDumpConn{}, err
			}
		} else {
			return panelDumpConn{}, err
		}
	}

	adminUser, adminPass, adminTCP := "", "", false
	if s.pgdb != nil {
		if c, u, p, tcp, adminErr := s.pgdb.AdminExecCreds(ctx); adminErr == nil {
			adminUser, adminPass, adminTCP = u, p, tcp
			if c != "" {
				container = c
			}
		}
	}

	// 1) Exact DB the running API is connected to (panel only — never dump managed DBs here).
	if live, liveErr := s.probeLiveDatabaseURL(ctx); liveErr == nil {
		managed := s.managedDBSet(ctx)
		if managed[live.dbName] {
			s.logger.WarnContext(ctx, "DATABASE_URL points at a managed DB name; searching for panel DB instead",
				"database", live.dbName)
		} else {
			s.logger.InfoContext(ctx, "panel DB from live DATABASE_URL",
				"database", live.dbName, "user", live.user)
			if adminUser != "" {
				if ok, _ := s.probeDatabase(ctx, container, adminUser, adminPass, live.dbName, adminTCP); ok {
					return panelDumpConn{container: container, user: adminUser, password: adminPass, dbName: live.dbName, tcp: adminTCP}, nil
				}
			}
			if ok, _ := s.probeDatabase(ctx, container, live.user, live.password, live.dbName, true); ok {
				return panelDumpConn{container: container, user: live.user, password: live.password, dbName: live.dbName, tcp: true}, nil
			}
			if ok, detail := s.probeDatabase(ctx, container, live.user, live.password, live.dbName, false); ok {
				return panelDumpConn{container: container, user: live.user, password: live.password, dbName: live.dbName, tcp: false}, nil
			} else {
				s.logger.WarnContext(ctx, "live DB not reachable via docker exec",
					"database", live.dbName, "detail", detail)
			}
		}
	} else {
		s.logger.WarnContext(ctx, "DATABASE_URL live probe failed", "error", liveErr)
	}

	// 2) Find the panel DB by public.sites, skipping managed application databases
	// (those are backed up separately — never included in the full panel snapshot).
	if adminUser == "" {
		return panelDumpConn{}, fmt.Errorf("no admin credentials and DATABASE_URL probe failed")
	}
	managed := s.managedDBSet(ctx)
	dbName, err := s.findPanelDatabase(ctx, container, adminUser, adminPass, adminTCP, managed)
	if err != nil {
		return panelDumpConn{}, err
	}
	s.logger.InfoContext(ctx, "panel dump connection resolved",
		"source", "sites_table", "user", adminUser, "database", dbName, "container", container, "tcp", adminTCP)
	return panelDumpConn{container: container, user: adminUser, password: adminPass, dbName: dbName, tcp: adminTCP}, nil
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
	var hasSites bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'sites'
		)`).Scan(&hasSites); err != nil {
		return panelDumpConn{}, err
	}
	if !hasSites {
		return panelDumpConn{}, fmt.Errorf("database %q has no public.sites table", dbName)
	}
	return panelDumpConn{user: user, password: password, dbName: dbName}, nil
}

// findPanelDatabase returns a non-managed DB that has public.sites (panel schema).
func (s *Service) findPanelDatabase(ctx context.Context, container, user, password string, tcp bool, managed map[string]bool) (string, error) {
	listed, err := s.listDatabases(ctx, container, user, password, tcp)
	if err != nil {
		return "", err
	}
	var checked []string
	for _, name := range listed {
		if managed[name] {
			continue
		}
		checked = append(checked, name)
		if s.databaseHasSitesTable(ctx, container, user, password, name, tcp) {
			return name, nil
		}
	}
	return "", fmt.Errorf("no panel database with public.sites (checked non-managed=[%s])", strings.Join(checked, ","))
}

func (s *Service) probeDatabase(ctx context.Context, container, user, password, dbName string, tcp bool) (ok bool, detail string) {
	var stdout, stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: container,
		Cmd: append(pgClientPrefix("psql", tcp),
			"-U", user,
			"-d", dbName,
			"-tAc", "SELECT 1",
		),
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

func (s *Service) listDatabases(ctx context.Context, container, user, password string, tcp bool) ([]string, error) {
	for _, maintenance := range []string{"postgres", user} {
		var stdout, stderr bytes.Buffer
		code, err := s.docker.Exec(ctx, docker.ExecOptions{
			ContainerName: container,
			Cmd: append(pgClientPrefix("psql", tcp),
				"-U", user,
				"-d", maintenance,
				"-tAc", "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname",
			),
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

func (s *Service) databaseHasSitesTable(ctx context.Context, container, user, password, dbName string, tcp bool) bool {
	var stdout, stderr bytes.Buffer
	code, err := s.docker.Exec(ctx, docker.ExecOptions{
		ContainerName: container,
		Cmd: append(pgClientPrefix("psql", tcp),
			"-U", user,
			"-d", dbName,
			"-tAc", "SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'sites' LIMIT 1",
		),
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
