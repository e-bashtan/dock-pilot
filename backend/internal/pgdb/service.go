package pgdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/docker"
	"github.com/ebash/dock-pilot/backend/internal/secrets"
)

type Service struct {
	queries *db.Queries
	docker  docker.Client
	cipher  *secrets.Cipher
	logger  *slog.Logger
}

func NewService(queries *db.Queries, dockerClient docker.Client, cipher *secrets.Cipher, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{queries: queries, docker: dockerClient, cipher: cipher, logger: logger}
}

func (s *Service) ListInstances(ctx context.Context) ([]InstanceResponse, error) {
	rows, err := s.queries.ListPgInstances(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InstanceResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInstanceResponse(row))
	}
	return out, nil
}

func (s *Service) GetInstance(ctx context.Context, id uuid.UUID) (InstanceResponse, error) {
	row, err := s.queries.GetPgInstance(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InstanceResponse{}, ErrNotFound
		}
		return InstanceResponse{}, err
	}
	return toInstanceResponse(row), nil
}

func (s *Service) CreateInstance(ctx context.Context, req CreateInstanceRequest) (InstanceResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return InstanceResponse{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = slugify(name)
	}
	if err := validateSlug(slug); err != nil {
		return InstanceResponse{}, err
	}
	image := strings.TrimSpace(req.Image)
	if image == "" {
		image = "postgres:16-alpine"
	}
	adminUser := strings.TrimSpace(req.AdminUser)
	if adminUser == "" {
		adminUser = "postgres"
	}
	if err := validateRoleName(adminUser); err != nil && adminUser != "postgres" {
		return InstanceResponse{}, err
	}
	password := strings.TrimSpace(req.AdminPassword)
	if password == "" {
		var err error
		password, err = generatePassword(24)
		if err != nil {
			return InstanceResponse{}, err
		}
	}
	enc, err := s.cipher.Encrypt(password)
	if err != nil {
		return InstanceResponse{}, err
	}
	port := req.ContainerPort
	if port == 0 {
		port = 5432
	}

	var hostPort pgtype.Int4
	if !req.DockerNetworkHost {
		if req.HostPort != nil && *req.HostPort > 0 {
			hostPort = pgtype.Int4{Int32: int32(*req.HostPort), Valid: true}
		}
	}

	row, err := s.queries.CreatePgInstance(ctx, db.CreatePgInstanceParams{
		Name:                   name,
		Slug:                   slug,
		Image:                  image,
		ContainerPort:          int32(port),
		HostPort:               hostPort,
		DockerNetworkHost:      req.DockerNetworkHost,
		AdminUser:              adminUser,
		EncryptedAdminPassword: enc,
		Status:                 "draft",
		Message:                "",
	})
	if err != nil {
		if isUniqueViolation(err) {
			return InstanceResponse{}, ErrSlugConflict
		}
		return InstanceResponse{}, err
	}
	return toInstanceResponse(row), nil
}

func (s *Service) DeployInstance(ctx context.Context, id uuid.UUID) (InstanceResponse, error) {
	inst, err := s.queries.GetPgInstance(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InstanceResponse{}, ErrNotFound
		}
		return InstanceResponse{}, err
	}

	_, _ = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
		ID: id, Status: "deploying", Message: "pulling image",
	})

	if err := s.docker.Pull(ctx, inst.Image); err != nil {
		_, _ = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
			ID: id, Status: "error", Message: err.Error(),
		})
		return InstanceResponse{}, err
	}

	password, err := s.adminPassword(inst)
	if err != nil {
		return InstanceResponse{}, err
	}

	hostPort := 0
	publish := !inst.DockerNetworkHost
	if publish {
		if inst.HostPort.Valid && inst.HostPort.Int32 > 0 {
			hostPort = int(inst.HostPort.Int32)
		} else {
			hostPort, err = s.docker.AllocatePort(ctx)
			if err != nil {
				_, _ = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
					ID: id, Status: "error", Message: err.Error(),
				})
				return InstanceResponse{}, err
			}
			inst, err = s.queries.UpdatePgInstanceHostPort(ctx, db.UpdatePgInstanceHostPortParams{
				ID:       id,
				HostPort: pgtype.Int4{Int32: int32(hostPort), Valid: true},
			})
			if err != nil {
				return InstanceResponse{}, err
			}
		}
	}

	vol := s.volumeName(inst)
	cname := s.containerName(inst)
	_, _ = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
		ID: id, Status: "deploying", Message: "starting container",
	})

	_, err = s.docker.Run(ctx, docker.RunOptions{
		ImageTag:      inst.Image,
		ContainerName: cname,
		StopNames:     []string{cname},
		HostPort:      hostPort,
		ContainerPort: int(inst.ContainerPort),
		PublishPorts:  publish,
		NetworkHost:   inst.DockerNetworkHost,
		Env: map[string]string{
			"POSTGRES_USER":     inst.AdminUser,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       "postgres",
		},
		Mounts: []docker.Mount{{
			Source: vol,
			Target: "/var/lib/postgresql/data",
			Type:   "volume",
		}},
		EnsureVolumes: []string{vol},
	})
	if err != nil {
		_, _ = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
			ID: id, Status: "error", Message: err.Error(),
		})
		return InstanceResponse{}, err
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if err := s.waitReady(ctx, inst); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err := s.waitReady(ctx, inst); err != nil {
		_, _ = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
			ID: id, Status: "error", Message: "container started but postgres not ready: " + err.Error(),
		})
		return InstanceResponse{}, err
	}

	inst, err = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
		ID: id, Status: "running", Message: "",
	})
	if err != nil {
		return InstanceResponse{}, err
	}
	return toInstanceResponse(inst), nil
}

