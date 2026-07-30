-- +goose Up
ALTER TABLE fleet_node_billing
  ADD COLUMN IF NOT EXISTS alert_days INT NOT NULL DEFAULT 10
  CHECK (alert_days >= 1 AND alert_days <= 90);

-- +goose Down
ALTER TABLE fleet_node_billing DROP COLUMN IF EXISTS alert_days;
