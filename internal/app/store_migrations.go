package app

import (
	"context"
)

func (s *PostgresStore) applyMigrations(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrationSQL)
	return err
}

const migrationSQL = `
CREATE TABLE IF NOT EXISTS proxy_runtime_provider_accounts (account_id text PRIMARY KEY, provider_id text NOT NULL, display_name text NOT NULL, enabled boolean NOT NULL DEFAULT true, credential_secret text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
ALTER TABLE proxy_runtime_provider_accounts DROP COLUMN IF EXISTS proxy_addr, DROP COLUMN IF EXISTS protocol, DROP COLUMN IF EXISTS default_sticky_seconds, DROP COLUMN IF EXISTS default_region, DROP COLUMN IF EXISTS default_state, DROP COLUMN IF EXISTS default_city, DROP COLUMN IF EXISTS default_asn;
DROP TABLE IF EXISTS proxy_runtime_subscription_sources;
DROP TABLE IF EXISTS proxy_runtime_dynamic_leases;
CREATE TABLE IF NOT EXISTS proxy_runtime_settings (setting_key text PRIMARY KEY, setting_json jsonb NOT NULL DEFAULT '{}'::jsonb, updated_at timestamptz NOT NULL DEFAULT now());
UPDATE proxy_runtime_settings
SET setting_json = jsonb_set(
	setting_json,
	'{ip_fraud_providers}',
	COALESCE((
		SELECT jsonb_agg(provider)
		FROM jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(setting_json->'ip_fraud_providers') = 'array' THEN setting_json->'ip_fraud_providers'
				ELSE '[]'::jsonb
			END
		) AS provider
		WHERE provider->>'kind' <> 'PROXY_IP_FRAUD_PROVIDER_KIND_FFRAUD'
			AND provider->>'provider_id' <> 'ffraud'
	), '[]'::jsonb),
	true
)
WHERE setting_key = 'runtime'
	AND setting_json ? 'ip_fraud_providers';
`