func (s *Service) StopInstance(ctx context.Context, id uuid.UUID) (InstanceResponse, error) {
	inst, err := s.queries.GetPgInstance(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InstanceResponse{}, ErrNotFound
		}
		return InstanceResponse{}, err
	}
	if err := s.docker.Stop(ctx, s.containerName(inst)); err != nil {
		return InstanceResponse{}, err
	}
	inst, err = s.queries.UpdatePgInstanceStatus(ctx, db.UpdatePgInstanceStatusParams{
		ID: id, Status: "stopped", Message: "",
	})
	if err != nil {
		return InstanceResponse{}, err
	}
	return toInstanceResponse(inst), nil
}

func (s *Service) DeleteInstance(ctx context.Context, id uuid.UUID) error {
	inst, err := s.queries.GetPgInstance(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = s.docker.Stop(ctx, s.containerName(inst))
	return s.queries.DeletePgInstance(ctx, id)
}

func (s *Service) ListDatabases(ctx context.Context, instanceID uuid.UUID) ([]DatabaseResponse, error) {
	if _, err := s.requireInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListPgDatabases(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	out := make([]DatabaseResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDatabaseResponse(row))
	}
	return out, nil
}

func (s *Service) CreateDatabase(ctx context.Context, instanceID uuid.UUID, req CreateDatabaseRequest) (DatabaseResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return DatabaseResponse{}, err
	}
	if err := validateDBName(req.Name); err != nil {
		return DatabaseResponse{}, err
	}
	owner := strings.TrimSpace(req.OwnerRole)
	if owner == "" {
		owner = inst.AdminUser
	}
	if err := validateRoleName(owner); err != nil && owner != inst.AdminUser {
		return DatabaseResponse{}, err
	}
	dbIdent, err := quoteIdent(req.Name)
	if err != nil {
		return DatabaseResponse{}, err
	}
	ownerIdent, err := quoteIdent(owner)
	if err != nil {
		return DatabaseResponse{}, err
	}
	sql := fmt.Sprintf("CREATE DATABASE %s OWNER %s", dbIdent, ownerIdent)
	if err := s.execSQL(ctx, inst, "postgres", sql); err != nil {
		return DatabaseResponse{}, fmt.Errorf("create database in postgres: %w", err)
	}
	row, err := s.queries.CreatePgDatabase(ctx, db.CreatePgDatabaseParams{
		InstanceID: instanceID,
		Name:       strings.TrimSpace(req.Name),
		OwnerRole:  owner,
	})
	if err != nil {
		return DatabaseResponse{}, err
	}
	return toDatabaseResponse(row), nil
}

