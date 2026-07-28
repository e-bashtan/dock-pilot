package pgdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/dock-pilot/backend/internal/db"
)

func (s *Service) ListSchedules(ctx context.Context, instanceID uuid.UUID) ([]ScheduleResponse, error) {
	if _, err := s.requireInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListPgBackupSchedules(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	out := make([]ScheduleResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toScheduleResponse(row))
	}
	return out, nil
}

func (s *Service) CreateSchedule(ctx context.Context, instanceID uuid.UUID, req CreateScheduleRequest) (ScheduleResponse, error) {
	if _, err := s.requireInstance(ctx, instanceID); err != nil {
		return ScheduleResponse{}, err
	}
	if err := validateScheduleTiming(req.Hour, req.Minute, req.Timezone); err != nil {
		return ScheduleResponse{}, err
	}
	if strings.TrimSpace(req.S3Bucket) == "" {
		return ScheduleResponse{}, fmt.Errorf("%w: s3_bucket is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.S3AccessKey) == "" || strings.TrimSpace(req.S3SecretKey) == "" {
		return ScheduleResponse{}, fmt.Errorf("%w: s3 credentials are required", ErrInvalidInput)
	}
	var dbID pgtype.UUID
	if req.DatabaseID != nil {
		database, err := s.queries.GetPgDatabase(ctx, *req.DatabaseID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ScheduleResponse{}, ErrNotFound
			}
			return ScheduleResponse{}, err
		}
		if database.InstanceID != instanceID {
			return ScheduleResponse{}, ErrNotFound
		}
		dbID = pgtype.UUID{Bytes: *req.DatabaseID, Valid: true}
	}
	encAccess, err := s.cipher.Encrypt(strings.TrimSpace(req.S3AccessKey))
	if err != nil {
		return ScheduleResponse{}, err
	}
	encSecret, err := s.cipher.Encrypt(strings.TrimSpace(req.S3SecretKey))
	if err != nil {
		return ScheduleResponse{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	retention := req.RetentionCount
	if retention == 0 {
		retention = 7
	}
	if retention < 1 || retention > 365 {
		return ScheduleResponse{}, fmt.Errorf("%w: retention_count must be 1-365", ErrInvalidInput)
	}
	prefix := strings.TrimSpace(req.S3Prefix)
	if prefix == "" {
		prefix = "dock-pilot/pg-backups"
	}
	region := strings.TrimSpace(req.S3Region)
	if region == "" {
		region = "us-east-1"
	}
	tz := strings.TrimSpace(req.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	row, err := s.queries.CreatePgBackupSchedule(ctx, db.CreatePgBackupScheduleParams{
		InstanceID:           instanceID,
		DatabaseID:           dbID,
		Enabled:              enabled,
		Hour:                 int32(req.Hour),
		Minute:               int32(req.Minute),
		Timezone:             tz,
		S3Endpoint:           strings.TrimSpace(req.S3Endpoint),
		S3Region:             region,
		S3Bucket:             strings.TrimSpace(req.S3Bucket),
		S3Prefix:             strings.Trim(prefix, "/"),
		EncryptedS3AccessKey: encAccess,
		EncryptedS3SecretKey: encSecret,
		S3ForcePathStyle:     req.S3ForcePathStyle,
		RetentionCount:       int32(retention),
	})
	if err != nil {
		return ScheduleResponse{}, err
	}
	return toScheduleResponse(row), nil
}

func (s *Service) UpdateSchedule(ctx context.Context, instanceID, scheduleID uuid.UUID, req UpdateScheduleRequest) (ScheduleResponse, error) {
	if _, err := s.requireInstance(ctx, instanceID); err != nil {
		return ScheduleResponse{}, err
	}
	existing, err := s.queries.GetPgBackupSchedule(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ScheduleResponse{}, ErrNotFound
		}
		return ScheduleResponse{}, err
	}
	if existing.InstanceID != instanceID {
		return ScheduleResponse{}, ErrNotFound
	}

	params := db.UpdatePgBackupScheduleParams{ID: scheduleID}
	if req.ClearDatabaseID {
		params.ClearDatabaseID = pgtype.Bool{Bool: true, Valid: true}
	} else if req.DatabaseID != nil {
		database, err := s.queries.GetPgDatabase(ctx, *req.DatabaseID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ScheduleResponse{}, ErrNotFound
			}
			return ScheduleResponse{}, err
		}
		if database.InstanceID != instanceID {
			return ScheduleResponse{}, ErrNotFound
		}
		params.DatabaseID = pgtype.UUID{Bytes: *req.DatabaseID, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}
	if req.Hour != nil {
		if *req.Hour < 0 || *req.Hour > 23 {
			return ScheduleResponse{}, fmt.Errorf("%w: hour must be 0-23", ErrInvalidInput)
		}
		params.Hour = pgtype.Int4{Int32: int32(*req.Hour), Valid: true}
	}
	if req.Minute != nil {
		if *req.Minute < 0 || *req.Minute > 59 {
			return ScheduleResponse{}, fmt.Errorf("%w: minute must be 0-59", ErrInvalidInput)
		}
		params.Minute = pgtype.Int4{Int32: int32(*req.Minute), Valid: true}
	}
	if req.Timezone != nil {
		tz := strings.TrimSpace(*req.Timezone)
		if _, err := time.LoadLocation(tz); err != nil {
			return ScheduleResponse{}, fmt.Errorf("%w: invalid timezone", ErrInvalidInput)
		}
		params.Timezone = pgtype.Text{String: tz, Valid: true}
	}
	if req.S3Endpoint != nil {
		params.S3Endpoint = pgtype.Text{String: strings.TrimSpace(*req.S3Endpoint), Valid: true}
	}
	if req.S3Region != nil {
		region := strings.TrimSpace(*req.S3Region)
		if region == "" {
			region = "us-east-1"
		}
		params.S3Region = pgtype.Text{String: region, Valid: true}
	}
	if req.S3Bucket != nil {
		bucket := strings.TrimSpace(*req.S3Bucket)
		if bucket == "" {
			return ScheduleResponse{}, fmt.Errorf("%w: s3_bucket is required", ErrInvalidInput)
		}
		params.S3Bucket = pgtype.Text{String: bucket, Valid: true}
	}
	if req.S3Prefix != nil {
		prefix := strings.Trim(strings.TrimSpace(*req.S3Prefix), "/")
		if prefix == "" {
			prefix = "dock-pilot/pg-backups"
		}
		params.S3Prefix = pgtype.Text{String: prefix, Valid: true}
	}
	if req.S3AccessKey != nil && strings.TrimSpace(*req.S3AccessKey) != "" {
		enc, err := s.cipher.Encrypt(strings.TrimSpace(*req.S3AccessKey))
		if err != nil {
			return ScheduleResponse{}, err
		}
		params.EncryptedS3AccessKey = enc
	}
	if req.S3SecretKey != nil && strings.TrimSpace(*req.S3SecretKey) != "" {
		enc, err := s.cipher.Encrypt(strings.TrimSpace(*req.S3SecretKey))
		if err != nil {
			return ScheduleResponse{}, err
		}
		params.EncryptedS3SecretKey = enc
	}
	if req.S3ForcePathStyle != nil {
		params.S3ForcePathStyle = pgtype.Bool{Bool: *req.S3ForcePathStyle, Valid: true}
	}
	if req.RetentionCount != nil {
		if *req.RetentionCount < 1 || *req.RetentionCount > 365 {
			return ScheduleResponse{}, fmt.Errorf("%w: retention_count must be 1-365", ErrInvalidInput)
		}
		params.RetentionCount = pgtype.Int4{Int32: int32(*req.RetentionCount), Valid: true}
	}

	row, err := s.queries.UpdatePgBackupSchedule(ctx, params)
	if err != nil {
		return ScheduleResponse{}, err
	}
	return toScheduleResponse(row), nil
}

func (s *Service) DeleteSchedule(ctx context.Context, instanceID, scheduleID uuid.UUID) error {
	existing, err := s.queries.GetPgBackupSchedule(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if existing.InstanceID != instanceID {
		return ErrNotFound
	}
	return s.queries.DeletePgBackupSchedule(ctx, scheduleID)
}

func (s *Service) ListBackups(ctx context.Context, instanceID uuid.UUID, scheduleID *uuid.UUID, limit int32) ([]BackupResponse, error) {
	if _, err := s.requireInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	schedule, err := s.resolveSchedule(ctx, instanceID, scheduleID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []BackupResponse{}, nil
		}
		return nil, err
	}
	cfg, err := s.s3ConfigFromSchedule(schedule)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	objects, err := listFromS3(ctx, cfg, schedule.S3Prefix, int(limit))
	if err != nil {
		return nil, err
	}
	sid := schedule.ID
	out := make([]BackupResponse, 0, len(objects))
	for _, obj := range objects {
		out = append(out, BackupResponse{
			S3Key:        obj.Key,
			DatabaseName: databaseNameFromS3Key(schedule.S3Prefix, obj.Key),
			Status:       "success",
			S3Endpoint:   schedule.S3Endpoint,
			S3Region:     schedule.S3Region,
			S3Bucket:     schedule.S3Bucket,
			SizeBytes:    obj.Size,
			CreatedAt:    obj.LastModified,
			ScheduleID:   &sid,
		})
	}
	return out, nil
}

func (s *Service) ManualBackup(ctx context.Context, instanceID uuid.UUID, req ManualBackupRequest) (BackupResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return BackupResponse{}, err
	}
	database, err := s.queries.GetPgDatabase(ctx, req.DatabaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BackupResponse{}, ErrNotFound
		}
		return BackupResponse{}, err
	}
	if database.InstanceID != instanceID {
		return BackupResponse{}, ErrNotFound
	}

	cfg := s3Config{
		Endpoint:       strings.TrimSpace(req.S3Endpoint),
		Region:         defaultStr(req.S3Region, "us-east-1"),
		Bucket:         strings.TrimSpace(req.S3Bucket),
		AccessKey:      strings.TrimSpace(req.S3AccessKey),
		SecretKey:      strings.TrimSpace(req.S3SecretKey),
		ForcePathStyle: req.S3ForcePathStyle,
	}
	prefix := defaultStr(req.S3Prefix, "dock-pilot/pg-backups")
	var scheduleID *uuid.UUID
	var retention int

	if req.ScheduleID != nil {
		schedule, err := s.queries.GetPgBackupSchedule(ctx, *req.ScheduleID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return BackupResponse{}, ErrNotFound
			}
			return BackupResponse{}, err
		}
		if schedule.InstanceID != instanceID {
			return BackupResponse{}, ErrNotFound
		}
		cfg, err = s.s3ConfigFromSchedule(schedule)
		if err != nil {
			return BackupResponse{}, err
		}
		prefix = schedule.S3Prefix
		id := schedule.ID
		scheduleID = &id
		retention = int(schedule.RetentionCount)
	} else if cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return BackupResponse{}, fmt.Errorf("%w: provide schedule_id or S3 settings", ErrInvalidInput)
	}

	backup, err := s.runBackup(ctx, inst, database, scheduleID, cfg, prefix)
	if err != nil {
		return BackupResponse{}, err
	}
	if retention > 0 {
		_ = s.applyRetentionS3(ctx, cfg, prefix, inst.Slug, database.Name, retention)
	}
	return backup, nil
}

