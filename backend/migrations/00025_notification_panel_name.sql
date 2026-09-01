-- +goose Up
ALTER TABLE notification_settings
    ADD COLUMN panel_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE notification_settings DROP COLUMN IF EXISTS panel_name;