func (s *Service) DeleteDatabase(ctx context.Context, instanceID, databaseID uuid.UUID) error {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	row, err := s.queries.GetPgDatabase(ctx, databaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if row.InstanceID != instanceID {
		return ErrNotFound
	}
	dbIdent, err := quoteIdent(row.Name)
	if err != nil {
		return err
	}
	_ = s.execSQL(ctx, inst, "postgres", fmt.Sprintf(
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid()",
		quoteLiteral(row.Name),
	))
	if err := s.execSQL(ctx, inst, "postgres", "DROP DATABASE IF EXISTS "+dbIdent); err != nil {
		return fmt.Errorf("drop database in postgres: %w", err)
	}
	return s.queries.DeletePgDatabase(ctx, databaseID)
}

func (s *Service) ListRoles(ctx context.Context, instanceID uuid.UUID) ([]RoleResponse, error) {
	if _, err := s.requireInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	roles, err := s.queries.ListPgRoles(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	dbs, err := s.queries.ListPgDatabases(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	dbNames := map[uuid.UUID]string{}
	for _, d := range dbs {
		dbNames[d.ID] = d.Name
	}
	out := make([]RoleResponse, 0, len(roles))
	for _, role := range roles {
		grants, err := s.queries.ListPgRoleGrantsByRole(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		gOut := make([]GrantResponse, 0, len(grants))
		for _, g := range grants {
			gOut = append(gOut, GrantResponse{
				ID:           g.ID,
				RoleID:       g.RoleID,
				DatabaseID:   g.DatabaseID,
				DatabaseName: dbNames[g.DatabaseID],
				IsOwner:      g.IsOwner,
			})
		}
		out = append(out, RoleResponse{
			ID:         role.ID,
			InstanceID: role.InstanceID,
			Name:       role.Name,
			CreatedAt:  role.CreatedAt,
			Grants:     gOut,
		})
	}
	return out, nil
}

func (s *Service) CreateRole(ctx context.Context, instanceID uuid.UUID, req CreateRoleRequest) (RoleResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return RoleResponse{}, err
	}
	if err := validateRoleName(req.Name); err != nil {
		return RoleResponse{}, err
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		password, err = generatePassword(24)
		if err != nil {
			return RoleResponse{}, err
		}
	}
	roleIdent, err := quoteIdent(req.Name)
	if err != nil {
		return RoleResponse{}, err
	}
	sql := fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD %s", roleIdent, quoteLiteral(password))
	if err := s.execSQL(ctx, inst, "postgres", sql); err != nil {
		return RoleResponse{}, fmt.Errorf("create role in postgres: %w", err)
	}
	enc, err := s.cipher.Encrypt(password)
	if err != nil {
		return RoleResponse{}, err
	}
	row, err := s.queries.CreatePgRole(ctx, db.CreatePgRoleParams{
		InstanceID:        instanceID,
		Name:              strings.TrimSpace(req.Name),
		EncryptedPassword: enc,
	})
	if err != nil {
		return RoleResponse{}, err
	}
	return RoleResponse{
		ID:         row.ID,
		InstanceID: row.InstanceID,
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		Grants:     []GrantResponse{},
	}, nil
}

func (s *Service) DeleteRole(ctx context.Context, instanceID, roleID uuid.UUID) error {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	role, err := s.queries.GetPgRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if role.InstanceID != instanceID {
		return ErrNotFound
	}
	roleIdent, err := quoteIdent(role.Name)
	if err != nil {
		return err
	}
	_ = s.execSQL(ctx, inst, "postgres", "DROP OWNED BY "+roleIdent+" CASCADE")
	if err := s.execSQL(ctx, inst, "postgres", "DROP ROLE IF EXISTS "+roleIdent); err != nil {
		return fmt.Errorf("drop role in postgres: %w", err)
	}
	return s.queries.DeletePgRole(ctx, roleID)
}

func (s *Service) GrantRole(ctx context.Context, instanceID, roleID uuid.UUID, req GrantRequest) (GrantResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return GrantResponse{}, err
	}
	role, err := s.queries.GetPgRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GrantResponse{}, ErrNotFound
		}
		return GrantResponse{}, err
	}
	if role.InstanceID != instanceID {
		return GrantResponse{}, ErrNotFound
	}
	database, err := s.queries.GetPgDatabase(ctx, req.DatabaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GrantResponse{}, ErrNotFound
		}
		return GrantResponse{}, err
	}
	if database.InstanceID != instanceID {
		return GrantResponse{}, ErrNotFound
	}
	roleIdent, err := quoteIdent(role.Name)
	if err != nil {
		return GrantResponse{}, err
	}
	dbIdent, err := quoteIdent(database.Name)
	if err != nil {
		return GrantResponse{}, err
	}
	if req.IsOwner {
		if err := s.execSQL(ctx, inst, "postgres", fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", dbIdent, roleIdent)); err != nil {
			return GrantResponse{}, err
		}
	} else {
		if err := s.execSQL(ctx, inst, "postgres", fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", dbIdent, roleIdent)); err != nil {
			return GrantResponse{}, err
		}
		grants := []string{
			fmt.Sprintf("GRANT USAGE, CREATE ON SCHEMA public TO %s", roleIdent),
			fmt.Sprintf("GRANT ALL ON ALL TABLES IN SCHEMA public TO %s", roleIdent),
			fmt.Sprintf("GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO %s", roleIdent),
			fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s", roleIdent),
			fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s", roleIdent),
		}
		for _, sql := range grants {
			if err := s.execSQL(ctx, inst, database.Name, sql); err != nil {
				return GrantResponse{}, err
			}
		}
	}
	grant, err := s.queries.UpsertPgRoleGrant(ctx, db.UpsertPgRoleGrantParams{
		RoleID:     roleID,
		DatabaseID: req.DatabaseID,
		IsOwner:    req.IsOwner,
	})
	if err != nil {
		return GrantResponse{}, err
	}
	return GrantResponse{
		ID:           grant.ID,
		RoleID:       grant.RoleID,
		DatabaseID:   grant.DatabaseID,
		DatabaseName: database.Name,
		IsOwner:      grant.IsOwner,
	}, nil
}