func (s *Service) RestoreBackup(ctx context.Context, instanceID uuid.UUID, req RestoreRequest) (DatabaseResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return DatabaseResponse{}, err
	}
	key := strings.TrimSpace(req.S3Key)
	if key == "" {
		return DatabaseResponse{}, fmt.Errorf("%w: s3_key is required", ErrInvalidInput)
	}

	schedule, err := s.resolveSchedule(ctx, instanceID, &req.ScheduleID)
	if err != nil {
		return DatabaseResponse{}, err
	}
	cfg, err := s.s3ConfigFromSchedule(schedule)
	if err != nil {
		return DatabaseResponse{}, err
	}

	targetName := strings.TrimSpace(req.TargetDatabaseName)
	if targetName == "" {
		targetName = databaseNameFromS3Key(schedule.S3Prefix, key)
	}

	body, err := downloadFromS3(ctx, cfg, key)
	if err != nil {
		return DatabaseResponse{}, err
	}
	defer body.Close()

	return s.restoreDumpInto(ctx, inst, RestoreUploadOptions{
		TargetDatabaseName: targetName,
		CreateDatabase:     req.CreateDatabase,
		DropExisting:       req.DropExisting,
	}, body)
}

func (s *Service) RestoreFromUpload(ctx context.Context, instanceID uuid.UUID, opts RestoreUploadOptions, body io.Reader) (DatabaseResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return DatabaseResponse{}, err
	}
	return s.restoreDumpInto(ctx, inst, opts, body)
}

