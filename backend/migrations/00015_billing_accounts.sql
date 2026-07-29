-- +goose Up
CREATE TABLE billing_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL DEFAULT 'planetahost'
        CHECK (provider IN ('planetahost')),
    server_ip TEXT NOT NULL,
    login TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    billmgr_url TEXT NOT NULL DEFAULT 'https://bill.planetahost.ru/billmgr',
    alert_days INT NOT NULL DEFAULT 10 CHECK (alert_days >= 1 AND alert_days <= 90),
    enabled BOOLEAN NOT NULL DEFAULT true,
    cached_expire_date DATE,
    cached_status TEXT NOT NULL DEFAULT '',
    cached_name TEXT NOT NULL DEFAULT '',
    cached_cost TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    last_check_error TEXT NOT NULL DEFAULT '',
    last_alert_expire_date DATE,
    last_alert_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, server_ip)
);

CREATE INDEX idx_billing_accounts_enabled ON billing_accounts(enabled);

-- +goose Down
DROP TABLE IF EXISTS billing_accounts;
