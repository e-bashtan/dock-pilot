-- +goose Up
-- Allow agent binary redeploy jobs on existing agent nodes.

ALTER TABLE servers_installations
  DROP CONSTRAINT IF EXISTS servers_installations_install_kind_check;

ALTER TABLE servers_installations
  DROP CONSTRAINT IF EXISTS fleet_installations_install_kind_check;

ALTER TABLE servers_installations
  ADD CONSTRAINT servers_installations_install_kind_check
  CHECK (install_kind IN ('agent', 'barn', 'agent_update'));

-- +goose Down
ALTER TABLE servers_installations
  DROP CONSTRAINT IF EXISTS servers_installations_install_kind_check;

ALTER TABLE servers_installations
  ADD CONSTRAINT servers_installations_install_kind_check
  CHECK (install_kind IN ('agent', 'barn'));
