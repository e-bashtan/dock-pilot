-- name: ListBillingAccounts :many
SELECT * FROM billing_accounts
ORDER BY server_ip ASC;

-- name: GetBillingAccount :one
SELECT * FROM billing_accounts WHERE id = $1;

-- name: CreateBillingAccount :one
INSERT INTO billing_accounts (
    provider, server_ip, login, encrypted_password, billmgr_url, alert_days, enabled
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateBillingAccount :one
UPDATE billing_accounts SET
    provider = $2,
    server_ip = $3,
    login = $4,
    billmgr_url = $5,
    alert_days = $6,
    enabled = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateBillingAccountPassword :one
UPDATE billing_accounts SET
    encrypted_password = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateBillingAccountCache :one
UPDATE billing_accounts SET
    cached_expire_date = $2,
    cached_status = $3,
    cached_name = $4,
    cached_cost = $5,
    last_checked_at = now(),
    last_check_error = $6,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkBillingAccountAlerted :one
UPDATE billing_accounts SET
    last_alert_expire_date = $2,
    last_alert_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteBillingAccount :exec
DELETE FROM billing_accounts WHERE id = $1;

-- name: ListEnabledBillingAccounts :many
SELECT * FROM billing_accounts
WHERE enabled = true
ORDER BY server_ip ASC;
