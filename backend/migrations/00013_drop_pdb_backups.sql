-- +goose Up
DROP TABLE IF EXISTS pdb_backups;

-- +goose Down
CREATE TABLE pdb_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pdb_instances(id) ON DELETE CASCADE,
    database_id UUID REFERENCES pdb_databases(id) ON DELETE SET NULL,
    schedule_id UUID REFERENCES pdb_backup_schedules(id) ON DELETE SET NULL,
    database_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    s3_endpoint TEXT NOT NULL DEFAULT '',
    s3_region TEXT NOT NULL DEFAULT 'us-east-1',
    s3_bucket TEXT NOT NULL DEFAULT '',
    s3_key TEXT NOT NULL DEFAULT '',
    s3_force_path_style BOOLEAN NOT NULL DEFAULT FALSE,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_pdb_backups_instance ON pdb_backups(instance_id, created_at DESC);
