-- +goose Up
CREATE TABLE panel_backup_settings (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT false,
    hour INT NOT NULL DEFAULT 3 CHECK (hour >= 0 AND hour <= 23),
    minute INT NOT NULL DEFAULT 0 CHECK (minute >= 0 AND minute <= 59),
    timezone TEXT NOT NULL DEFAULT 'UTC',
    s3_endpoint TEXT NOT NULL DEFAULT '',
    s3_region TEXT NOT NULL DEFAULT 'ru-central1',
    s3_bucket TEXT NOT NULL DEFAULT '',
    s3_prefix TEXT NOT NULL DEFAULT 'dock-pilot/backups',
    encrypted_s3_access_key BYTEA,
    encrypted_s3_secret_key BYTEA,
    s3_force_path_style BOOLEAN NOT NULL DEFAULT false,
    retention_count INT NOT NULL DEFAULT 7 CHECK (retention_count >= 1 AND retention_count <= 365),
    last_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO panel_backup_settings (id) VALUES (1);

-- +goose Down
DROP TABLE IF EXISTS panel_backup_settings;