func (s *Service) restoreDumpInto(ctx context.Context, inst db.PdbInstance, opts RestoreUploadOptions, body io.Reader) (DatabaseResponse, error) {
	targetName := strings.TrimSpace(opts.TargetDatabaseName)
	if err := validateDBName(targetName); err != nil {
		return DatabaseResponse{}, err
	}

	dump, closer, err := openSQLDump(body)
	if err != nil {
		return DatabaseResponse{}, err
	}
	if closer != nil {
		defer closer.Close()
	}

	instanceID := inst.ID
	existingPanelDB, panelErr := findDatabaseByName(ctx, s.queries, instanceID, targetName)
	dbIdent, err := quoteIdent(targetName)
	if err != nil {
		return DatabaseResponse{}, err
	}

	createDB := opts.CreateDatabase
	if opts.DropExisting {
		_ = s.execSQL(ctx, inst, "postgres", fmt.Sprintf(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid()",
			quoteLiteral(targetName),
		))
		if err := s.execSQL(ctx, inst, "postgres", "DROP DATABASE IF EXISTS "+dbIdent); err != nil {
			return DatabaseResponse{}, err
		}
		if panelErr == nil {
			_ = s.queries.DeletePgDatabase(ctx, existingPanelDB.ID)
		}
		createDB = true
	}

	if createDB || panelErr != nil {
		ownerIdent, err := quoteIdent(inst.AdminUser)
		if err != nil {
			return DatabaseResponse{}, err
		}
		if err := s.execSQL(ctx, inst, "postgres", fmt.Sprintf("CREATE DATABASE %s OWNER %s", dbIdent, ownerIdent)); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
				return DatabaseResponse{}, err
			}
		}
	}

	if err := s.restoreDatabase(ctx, inst, targetName, dump); err != nil {
		return DatabaseResponse{}, fmt.Errorf("restore dump: %w", err)
	}

	row, err := findDatabaseByName(ctx, s.queries, instanceID, targetName)
	if err != nil {
		created, cerr := s.queries.CreatePgDatabase(ctx, db.CreatePgDatabaseParams{
			InstanceID: instanceID,
			Name:       targetName,
			OwnerRole:  inst.AdminUser,
		})
		if cerr != nil {
			return DatabaseResponse{}, cerr
		}
		return toDatabaseResponse(created), nil
	}
	return toDatabaseResponse(row), nil
}

