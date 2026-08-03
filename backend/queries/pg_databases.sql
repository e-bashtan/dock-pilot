-- name: CreatePgInstance :one
INSERT INTO pdb_instances (
    name, slug, image, container_port, host_port, docker_network_host,
    admin_user, encrypted_admin_password, status, message
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetPgInstance :one
SELECT * FROM pdb_instances WHERE id = $1;

-- name: GetPgInstanceBySlug :one
SELECT * FROM pdb_instances WHERE slug = $1;

-- name: ListPgInstances :many
SELECT * FROM pdb_instances ORDER BY created_at DESC;

-- name: UpdatePgInstance :one
UPDATE pdb_instances SET
    name = COALESCE(sqlc.narg('name'), name),
    image = COALESCE(sqlc.narg('image'), image),
    container_port = COALESCE(sqlc.narg('container_port'), container_port),
    host_port = COALESCE(sqlc.narg('host_port'), host_port),
    docker_network_host = COALESCE(sqlc.narg('docker_network_host'), docker_network_host),
    status = COALESCE(sqlc.narg('status'), status),
    message = COALESCE(sqlc.narg('message'), message),
    encrypted_admin_password = COALESCE(sqlc.narg('encrypted_admin_password'), encrypted_admin_password),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdatePgInstanceHostPort :one
UPDATE pdb_instances SET host_port = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdatePgInstanceStatus :one
UPDATE pdb_instances SET status = $2, message = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeletePgInstance :exec
DELETE FROM pdb_instances WHERE id = $1;

-- name: CreatePgDatabase :one
INSERT INTO pdb_databases (instance_id, name, owner_role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPgDatabase :one
SELECT * FROM pdb_databases WHERE id = $1;

-- name: ListPgDatabases :many
SELECT * FROM pdb_databases WHERE instance_id = $1 ORDER BY name ASC;

-- name: DeletePgDatabase :exec
DELETE FROM pdb_databases WHERE id = $1;

-- name: CreatePgRole :one
INSERT INTO pdb_roles (instance_id, name, encrypted_password)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPgRole :one
SELECT * FROM pdb_roles WHERE id = $1;

-- name: ListPgRoles :many
SELECT * FROM pdb_roles WHERE instance_id = $1 ORDER BY name ASC;

-- name: DeletePgRole :exec
DELETE FROM pdb_roles WHERE id = $1;

-- name: UpsertPgRoleGrant :one
INSERT INTO pdb_role_grants (role_id, database_id, is_owner)
VALUES ($1, $2, $3)
ON CONFLICT (role_id, database_id) DO UPDATE SET is_owner = EXCLUDED.is_owner
RETURNING *;

-- name: ListPgRoleGrantsByRole :many
SELECT * FROM pdb_role_grants WHERE role_id = $1;

-- name: ListPgRoleGrantsByDatabase :many
SELECT * FROM pdb_role_grants WHERE database_id = $1;

-- name: DeletePgRoleGrant :exec
DELETE FROM pdb_role_grants WHERE role_id = $1 AND database_id = $2;

-- name: CreatePgBackupSchedule :one
INSERT INTO pdb_backup_schedules (
    instance_id, database_id, enabled, hour, minute, timezone,
    s3_endpoint, s3_region, s3_bucket, s3_prefix,
    encrypted_s3_access_key, encrypted_s3_secret_key, s3_force_path_style,
    use_panel_s3, retention_count
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
) RETURNING *;

-- name: GetPgBackupSchedule :one
SELECT * FROM pdb_backup_schedules WHERE id = $1;

-- name: ListPgBackupSchedules :many
SELECT * FROM pdb_backup_schedules WHERE instance_id = $1 ORDER BY created_at DESC;

-- name: ListEnabledPgBackupSchedules :many
SELECT * FROM pdb_backup_schedules WHERE enabled = true ORDER BY created_at ASC;

-- name: UpdatePgBackupSchedule :one
UPDATE pdb_backup_schedules SET
    database_id = CASE WHEN sqlc.narg('clear_database_id')::boolean = true THEN NULL
                       WHEN sqlc.narg('database_id')::uuid IS NOT NULL THEN sqlc.narg('database_id')::uuid
                       ELSE database_id END,
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    hour = COALESCE(sqlc.narg('hour'), hour),
    minute = COALESCE(sqlc.narg('minute'), minute),
    timezone = COALESCE(sqlc.narg('timezone'), timezone),
    s3_endpoint = COALESCE(sqlc.narg('s3_endpoint'), s3_endpoint),
    s3_region = COALESCE(sqlc.narg('s3_region'), s3_region),
    s3_bucket = COALESCE(sqlc.narg('s3_bucket'), s3_bucket),
    s3_prefix = COALESCE(sqlc.narg('s3_prefix'), s3_prefix),
    encrypted_s3_access_key = COALESCE(sqlc.narg('encrypted_s3_access_key'), encrypted_s3_access_key),
    encrypted_s3_secret_key = COALESCE(sqlc.narg('encrypted_s3_secret_key'), encrypted_s3_secret_key),
    s3_force_path_style = COALESCE(sqlc.narg('s3_force_path_style'), s3_force_path_style),
    use_panel_s3 = COALESCE(sqlc.narg('use_panel_s3'), use_panel_s3),
    retention_count = COALESCE(sqlc.narg('retention_count'), retention_count),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdatePgBackupScheduleRun :one
UPDATE pdb_backup_schedules SET
    last_run_at = $2,
    last_status = $3,
    last_error = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeletePgBackupSchedule :exec
DELETE FROM pdb_backup_schedules WHERE id = $1;
