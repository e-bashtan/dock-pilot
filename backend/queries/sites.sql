-- name: CreateSite :one
INSERT INTO sites (
    name, slug, primary_url, git_repo_url, git_branch,
    dockerfile_path, build_context, container_port, host_port,
    nginx_ssl_enabled, nginx_force_https, site_type,
    docker_volume_mounts, docker_named_volumes, docker_network_host,
    health_check_path, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
) RETURNING *;

-- name: GetSite :one
SELECT * FROM sites WHERE id = $1;

-- name: GetSiteBySlug :one
SELECT * FROM sites WHERE slug = $1;

-- name: ListSites :many
SELECT * FROM sites
ORDER BY (
    SELECT MAX(created_at) FROM deployments WHERE deployments.site_id = sites.id
) DESC NULLS LAST, created_at DESC;

-- name: ListLatestDeployTimes :many
SELECT DISTINCT ON (site_id) site_id, created_at
FROM deployments
ORDER BY site_id, created_at DESC;

-- name: UpdateSite :one
UPDATE sites SET
    name = COALESCE(sqlc.narg('name'), name),
    primary_url = COALESCE(sqlc.narg('primary_url'), primary_url),
    git_repo_url = COALESCE(sqlc.narg('git_repo_url'), git_repo_url),
    git_branch = COALESCE(sqlc.narg('git_branch'), git_branch),
    dockerfile_path = COALESCE(sqlc.narg('dockerfile_path'), dockerfile_path),
    build_context = COALESCE(sqlc.narg('build_context'), build_context),
    container_port = COALESCE(sqlc.narg('container_port'), container_port),
    host_port = COALESCE(sqlc.narg('host_port'), host_port),
    nginx_ssl_enabled = COALESCE(sqlc.narg('nginx_ssl_enabled'), nginx_ssl_enabled),
    nginx_force_https = COALESCE(sqlc.narg('nginx_force_https'), nginx_force_https),
    site_type = COALESCE(sqlc.narg('site_type'), site_type),
    docker_volume_mounts = COALESCE(sqlc.narg('docker_volume_mounts'), docker_volume_mounts),
    docker_named_volumes = COALESCE(sqlc.narg('docker_named_volumes'), docker_named_volumes),
    docker_network_host = COALESCE(sqlc.narg('docker_network_host'), docker_network_host),
    health_check_path = COALESCE(sqlc.narg('health_check_path'), health_check_path),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteSite :exec
DELETE FROM sites WHERE id = $1;

-- name: UpdateSiteStatus :one
UPDATE sites SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateSiteHostPort :one
UPDATE sites SET host_port = $2, updated_at = now() WHERE id = $1 RETURNING *;