func (s *Service) ConnectionInfo(ctx context.Context, instanceID, databaseID, roleID uuid.UUID) (ConnectionInfoResponse, error) {
	inst, err := s.requireInstance(ctx, instanceID)
	if err != nil {
		return ConnectionInfoResponse{}, err
	}
	database, err := s.queries.GetPgDatabase(ctx, databaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConnectionInfoResponse{}, ErrNotFound
		}
		return ConnectionInfoResponse{}, err
	}
	if database.InstanceID != instanceID {
		return ConnectionInfoResponse{}, ErrNotFound
	}
	role, err := s.queries.GetPgRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConnectionInfoResponse{}, ErrNotFound
		}
		return ConnectionInfoResponse{}, err
	}
	if role.InstanceID != instanceID {
		return ConnectionInfoResponse{}, ErrNotFound
	}
	password, err := s.cipher.Decrypt(role.EncryptedPassword)
	if err != nil {
		return ConnectionInfoResponse{}, err
	}
	port := int(inst.ContainerPort)
	if !inst.DockerNetworkHost && inst.HostPort.Valid {
		port = int(inst.HostPort.Int32)
	}
	host := "127.0.0.1"
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", role.Name, password, host, port, database.Name)
	return ConnectionInfoResponse{
		Host:     host,
		Port:     port,
		Database: database.Name,
		User:     role.Name,
		URL:      url,
	}, nil
}

func (s *Service) requireInstance(ctx context.Context, id uuid.UUID) (db.PgInstance, error) {
	inst, err := s.queries.GetPgInstance(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PgInstance{}, ErrNotFound
		}
		return db.PgInstance{}, err
	}
	return inst, nil
}

func validateSlug(slug string) error {
	if slug == "" || len(slug) > 48 {
		return fmt.Errorf("%w: invalid slug", ErrInvalidInput)
	}
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("%w: invalid slug", ErrInvalidInput)
	}
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return fmt.Errorf("%w: invalid slug", ErrInvalidInput)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func toInstanceResponse(row db.PgInstance) InstanceResponse {
	var hostPort *int
	if row.HostPort.Valid {
		v := int(row.HostPort.Int32)
		hostPort = &v
	}
	return InstanceResponse{
		ID:                row.ID,
		Name:              row.Name,
		Slug:              row.Slug,
		Image:             row.Image,
		ContainerPort:     int(row.ContainerPort),
		HostPort:          hostPort,
		DockerNetworkHost: row.DockerNetworkHost,
		AdminUser:         row.AdminUser,
		Status:            row.Status,
		Message:           row.Message,
		ContainerName:     docker.SanitizeContainerName("dockpilot-pg-" + row.Slug),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func toDatabaseResponse(row db.PgDatabase) DatabaseResponse {
	return DatabaseResponse{
		ID:         row.ID,
		InstanceID: row.InstanceID,
		Name:       row.Name,
		OwnerRole:  row.OwnerRole,
		CreatedAt:  row.CreatedAt,
	}
}

func optionalUUID(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func optionalTime(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
