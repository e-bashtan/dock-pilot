CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    primary_url TEXT NOT NULL,
    git_repo_url TEXT NOT NULL DEFAULT '',
    git_branch TEXT NOT NULL DEFAULT 'main',
    dockerfile_path TEXT NOT NULL DEFAULT 'Dockerfile',
    build_context TEXT NOT NULL DEFAULT '.',
    container_port INT NOT NULL DEFAULT 3000,
    host_port INT,
    nginx_ssl_enabled BOOLEAN NOT NULL DEFAULT true,
    nginx_force_https BOOLEAN NOT NULL DEFAULT true,
    site_type TEXT NOT NULL DEFAULT 'web',
    docker_volume_mounts TEXT NOT NULL DEFAULT '',
    docker_named_volumes TEXT NOT NULL DEFAULT '',
    docker_network_host BOOLEAN NOT NULL DEFAULT false,
    health_check_path TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE site_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (site_id, domain)
);

CREATE TABLE site_env_vars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (site_id, key),
    CONSTRAINT site_env_vars_key_not_empty CHECK (length(trim(key)) > 0)
);

CREATE TABLE site_secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (site_id, key),
    CONSTRAINT site_secrets_key_not_empty CHECK (length(trim(key)) > 0)
);

CREATE TABLE deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notification_settings (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT false,
    telegram_chat_id TEXT NOT NULL DEFAULT '',
    telegram_http_proxy TEXT NOT NULL DEFAULT '',
    daily_digest_enabled BOOLEAN NOT NULL DEFAULT false,
    daily_digest_hour INT NOT NULL DEFAULT 9 CHECK (daily_digest_hour >= 0 AND daily_digest_hour <= 23),
    daily_digest_timezone TEXT NOT NULL DEFAULT 'UTC',
    alert_on_incident_enabled BOOLEAN NOT NULL DEFAULT true,
    encrypted_telegram_bot_token BYTEA,
    last_daily_sent_at TIMESTAMPTZ,
    last_overall_by_site JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pdb_instances (
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

CREATE TABLE pdb_databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pdb_instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    owner_role TEXT NOT NULL DEFAULT 'postgres',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (instance_id, name)
);

CREATE TABLE pdb_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pdb_instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (instance_id, name)
);

CREATE TABLE pdb_role_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES pdb_roles(id) ON DELETE CASCADE,
    database_id UUID NOT NULL REFERENCES pdb_databases(id) ON DELETE CASCADE,
    is_owner BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (role_id, database_id)
);

CREATE TABLE pdb_backup_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pdb_instances(id) ON DELETE CASCADE,
    database_id UUID REFERENCES pdb_databases(id) ON DELETE CASCADE,
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

CREATE TABLE pdb_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES pdb_instances(id) ON DELETE CASCADE,
    database_id UUID REFERENCES pdb_databases(id) ON DELETE SET NULL,
    schedule_id UUID REFERENCES pdb_backup_schedules(id) ON DELETE SET NULL,
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

CREATE INDEX idx_pdb_databases_instance ON pdb_databases(instance_id);
CREATE INDEX idx_pdb_roles_instance ON pdb_roles(instance_id);
CREATE INDEX idx_pdb_backups_instance ON pdb_backups(instance_id, created_at DESC);
CREATE INDEX idx_pdb_backup_schedules_instance ON pdb_backup_schedules(instance_id);
