-- +goose Up
CREATE TABLE pg_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    image TEXT NOT NULL DEFAULT 'postgres:16-alpine',
    container_port INT NOT NULL DEFAULT 5432,
    host_port INT,
    docker_network_host BOOLEAN NOT NULL DEFAULT false,
    admin_user TEXT NOT NULL DEFAULT 'postgres',
    encrypted_admin_password BYTEA NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pg_databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pg_instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    owner_role TEXT NOT NULL DEFAULT 'postgres',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (instance_id, name)
);

CREATE TABLE pg_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pg_instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (instance_id, name)
);

CREATE TABLE pg_role_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES pg_roles(id) ON DELETE CASCADE,
    database_id UUID NOT NULL REFERENCES pg_databases(id) ON DELETE CASCADE,
    is_owner BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (role_id, database_id)
);

CREATE TABLE pg_backup_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pg_instances(id) ON DELETE CASCADE,
    database_id UUID REFERENCES pg_databases(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    hour INT NOT NULL DEFAULT 3 CHECK (hour >= 0 AND hour <= 23),
    minute INT NOT NULL DEFAULT 0 CHECK (minute >= 0 AND minute <= 59),
    timezone TEXT NOT NULL DEFAULT 'UTC',
    s3_endpoint TEXT NOT NULL DEFAULT '',
    s3_region TEXT NOT NULL DEFAULT 'us-east-1',
    s3_bucket TEXT NOT NULL,
    s3_prefix TEXT NOT NULL DEFAULT 'dock-pilot/pg-backups',
    encrypted_s3_access_key BYTEA NOT NULL,
    encrypted_s3_secret_key BYTEA NOT NULL,
    s3_force_path_style BOOLEAN NOT NULL DEFAULT false,
    retention_count INT NOT NULL DEFAULT 7 CHECK (retention_count >= 1 AND retention_count <= 365),
    last_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pg_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pg_instances(id) ON DELETE CASCADE,
    database_id UUID REFERENCES pg_databases(id) ON DELETE SET NULL,
    schedule_id UUID REFERENCES pg_backup_schedules(id) ON DELETE SET NULL,
    database_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    s3_endpoint TEXT NOT NULL DEFAULT '',
    s3_region TEXT NOT NULL DEFAULT 'us-east-1',
    s3_bucket TEXT NOT NULL DEFAULT '',
    s3_key TEXT NOT NULL DEFAULT '',
    s3_force_path_style BOOLEAN NOT NULL DEFAULT false,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_pg_databases_instance ON pg_databases(instance_id);
CREATE INDEX idx_pg_roles_instance ON pg_roles(instance_id);
CREATE INDEX idx_pg_backups_instance ON pg_backups(instance_id, created_at DESC);
CREATE INDEX idx_pg_backup_schedules_instance ON pg_backup_schedules(instance_id);

-- +goose Down
DROP TABLE IF EXISTS pg_backups;
DROP TABLE IF EXISTS pg_backup_schedules;
DROP TABLE IF EXISTS pg_role_grants;
DROP TABLE IF EXISTS pg_roles;
DROP TABLE IF EXISTS pg_databases;
DROP TABLE IF EXISTS pg_instances;
