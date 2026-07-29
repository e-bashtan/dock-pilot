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

CREATE INDEX idx_pdb_databases_instance ON pdb_databases(instance_id);
CREATE INDEX idx_pdb_roles_instance ON pdb_roles(instance_id);
CREATE INDEX idx_pdb_backup_schedules_instance ON pdb_backup_schedules(instance_id);

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

CREATE TABLE billing_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL DEFAULT 'planetahost'
        CHECK (provider IN ('planetahost')),
    server_ip TEXT NOT NULL,
    login TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    billmgr_url TEXT NOT NULL DEFAULT 'https://bill.planetahost.ru/billmgr',
    alert_days INT NOT NULL DEFAULT 10 CHECK (alert_days >= 1 AND alert_days <= 90),
    enabled BOOLEAN NOT NULL DEFAULT true,
    cached_expire_date DATE,
    cached_status TEXT NOT NULL DEFAULT '',
    cached_name TEXT NOT NULL DEFAULT '',
    cached_cost TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    last_check_error TEXT NOT NULL DEFAULT '',
    last_alert_expire_date DATE,
    last_alert_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, server_ip)
);

CREATE INDEX idx_billing_accounts_enabled ON billing_accounts(enabled);


CREATE TABLE fleet_settings (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    mode TEXT NOT NULL DEFAULT 'standalone'
        CHECK (mode IN ('standalone', 'master', 'managed_node')),
    node_uid UUID NOT NULL DEFAULT gen_random_uuid(),
    node_name TEXT NOT NULL DEFAULT '',
    public_url TEXT NOT NULL DEFAULT '',
    master_url TEXT NOT NULL DEFAULT '',
    notification_mode TEXT NOT NULL DEFAULT 'local'
        CHECK (notification_mode IN ('local', 'master', 'disabled')),
    encrypted_master_token BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE fleet_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_uid UUID NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'node'
        CHECK (role IN ('master', 'node', 'agent')),
    connection_type TEXT NOT NULL
        CHECK (connection_type IN ('local', 'dockpilot', 'agent')),
    base_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'online'
        CHECK (status IN ('online', 'warning', 'offline')),
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    version TEXT NOT NULL DEFAULT '',
    agent_version TEXT NOT NULL DEFAULT '',
    last_seen_at TIMESTAMPTZ,
    last_heartbeat_at TIMESTAMPTZ,
    last_poll_at TIMESTAMPTZ,
    paired_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_fleet_nodes_uid_active ON fleet_nodes (node_uid) WHERE deleted_at IS NULL;
CREATE INDEX idx_fleet_nodes_status ON fleet_nodes (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_fleet_nodes_connection ON fleet_nodes (connection_type) WHERE deleted_at IS NULL;

CREATE TABLE fleet_node_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES fleet_nodes(id) ON DELETE CASCADE,
    direction TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    purpose TEXT NOT NULL DEFAULT '',
    scopes TEXT[] NOT NULL DEFAULT '{}',
    token_hash BYTEA,
    encrypted_token BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_fleet_node_credentials_node ON fleet_node_credentials (node_id);
CREATE INDEX idx_fleet_node_credentials_hash ON fleet_node_credentials (token_hash) WHERE revoked_at IS NULL AND token_hash IS NOT NULL;

CREATE TABLE fleet_pairing_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fleet_pairing_codes_hash ON fleet_pairing_codes (code_hash) WHERE used_at IS NULL;

CREATE TABLE fleet_registration_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    installation_id UUID,
    expected_node_uid UUID NOT NULL,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fleet_registration_tokens_hash ON fleet_registration_tokens (token_hash) WHERE used_at IS NULL;

CREATE TABLE fleet_snapshots (
    id BIGSERIAL PRIMARY KEY,
    node_id UUID NOT NULL REFERENCES fleet_nodes(id) ON DELETE CASCADE,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cpu_percent DOUBLE PRECISION,
    memory_used_bytes BIGINT,
    memory_total_bytes BIGINT,
    disk_used_percent DOUBLE PRECISION,
    uptime_seconds BIGINT,
    apps_total INT,
    apps_running INT,
    apps_unhealthy INT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_fleet_snapshots_node_collected ON fleet_snapshots (node_id, collected_at DESC);

CREATE TABLE fleet_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL UNIQUE,
    node_id UUID REFERENCES fleet_nodes(id) ON DELETE SET NULL,
    node_uid UUID,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info'
        CHECK (severity IN ('info', 'warning', 'critical')),
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fleet_events_node_occurred ON fleet_events (node_id, occurred_at DESC);
CREATE INDEX idx_fleet_events_type_occurred ON fleet_events (event_type, occurred_at DESC);
CREATE INDEX idx_fleet_events_severity_occurred ON fleet_events (severity, occurred_at DESC);

CREATE TABLE fleet_incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID REFERENCES fleet_nodes(id) ON DELETE SET NULL,
    dedup_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    count INT NOT NULL DEFAULT 1,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    last_event_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_fleet_incidents_open_dedup ON fleet_incidents (dedup_key) WHERE status = 'open';
CREATE INDEX idx_fleet_incidents_node_status ON fleet_incidents (node_id, status);

CREATE TABLE fleet_outbox (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fleet_outbox_pending ON fleet_outbox (next_attempt_at) WHERE delivered_at IS NULL;

CREATE TABLE fleet_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID REFERENCES fleet_nodes(id) ON DELETE SET NULL,
    host TEXT NOT NULL,
    port INT NOT NULL DEFAULT 22,
    username TEXT NOT NULL DEFAULT 'root',
    status TEXT NOT NULL DEFAULT 'pending',
    current_step TEXT NOT NULL DEFAULT '',
    ssh_fingerprint TEXT NOT NULL DEFAULT '',
    expected_node_uid UUID NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_fleet_installations_status ON fleet_installations (status);

CREATE TABLE fleet_installation_logs (
    id BIGSERIAL PRIMARY KEY,
    installation_id UUID NOT NULL REFERENCES fleet_installations(id) ON DELETE CASCADE,
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fleet_installation_logs_inst ON fleet_installation_logs (installation_id, id);

CREATE TABLE fleet_node_billing (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL UNIQUE REFERENCES fleet_nodes(id) ON DELETE CASCADE,
    billing_account_id UUID REFERENCES billing_accounts(id) ON DELETE SET NULL,
    mode TEXT NOT NULL DEFAULT 'manual' CHECK (mode IN ('manual', 'planetahost', 'external')),
    provider_name TEXT NOT NULL DEFAULT '',
    provider_url TEXT NOT NULL DEFAULT '',
    external_service_id TEXT NOT NULL DEFAULT '',
    cost_minor BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'RUB',
    period TEXT NOT NULL DEFAULT 'monthly'
        CHECK (period IN ('monthly', 'quarterly', 'yearly', 'custom')),
    next_due_date DATE,
    auto_renew BOOLEAN NOT NULL DEFAULT false,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE fleet_monitored_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES fleet_nodes(id) ON DELETE CASCADE,
    unit_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (node_id, unit_name)
);

CREATE TABLE fleet_known_hosts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host TEXT NOT NULL,
    port INT NOT NULL DEFAULT 22,
    fingerprint TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (host, port)
);