func (s *Service) runBackup(ctx context.Context, inst db.PdbInstance, database db.PdbDatabase, scheduleID *uuid.UUID, cfg s3Config, prefix string) (BackupResponse, error) {
	key := path.Join(strings.Trim(prefix, "/"), inst.Slug, database.Name, time.Now().UTC().Format("20060102-150405")+".sql")

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		errCh <- s.dumpDatabase(ctx, inst, database.Name, pw)
	}()

	size, uploadErr := uploadToS3(ctx, cfg, key, pr)
	dumpErr := <-errCh
	_ = pr.Close()
	if dumpErr != nil || uploadErr != nil {
		return BackupResponse{}, firstErr(dumpErr, uploadErr)
	}

	now := time.Now().UTC()
	return BackupResponse{
		S3Key:        key,
		DatabaseName: database.Name,
		Status:       "success",
		S3Endpoint:   cfg.Endpoint,
		S3Region:     cfg.Region,
		S3Bucket:     cfg.Bucket,
		SizeBytes:    size,
		CreatedAt:    now,
		ScheduleID:   scheduleID,
	}, nil
}

func (s *Service) applyRetentionS3(ctx context.Context, cfg s3Config, prefix, slug, databaseName string, keep int) error {
	if keep <= 0 {
		return nil
	}
	dbPrefix := path.Join(strings.Trim(prefix, "/"), slug, databaseName)
	objects, err := listFromS3(ctx, cfg, dbPrefix, 500)
	if err != nil {
		return err
	}
	if len(objects) <= keep {
		return nil
	}
	for _, old := range objects[keep:] {
		_ = deleteFromS3(ctx, cfg, old.Key)
	}
	return nil
}

