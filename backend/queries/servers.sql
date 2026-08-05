-- name: GetServersSettings :one
SELECT * FROM servers_settings WHERE id = 1;

-- name: UpdateServersSettings :one
UPDATE servers_settings SET
    mode = $2,
    node_name = $3,
    public_url = $4,
    master_url = $5,
    notification_mode = $6,
    encrypted_master_token = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: EnsureServersSettings :exec
INSERT INTO servers_settings (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

-- name: ListServersNodes :many
SELECT * FROM servers_nodes
WHERE deleted_at IS NULL
ORDER BY
    CASE status WHEN 'offline' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END,
    CASE role WHEN 'master' THEN 0 ELSE 1 END,
    name ASC;

-- name: GetServersNode :one
SELECT * FROM servers_nodes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetServersNodeByUID :one
SELECT * FROM servers_nodes WHERE node_uid = $1 AND deleted_at IS NULL;

-- name: GetLocalServersNode :one
SELECT * FROM servers_nodes WHERE connection_type = 'local' AND deleted_at IS NULL LIMIT 1;

-- name: CreateServersNode :one
INSERT INTO servers_nodes (
    node_uid, name, role, connection_type, base_url, status, capabilities,
    version, agent_version, last_seen_at, paired_at, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: UpdateServersNode :one
UPDATE servers_nodes SET
    name = $2,
    base_url = $3,
    metadata = $4,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteServersNode :exec
UPDATE servers_nodes SET deleted_at = now(), updated_at = now() WHERE id = $1;

-- name: CountActiveRemoteNodes :one
SELECT count(*)::int FROM servers_nodes
WHERE deleted_at IS NULL AND connection_type <> 'local';

-- name: UpdateServersNodeHeartbeat :one
UPDATE servers_nodes SET
    status = $2,
    last_seen_at = $3,
    last_heartbeat_at = $3,
    version = CASE WHEN sqlc.arg(version)::text = '' THEN version ELSE sqlc.arg(version)::text END,
    agent_version = CASE WHEN sqlc.arg(agent_version)::text = '' THEN agent_version ELSE sqlc.arg(agent_version)::text END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateServersNodePoll :one
UPDATE servers_nodes SET
    status = $2,
    last_seen_at = $3,
    last_poll_at = $3,
    version = CASE WHEN sqlc.arg(version)::text = '' THEN version ELSE sqlc.arg(version)::text END,
    capabilities = COALESCE(sqlc.narg(capabilities)::jsonb, capabilities),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateServersNodeStatus :exec
UPDATE servers_nodes SET status = $2, updated_at = now() WHERE id = $1;

-- name: ListBarnNodes :many
SELECT * FROM servers_nodes
WHERE deleted_at IS NULL AND connection_type = 'barn';

-- name: CreateServersCredential :one
INSERT INTO servers_node_credentials (
    node_id, direction, purpose, scopes, token_hash, encrypted_token
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInboundCredentialByHash :one
SELECT * FROM servers_node_credentials
WHERE token_hash = $1
  AND direction = 'inbound'
  AND revoked_at IS NULL
LIMIT 1;

-- name: GetOutboundCredential :one
SELECT * FROM servers_node_credentials
WHERE node_id = $1 AND direction = 'outbound' AND revoked_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: RevokeNodeCredentials :exec
UPDATE servers_node_credentials SET revoked_at = now()
WHERE node_id = $1 AND revoked_at IS NULL;

-- name: CreatePairingCode :one
INSERT INTO servers_pairing_codes (code_hash, expires_at) VALUES ($1, $2) RETURNING *;

-- name: GetValidPairingCodeByHash :one
SELECT * FROM servers_pairing_codes
WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
LIMIT 1;

-- name: MarkPairingCodeUsed :exec
UPDATE servers_pairing_codes SET used_at = now() WHERE id = $1 AND used_at IS NULL;

-- name: CreateRegistrationToken :one
INSERT INTO servers_registration_tokens (installation_id, expected_node_uid, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetValidRegistrationTokenByHash :one
SELECT * FROM servers_registration_tokens
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
LIMIT 1;

-- name: MarkRegistrationTokenUsed :exec
UPDATE servers_registration_tokens SET used_at = now() WHERE id = $1 AND used_at IS NULL;

-- name: InsertServersSnapshot :one
INSERT INTO servers_snapshots (
    node_id, collected_at, cpu_percent, memory_used_bytes, memory_total_bytes,
    disk_used_percent, uptime_seconds, apps_total, apps_running, apps_unhealthy, payload
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetLatestServersSnapshot :one
SELECT * FROM servers_snapshots
WHERE node_id = $1
ORDER BY collected_at DESC
LIMIT 1;

-- name: DeleteOldServersSnapshots :exec
DELETE FROM servers_snapshots WHERE collected_at < $1;

-- name: InsertServersEvent :one
INSERT INTO servers_events (
    event_id, node_id, node_uid, event_type, severity, resource_type, resource_id,
    title, message, payload, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (event_id) DO NOTHING
RETURNING *;

-- name: ListServersEvents :many
SELECT * FROM servers_events
ORDER BY occurred_at DESC
LIMIT $1 OFFSET $2;

-- name: ListServersEventsByNode :many
SELECT * FROM servers_events
WHERE node_id = $1
ORDER BY occurred_at DESC
LIMIT $2 OFFSET $3;

-- name: GetOpenIncidentByDedup :one
SELECT * FROM servers_incidents WHERE dedup_key = $1 AND status = 'open';

-- name: CreateServersIncident :one
INSERT INTO servers_incidents (
    node_id, dedup_key, event_type, resource_type, resource_id, title, last_event_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: TouchServersIncident :one
UPDATE servers_incidents SET
    count = count + 1,
    last_seen_at = now(),
    last_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ResolveServersIncident :one
UPDATE servers_incidents SET
    status = 'resolved',
    resolved_at = now(),
    last_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListOpenServersIncidents :many
SELECT * FROM servers_incidents WHERE status = 'open' ORDER BY last_seen_at DESC;

-- name: ListOpenServersIncidentsByNode :many
SELECT * FROM servers_incidents WHERE node_id = $1 AND status = 'open' ORDER BY last_seen_at DESC;

-- name: CountOpenIncidentsByNode :one
SELECT count(*)::int FROM servers_incidents WHERE node_id = $1 AND status = 'open';

-- name: InsertServersOutbox :one
INSERT INTO servers_outbox (event_id, payload) VALUES ($1, $2)
ON CONFLICT (event_id) DO NOTHING
RETURNING *;

-- name: ListPendingOutbox :many
SELECT * FROM servers_outbox
WHERE delivered_at IS NULL AND next_attempt_at <= now()
ORDER BY next_attempt_at ASC
LIMIT $1;

-- name: MarkOutboxDelivered :exec
UPDATE servers_outbox SET delivered_at = now(), last_error = '' WHERE id = $1;

-- name: BumpOutboxAttempt :exec
UPDATE servers_outbox SET
    attempts = attempts + 1,
    next_attempt_at = $2,
    last_error = $3
WHERE id = $1;

-- name: DeleteOldDeliveredOutbox :exec
DELETE FROM servers_outbox WHERE delivered_at IS NOT NULL AND delivered_at < $1;

-- name: CreateServersInstallation :one
INSERT INTO servers_installations (
    node_id, host, port, username, status, current_step, expected_node_uid,
    install_kind, panel_url, cert_email, display_name
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetServersInstallation :one
SELECT * FROM servers_installations WHERE id = $1;

-- name: UpdateServersInstallation :one
UPDATE servers_installations SET
    status = $2,
    current_step = $3,
    ssh_fingerprint = CASE WHEN sqlc.arg(ssh_fingerprint)::text = '' THEN ssh_fingerprint ELSE sqlc.arg(ssh_fingerprint)::text END,
    node_id = COALESCE(sqlc.narg(node_id), node_id),
    error_code = $4,
    error_message = $5,
    completed_at = sqlc.narg(completed_at),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: InsertServersInstallationLog :one
INSERT INTO servers_installation_logs (installation_id, level, message)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListServersInstallationLogs :many
SELECT * FROM servers_installation_logs
WHERE installation_id = $1 AND id > $2
ORDER BY id ASC
LIMIT $3;

-- name: UpsertServersNodeBilling :one
INSERT INTO servers_node_billing (
    node_id, billing_account_id, mode, provider_name, provider_url, external_service_id,
    cost_minor, currency, period, next_due_date, auto_renew, alert_days, comment
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
ON CONFLICT (node_id) DO UPDATE SET
    billing_account_id = EXCLUDED.billing_account_id,
    mode = EXCLUDED.mode,
    provider_name = EXCLUDED.provider_name,
    provider_url = EXCLUDED.provider_url,
    external_service_id = EXCLUDED.external_service_id,
    cost_minor = EXCLUDED.cost_minor,
    currency = EXCLUDED.currency,
    period = EXCLUDED.period,
    next_due_date = EXCLUDED.next_due_date,
    auto_renew = EXCLUDED.auto_renew,
    alert_days = EXCLUDED.alert_days,
    comment = EXCLUDED.comment,
    updated_at = now()
RETURNING *;

-- name: GetServersNodeBilling :one
SELECT * FROM servers_node_billing WHERE node_id = $1;

-- name: ListServersNodeBilling :many
SELECT * FROM servers_node_billing;

-- name: ListMonitoredServices :many
SELECT * FROM servers_monitored_services WHERE node_id = $1 ORDER BY unit_name;

-- name: ReplaceMonitoredServices :exec
DELETE FROM servers_monitored_services WHERE node_id = $1;

-- name: InsertMonitoredService :one
INSERT INTO servers_monitored_services (node_id, unit_name) VALUES ($1, $2)
ON CONFLICT (node_id, unit_name) DO NOTHING
RETURNING *;

-- name: GetKnownHost :one
SELECT * FROM servers_known_hosts WHERE host = $1 AND port = $2;

-- name: UpsertKnownHost :one
INSERT INTO servers_known_hosts (host, port, fingerprint)
VALUES ($1, $2, $3)
ON CONFLICT (host, port) DO UPDATE SET
    fingerprint = EXCLUDED.fingerprint,
    updated_at = now()
RETURNING *;
