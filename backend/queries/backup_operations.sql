-- name: CreateBackupOperation :one
INSERT INTO backup_operations (
    kind, status, database_name, instance_id, schedule_id, s3_key, size_bytes, message
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: FinishBackupOperation :one
UPDATE backup_operations SET
    status = $2,
    s3_key = CASE WHEN sqlc.arg('s3_key')::text <> '' THEN sqlc.arg('s3_key')::text ELSE s3_key END,
    size_bytes = CASE WHEN sqlc.arg('size_bytes')::bigint > 0 THEN sqlc.arg('size_bytes')::bigint ELSE size_bytes END,
    message = $3,
    finished_at = now()
WHERE id = $1
RETURNING *;

-- name: ListBackupOperations :many
SELECT * FROM backup_operations
ORDER BY started_at DESC
LIMIT $1;
