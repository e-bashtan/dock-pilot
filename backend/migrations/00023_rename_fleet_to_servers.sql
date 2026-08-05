-- +goose Up
-- Rename fleet_* tables/indexes/constraints to servers_* (product rebrand).

ALTER TABLE IF EXISTS fleet_settings RENAME TO servers_settings;
ALTER TABLE IF EXISTS fleet_nodes RENAME TO servers_nodes;
ALTER TABLE IF EXISTS fleet_node_credentials RENAME TO servers_node_credentials;
ALTER TABLE IF EXISTS fleet_pairing_codes RENAME TO servers_pairing_codes;
ALTER TABLE IF EXISTS fleet_registration_tokens RENAME TO servers_registration_tokens;
ALTER TABLE IF EXISTS fleet_snapshots RENAME TO servers_snapshots;
ALTER TABLE IF EXISTS fleet_events RENAME TO servers_events;
ALTER TABLE IF EXISTS fleet_incidents RENAME TO servers_incidents;
ALTER TABLE IF EXISTS fleet_outbox RENAME TO servers_outbox;
ALTER TABLE IF EXISTS fleet_installations RENAME TO servers_installations;
ALTER TABLE IF EXISTS fleet_installation_logs RENAME TO servers_installation_logs;
ALTER TABLE IF EXISTS fleet_node_billing RENAME TO servers_node_billing;
ALTER TABLE IF EXISTS fleet_monitored_services RENAME TO servers_monitored_services;
ALTER TABLE IF EXISTS fleet_known_hosts RENAME TO servers_known_hosts;

ALTER INDEX IF EXISTS idx_fleet_nodes_uid_active RENAME TO idx_servers_nodes_uid_active;
ALTER INDEX IF EXISTS idx_fleet_nodes_status RENAME TO idx_servers_nodes_status;
ALTER INDEX IF EXISTS idx_fleet_nodes_connection RENAME TO idx_servers_nodes_connection;
ALTER INDEX IF EXISTS idx_fleet_node_credentials_node RENAME TO idx_servers_node_credentials_node;
ALTER INDEX IF EXISTS idx_fleet_node_credentials_hash RENAME TO idx_servers_node_credentials_hash;
ALTER INDEX IF EXISTS idx_fleet_pairing_codes_hash RENAME TO idx_servers_pairing_codes_hash;
ALTER INDEX IF EXISTS idx_fleet_registration_tokens_hash RENAME TO idx_servers_registration_tokens_hash;
ALTER INDEX IF EXISTS idx_fleet_snapshots_node_collected RENAME TO idx_servers_snapshots_node_collected;
ALTER INDEX IF EXISTS idx_fleet_events_node_occurred RENAME TO idx_servers_events_node_occurred;
ALTER INDEX IF EXISTS idx_fleet_events_type_occurred RENAME TO idx_servers_events_type_occurred;
ALTER INDEX IF EXISTS idx_fleet_events_severity_occurred RENAME TO idx_servers_events_severity_occurred;
ALTER INDEX IF EXISTS idx_fleet_incidents_open_dedup RENAME TO idx_servers_incidents_open_dedup;
ALTER INDEX IF EXISTS idx_fleet_incidents_node_status RENAME TO idx_servers_incidents_node_status;
ALTER INDEX IF EXISTS idx_fleet_outbox_pending RENAME TO idx_servers_outbox_pending;
ALTER INDEX IF EXISTS idx_fleet_installations_status RENAME TO idx_servers_installations_status;
ALTER INDEX IF EXISTS idx_fleet_installation_logs_inst RENAME TO idx_servers_installation_logs_inst;

-- Named CHECK constraints from later migrations (ignore if missing on older DBs).
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fleet_nodes_connection_type_check'
  ) THEN
    ALTER TABLE servers_nodes
      RENAME CONSTRAINT fleet_nodes_connection_type_check TO servers_nodes_connection_type_check;
  END IF;
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fleet_installations_install_kind_check'
  ) THEN
    ALTER TABLE servers_installations
      RENAME CONSTRAINT fleet_installations_install_kind_check
      TO servers_installations_install_kind_check;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE IF EXISTS servers_settings RENAME TO fleet_settings;
