-- +goose Up
-- Rebrand DockPilot → Barn
-- CHECK must be dropped before UPDATE: old constraints reject 'barn' / new values.

-- Drop old CHECK constraints first
ALTER TABLE fleet_nodes DROP CONSTRAINT IF EXISTS fleet_nodes_connection_type_check;
ALTER TABLE fleet_installations DROP CONSTRAINT IF EXISTS fleet_installations_install_kind_check;

-- Update connection_type from 'dockpilot' to 'barn'
UPDATE fleet_nodes SET connection_type = 'barn' WHERE connection_type = 'dockpilot';

-- Update install_kind from 'dockpilot' to 'barn'
UPDATE fleet_installations SET install_kind = 'barn' WHERE install_kind = 'dockpilot';

-- Recreate CHECK constraints to allow 'barn' instead of 'dockpilot'
ALTER TABLE fleet_nodes
    ADD CONSTRAINT fleet_nodes_connection_type_check
    CHECK (connection_type IN ('local', 'barn', 'agent'));

ALTER TABLE fleet_installations
    ADD CONSTRAINT fleet_installations_install_kind_check
    CHECK (install_kind IN ('agent', 'barn'));

-- Update s3_prefix defaults in existing rows (optional - commented out to avoid changing existing data)
-- UPDATE pdb_backup_schedules SET s3_prefix = 'barn/pg-backups' WHERE s3_prefix = 'dock-pilot/pg-backups';
-- UPDATE panel_backup_settings SET s3_prefix = 'barn/backups' WHERE s3_prefix = 'dock-pilot/backups';

-- +goose Down
-- Drop new CHECK constraints first
ALTER TABLE fleet_nodes DROP CONSTRAINT IF EXISTS fleet_nodes_connection_type_check;
ALTER TABLE fleet_installations DROP CONSTRAINT IF EXISTS fleet_installations_install_kind_check;

-- Revert connection_type from 'barn' to 'dockpilot'
UPDATE fleet_nodes SET connection_type = 'dockpilot' WHERE connection_type = 'barn';

-- Revert install_kind from 'barn' to 'dockpilot'
UPDATE fleet_installations SET install_kind = 'dockpilot' WHERE install_kind = 'barn';

-- Recreate old CHECK constraints
ALTER TABLE fleet_nodes
    ADD CONSTRAINT fleet_nodes_connection_type_check
    CHECK (connection_type IN ('local', 'dockpilot', 'agent'));

ALTER TABLE fleet_installations
    ADD CONSTRAINT fleet_installations_install_kind_check
    CHECK (install_kind IN ('agent', 'dockpilot'));
