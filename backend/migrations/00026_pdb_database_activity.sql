-- +goose Up
CREATE TABLE pdb_database_activity (
    instance_id UUID NOT NULL REFERENCES pdb_instances(id) ON DELETE CASCADE,
    database_name TEXT NOT NULL,
    inserts BIGINT NOT NULL DEFAULT 0,
    updates BIGINT NOT NULL DEFAULT 0,
    deletes BIGINT NOT NULL DEFAULT 0,
    last_dml_at TIMESTAMPTZ,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, database_name)
);

-- +goose Down
DROP TABLE IF EXISTS pdb_database_activity;
