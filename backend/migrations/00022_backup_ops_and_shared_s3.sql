-- +goose Up
ALTER TABLE panel_backup_settings
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

ALTER TABLE pdb_backup_schedules
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS use_panel_s3 BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE pdb_backup_schedules
  ALTER COLUMN encrypted_s3_access_key DROP NOT NULL,
  ALTER COLUMN encrypted_s3_secret_key DROP NOT NULL;

-- Normalize legacy rows that stored error text in last_status.
UPDATE panel_backup_settings
SET last_error = CASE
      WHEN last_status <> '' AND lower(last_status) NOT IN ('ok', 'succeeded', 'success') THEN left(last_status, 2000)
      ELSE last_error
    END,
    last_status = CASE
      WHEN last_status = '' THEN ''
      WHEN lower(last_status) IN ('ok', 'succeeded', 'success') THEN 'ok'
      ELSE 'failed'
    END;

UPDATE pdb_backup_schedules
SET last_error = CASE
      WHEN last_status <> '' AND lower(last_status) NOT IN ('ok', 'succeeded', 'success') THEN left(last_status, 2000)
      ELSE last_error
    END,
    last_status = CASE
      WHEN last_status = '' THEN ''
      WHEN lower(last_status) IN ('ok', 'succeeded', 'success') THEN 'ok'
      ELSE 'failed'
    END;

CREATE TABLE IF NOT EXISTS backup_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL
        CHECK (kind IN ('panel_snapshot', 'pg_backup', 'pg_restore', 'panel_restore')),
    status TEXT NOT NULL
        CHECK (status IN ('running', 'ok', 'failed')),
    database_name TEXT NOT NULL DEFAULT '',
    instance_id UUID,
    schedule_id UUID,
    s3_key TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_backup_operations_started
  ON backup_operations (started_at DESC);

-- +goose Down
DROP TABLE IF EXISTS backup_operations;

ALTER TABLE pdb_backup_schedules
  DROP COLUMN IF EXISTS use_panel_s3,
  DROP COLUMN IF EXISTS last_error;

-- Restore NOT NULL on keys for schedules that still have credentials.
UPDATE pdb_backup_schedules
SET encrypted_s3_access_key = '\x'::bytea
WHERE encrypted_s3_access_key IS NULL;

UPDATE pdb_backup_schedules
SET encrypted_s3_secret_key = '\x'::bytea
WHERE encrypted_s3_secret_key IS NULL;

ALTER TABLE pdb_backup_schedules
  ALTER COLUMN encrypted_s3_access_key SET NOT NULL,
  ALTER COLUMN encrypted_s3_secret_key SET NOT NULL;

ALTER TABLE panel_backup_settings
  DROP COLUMN IF EXISTS last_error;
