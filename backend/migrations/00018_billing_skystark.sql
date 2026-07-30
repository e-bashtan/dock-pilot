-- +goose Up
ALTER TABLE billing_accounts DROP CONSTRAINT IF EXISTS billing_accounts_provider_check;
ALTER TABLE billing_accounts
  ADD CONSTRAINT billing_accounts_provider_check
  CHECK (provider IN ('planetahost', 'skystark'));

-- +goose Down
ALTER TABLE billing_accounts DROP CONSTRAINT IF EXISTS billing_accounts_provider_check;
ALTER TABLE billing_accounts
  ADD CONSTRAINT billing_accounts_provider_check
  CHECK (provider IN ('planetahost'));