func (s *Service) resolveSchedule(ctx context.Context, instanceID uuid.UUID, scheduleID *uuid.UUID) (db.PdbBackupSchedule, error) {
	if scheduleID != nil && *scheduleID != uuid.Nil {
		schedule, err := s.queries.GetPgBackupSchedule(ctx, *scheduleID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.PdbBackupSchedule{}, ErrNotFound
			}
			return db.PdbBackupSchedule{}, err
		}
		if schedule.InstanceID != instanceID {
			return db.PdbBackupSchedule{}, ErrNotFound
		}
		return schedule, nil
	}
	rows, err := s.queries.ListPgBackupSchedules(ctx, instanceID)
	if err != nil {
		return db.PdbBackupSchedule{}, err
	}
	if len(rows) == 0 {
		return db.PdbBackupSchedule{}, ErrNotFound
	}
	return rows[0], nil
}

func databaseNameFromS3Key(prefix, key string) string {
	rel := strings.TrimPrefix(strings.Trim(key, "/"), strings.Trim(prefix, "/")+"/")
	rel = strings.TrimPrefix(rel, "/")
	parts := strings.Split(rel, "/")
	switch {
	case len(parts) >= 3:
		// prefix/slug/dbname/file.sql
		return parts[len(parts)-2]
	case len(parts) == 2:
		return parts[0]
	case len(parts) == 1:
		name := parts[0]
		name = strings.TrimSuffix(name, ".sql.gz")
		name = strings.TrimSuffix(name, ".gz")
		name = strings.TrimSuffix(name, ".sql")
		return name
	default:
		return ""
	}
}

func (s *Service) s3ConfigFromSchedule(schedule db.PdbBackupSchedule) (s3Config, error) {
	access, err := s.cipher.Decrypt(schedule.EncryptedS3AccessKey)
	if err != nil {
		return s3Config{}, err
	}
	secret, err := s.cipher.Decrypt(schedule.EncryptedS3SecretKey)
	if err != nil {
		return s3Config{}, err
	}
	return s3Config{
		Endpoint:       schedule.S3Endpoint,
		Region:         schedule.S3Region,
		Bucket:         schedule.S3Bucket,
		AccessKey:      access,
		SecretKey:      secret,
		ForcePathStyle: schedule.S3ForcePathStyle,
	}, nil
}

