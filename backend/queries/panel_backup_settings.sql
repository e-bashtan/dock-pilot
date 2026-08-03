-- name: EnsurePanelBackupSettings :one
INSERT INTO panel_backup_settings (id) VALUES (1)
ON CONFLICT (id) DO UPDATE SET updated_at = panel_backup_settings.updated_at
RETURNING *;

-- name: GetPanelBackupSettings :one
SELECT * FROM panel_backup_settings WHERE id = 1;

-- name: UpdatePanelBackupSettings :one
UPDATE panel_backup_settings SET
    enabled = $1,
    hour = $2,
    minute = $3,
    timezone = $4,
    s3_endpoint = $5,
    s3_region = $6,
    s3_bucket = $7,
    s3_prefix = $8,
    s3_force_path_style = $9,
    retention_count = $10,
    updated_at = now()
WHERE id = 1
RETURNING *;

-- name: UpdatePanelBackupS3Keys :one
UPDATE panel_backup_settings SET
    encrypted_s3_access_key = $1,
    encrypted_s3_secret_key = $2,
    updated_at = now()
WHERE id = 1
RETURNING *;

-- name: ClearPanelBackupS3Keys :one
UPDATE panel_backup_settings SET
    encrypted_s3_access_key = NULL,
    encrypted_s3_secret_key = NULL,
    updated_at = now()
WHERE id = 1
RETURNING *;

-- name: UpdatePanelBackupRun :one
UPDATE panel_backup_settings SET
    last_run_at = $1,
    last_status = $2,
    last_error = $3,
    updated_at = now()
WHERE id = 1
RETURNING *;
