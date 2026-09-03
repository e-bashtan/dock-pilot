-- name: EnsureNotificationSettings :one
INSERT INTO notification_settings (id) VALUES (1)
ON CONFLICT (id) DO UPDATE SET updated_at = notification_settings.updated_at
RETURNING *;

-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE id = 1;

-- name: UpdateNotificationSettings :one
UPDATE notification_settings SET
    panel_name = $1,
    enabled = $2,
    telegram_chat_id = $3,
    telegram_http_proxy = $4,
    daily_digest_enabled = $5,
    daily_digest_hour = $6,
    daily_digest_minute = $7,
    daily_digest_timezone = $8,
    alert_on_incident_enabled = $9,
    updated_at = now()
WHERE id = 1
RETURNING *;

-- name: UpdateNotificationToken :exec
UPDATE notification_settings
SET encrypted_telegram_bot_token = $1, updated_at = now()
WHERE id = 1;

-- name: ClearNotificationToken :exec
UPDATE notification_settings
SET encrypted_telegram_bot_token = NULL, updated_at = now()
WHERE id = 1;

-- name: UpdateNotificationLastDailySent :exec
UPDATE notification_settings
SET last_daily_sent_at = $1, updated_at = now()
WHERE id = 1;

-- name: UpdateNotificationLastOverall :exec
UPDATE notification_settings
SET last_overall_by_site = $1, updated_at = now()
WHERE id = 1;