func (s *Service) RunDueSchedules(ctx context.Context) error {
	schedules, err := s.queries.ListEnabledPgBackupSchedules(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, schedule := range schedules {
		if !scheduleDue(schedule, now) {
			continue
		}
		status := "ok"
		if err := s.runSchedule(ctx, schedule); err != nil {
			status = err.Error()
			s.logger.Warn("pg backup schedule failed", "schedule_id", schedule.ID, "error", err)
		}
		_, _ = s.queries.UpdatePgBackupScheduleRun(ctx, db.UpdatePgBackupScheduleRunParams{
			ID:         schedule.ID,
			LastRunAt:  pgtype.Timestamptz{Time: now.UTC(), Valid: true},
			LastStatus: truncate(status, 500),
		})
	}
	return nil
}

func (s *Service) runSchedule(ctx context.Context, schedule db.PdbBackupSchedule) error {
	inst, err := s.queries.GetPgInstance(ctx, schedule.InstanceID)
	if err != nil {
		return err
	}
	cfg, err := s.s3ConfigFromSchedule(schedule)
	if err != nil {
		return err
	}
	var databases []db.PdbDatabase
	if schedule.DatabaseID.Valid {
		database, err := s.queries.GetPgDatabase(ctx, uuid.UUID(schedule.DatabaseID.Bytes))
		if err != nil {
			return err
		}
		databases = []db.PdbDatabase{database}
	} else {
		databases, err = s.queries.ListPgDatabases(ctx, schedule.InstanceID)
		if err != nil {
			return err
		}
	}
	if len(databases) == 0 {
		return fmt.Errorf("no databases to back up")
	}
	sid := schedule.ID
	var first error
	for _, database := range databases {
		_, err := s.runBackup(ctx, inst, database, &sid, cfg, schedule.S3Prefix)
		if err != nil && first == nil {
			first = err
			continue
		}
		_ = s.applyRetentionS3(ctx, cfg, schedule.S3Prefix, inst.Slug, database.Name, int(schedule.RetentionCount))
	}
	return first
}

func scheduleDue(schedule db.PdbBackupSchedule, now time.Time) bool {
	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	if local.Hour() != int(schedule.Hour) || local.Minute() != int(schedule.Minute) {
		return false
	}
	if !schedule.LastRunAt.Valid {
		return true
	}
	last := schedule.LastRunAt.Time.In(loc)
	return last.Year() != local.Year() || last.YearDay() != local.YearDay()
}

func validateScheduleTiming(hour, minute int, tz string) error {
	if hour < 0 || hour > 23 {
		return fmt.Errorf("%w: hour must be 0-23", ErrInvalidInput)
	}
	if minute < 0 || minute > 59 {
		return fmt.Errorf("%w: minute must be 0-59", ErrInvalidInput)
	}
	if strings.TrimSpace(tz) == "" {
		return nil
	}
	if _, err := time.LoadLocation(strings.TrimSpace(tz)); err != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalidInput)
	}
	return nil
}

func toScheduleResponse(row db.PdbBackupSchedule) ScheduleResponse {
	return ScheduleResponse{
		ID:               row.ID,
		InstanceID:       row.InstanceID,
		DatabaseID:       optionalUUID(row.DatabaseID),
		Enabled:          row.Enabled,
		Hour:             int(row.Hour),
		Minute:           int(row.Minute),
		Timezone:         row.Timezone,
		S3Endpoint:       row.S3Endpoint,
		S3Region:         row.S3Region,
		S3Bucket:         row.S3Bucket,
		S3Prefix:         row.S3Prefix,
		S3ForcePathStyle: row.S3ForcePathStyle,
		RetentionCount:   int(row.RetentionCount),
		LastRunAt:        optionalTime(row.LastRunAt),
		LastStatus:       row.LastStatus,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func findDatabaseByName(ctx context.Context, q *db.Queries, instanceID uuid.UUID, name string) (db.PdbDatabase, error) {
	rows, err := q.ListPgDatabases(ctx, instanceID)
	if err != nil {
		return db.PdbDatabase{}, err
	}
	for _, row := range rows {
		if row.Name == name {
			return row, nil
		}
	}
	return db.PdbDatabase{}, pgx.ErrNoRows
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

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
