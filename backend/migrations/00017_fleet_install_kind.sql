-- +goose Up
ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS install_kind TEXT NOT NULL DEFAULT 'agent';
ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS panel_url TEXT NOT NULL DEFAULT '';
ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS cert_email TEXT NOT NULL DEFAULT '';
ALTER TABLE fleet_installations ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fleet_installations_install_kind_check'
  ) THEN
    ALTER TABLE fleet_installations
      ADD CONSTRAINT fleet_installations_install_kind_check
      CHECK (install_kind IN ('agent', 'dockpilot'));
  END IF;
END $$;

-- +goose Down
ALTER TABLE fleet_installations DROP CONSTRAINT IF EXISTS fleet_installations_install_kind_check;
ALTER TABLE fleet_installations DROP COLUMN IF EXISTS install_kind;
ALTER TABLE fleet_installations DROP COLUMN IF EXISTS panel_url;
ALTER TABLE fleet_installations DROP COLUMN IF EXISTS cert_email;
ALTER TABLE fleet_installations DROP COLUMN IF EXISTS display_name;
