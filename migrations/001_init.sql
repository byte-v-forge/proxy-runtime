CREATE TABLE IF NOT EXISTS proxy_runtime_provider_accounts (
  account_id text PRIMARY KEY,
  provider_id text NOT NULL,
  display_name text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  credential_secret text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE proxy_runtime_provider_accounts
  DROP COLUMN IF EXISTS proxy_addr,
  DROP COLUMN IF EXISTS protocol,
  DROP COLUMN IF EXISTS default_sticky_seconds,
  DROP COLUMN IF EXISTS default_region,
  DROP COLUMN IF EXISTS default_state,
  DROP COLUMN IF EXISTS default_city,
  DROP COLUMN IF EXISTS default_asn;

DROP TABLE IF EXISTS proxy_runtime_subscription_sources;
DROP TABLE IF EXISTS proxy_runtime_dynamic_leases;

CREATE TABLE IF NOT EXISTS proxy_runtime_settings (
  setting_key text PRIMARY KEY,
  setting_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now()
);
