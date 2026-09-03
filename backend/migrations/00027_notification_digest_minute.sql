-- +goose Up
ALTER TABLE notification_settings
    ADD COLUMN daily_digest_minute INT NOT NULL DEFAULT 0
    CHECK (daily_digest_minute >= 0 AND daily_digest_minute <= 55 AND daily_digest_minute % 5 = 0);

-- +goose Down
ALTER TABLE notification_settings DROP COLUMN IF EXISTS daily_digest_minute;
