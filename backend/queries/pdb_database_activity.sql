-- name: UpsertPdbDatabaseActivity :one
INSERT INTO pdb_database_activity (
    instance_id, database_name, inserts, updates, deletes, checked_at
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (instance_id, database_name) DO UPDATE SET
    last_dml_at = CASE
        WHEN EXCLUDED.inserts > pdb_database_activity.inserts
          OR EXCLUDED.updates > pdb_database_activity.updates
          OR EXCLUDED.deletes > pdb_database_activity.deletes
        THEN EXCLUDED.checked_at
        ELSE pdb_database_activity.last_dml_at
    END,
    inserts = EXCLUDED.inserts,
    updates = EXCLUDED.updates,
    deletes = EXCLUDED.deletes,
    checked_at = EXCLUDED.checked_at
RETURNING *;