ALTER TABLE IF EXISTS servers_nodes RENAME TO fleet_nodes;
ALTER TABLE IF EXISTS servers_node_credentials RENAME TO fleet_node_credentials;
ALTER TABLE IF EXISTS servers_pairing_codes RENAME TO fleet_pairing_codes;
ALTER TABLE IF EXISTS servers_registration_tokens RENAME TO fleet_registration_tokens;
ALTER TABLE IF EXISTS servers_snapshots RENAME TO fleet_snapshots;
ALTER TABLE IF EXISTS servers_events RENAME TO fleet_events;
ALTER TABLE IF EXISTS servers_incidents RENAME TO fleet_incidents;
ALTER TABLE IF EXISTS servers_outbox RENAME TO fleet_outbox;
ALTER TABLE IF EXISTS servers_installations RENAME TO fleet_installations;
ALTER TABLE IF EXISTS servers_installation_logs RENAME TO fleet_installation_logs;
ALTER TABLE IF EXISTS servers_node_billing RENAME TO fleet_node_billing;
ALTER TABLE IF EXISTS servers_monitored_services RENAME TO fleet_monitored_services;
ALTER TABLE IF EXISTS servers_known_hosts RENAME TO fleet_known_hosts;

ALTER INDEX IF EXISTS idx_servers_nodes_uid_active RENAME TO idx_fleet_nodes_uid_active;
ALTER INDEX IF EXISTS idx_servers_nodes_status RENAME TO idx_fleet_nodes_status;
ALTER INDEX IF EXISTS idx_servers_nodes_connection RENAME TO idx_fleet_nodes_connection;
ALTER INDEX IF EXISTS idx_servers_node_credentials_node RENAME TO idx_fleet_node_credentials_node;
ALTER INDEX IF EXISTS idx_servers_node_credentials_hash RENAME TO idx_fleet_node_credentials_hash;
ALTER INDEX IF EXISTS idx_servers_pairing_codes_hash RENAME TO idx_fleet_pairing_codes_hash;
ALTER INDEX IF EXISTS idx_servers_registration_tokens_hash RENAME TO idx_fleet_registration_tokens_hash;
ALTER INDEX IF EXISTS idx_servers_snapshots_node_collected RENAME TO idx_fleet_snapshots_node_collected;
ALTER INDEX IF EXISTS idx_servers_events_node_occurred RENAME TO idx_fleet_events_node_occurred;
ALTER INDEX IF EXISTS idx_servers_events_type_occurred RENAME TO idx_fleet_events_type_occurred;
ALTER INDEX IF EXISTS idx_servers_events_severity_occurred RENAME TO idx_fleet_events_severity_occurred;
ALTER INDEX IF EXISTS idx_servers_incidents_open_dedup RENAME TO idx_fleet_incidents_open_dedup;
ALTER INDEX IF EXISTS idx_servers_incidents_node_status RENAME TO idx_fleet_incidents_node_status;
ALTER INDEX IF EXISTS idx_servers_outbox_pending RENAME TO idx_fleet_outbox_pending;
ALTER INDEX IF EXISTS idx_servers_installations_status RENAME TO idx_fleet_installations_status;
ALTER INDEX IF EXISTS idx_servers_installation_logs_inst RENAME TO idx_fleet_installation_logs_inst;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'servers_nodes_connection_type_check'
  ) THEN
    ALTER TABLE fleet_nodes
      RENAME CONSTRAINT servers_nodes_connection_type_check TO fleet_nodes_connection_type_check;
  END IF;
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'servers_installations_install_kind_check'
  ) THEN
    ALTER TABLE fleet_installations
      RENAME CONSTRAINT servers_installations_install_kind_check
      TO fleet_installations_install_kind_check;
  END IF;
END $$;
-- +goose StatementEnd
