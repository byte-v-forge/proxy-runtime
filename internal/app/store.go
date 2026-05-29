package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
	"github.com/byte-v-forge/proxy-runtime/internal/secretbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const runtimeSettingsKey = "runtime"

type PostgresStore struct {
	pool   *pgxpool.Pool
	box    secretbox.Box
	logger *slog.Logger
}

type providerCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type providerAccountRecord struct {
	AccountID        string
	ProviderID       string
	DisplayName      string
	Enabled          bool
	CredentialSecret string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewPostgresStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (*PostgresStore, error) {
	box, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	store := &PostgresStore{pool: pool, box: box, logger: logger}
	if cfg.ApplyMigrations {
		if err := store.applyMigrations(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	if err := store.seedFromConfig(ctx, cfg); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresStore) applyMigrations(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrationSQL)
	return err
}

func (s *PostgresStore) seedFromConfig(ctx context.Context, cfg config.Config) error {
	count := 0
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM proxy_runtime_provider_accounts`).Scan(&count); err != nil {
		return err
	}
	if count > 0 || strings.TrimSpace(cfg.Ten24.Username) == "" || strings.TrimSpace(cfg.Ten24.Password) == "" {
		return nil
	}
	_, err := s.UpsertProviderAccount(ctx, &proxyruntimev1.UpsertProxyProviderAccountRequest{
		AccountId:     "default-1024proxy",
		ProviderId:    accountproxy.ProviderTen24,
		DisplayName:   "Default 1024Proxy",
		Enabled:       true,
		Username:      cfg.Ten24.Username,
		Password:      cfg.Ten24.Password,
		ClearPassword: false,
	})
	return err
}

func (s *PostgresStore) ListProviderAccounts(ctx context.Context) ([]*proxyruntimev1.ProxyProviderAccount, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+providerAccountColumns()+` FROM proxy_runtime_provider_accounts ORDER BY account_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*proxyruntimev1.ProxyProviderAccount{}
	for rows.Next() {
		record, err := scanProviderAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record.toProto())
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpsertProviderAccount(ctx context.Context, req *proxyruntimev1.UpsertProxyProviderAccountRequest) (*proxyruntimev1.ProxyProviderAccount, error) {
	accountID := normalizeID(req.GetAccountId())
	if accountID == "" {
		generated, err := generatedID("dynacct")
		if err != nil {
			return nil, err
		}
		accountID = generated
	}
	providerID := firstNonEmpty(req.GetProviderId(), accountproxy.ProviderTen24)
	if !accountproxy.IsSupported(providerID) {
		return nil, fmt.Errorf("unsupported provider_id %q", providerID)
	}
	existing, _ := s.providerAccountRecord(ctx, accountID)
	secret := ""
	if existing != nil {
		secret = existing.CredentialSecret
	}
	if req.GetClearPassword() {
		secret = ""
	}
	if strings.TrimSpace(req.GetUsername()) != "" || strings.TrimSpace(req.GetPassword()) != "" {
		payload, err := json.Marshal(providerCredential{Username: strings.TrimSpace(req.GetUsername()), Password: strings.TrimSpace(req.GetPassword())})
		if err != nil {
			return nil, err
		}
		secret, err = s.box.Seal(payload)
		if err != nil {
			return nil, err
		}
	}
	enabled := req.GetEnabled()
	displayName := firstNonEmpty(req.GetDisplayName(), accountID)
	if enabled {
		cfg := accountproxy.Config{ProviderID: providerID}
		if plain := credentialFromSecret(s.box, secret); plain != nil {
			cfg.Username = plain.Username
			cfg.Password = plain.Password
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("enabled provider account invalid: %w", err)
		}
	}
	row := s.pool.QueryRow(ctx, `
INSERT INTO proxy_runtime_provider_accounts (account_id, provider_id, display_name, enabled, credential_secret)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (account_id) DO UPDATE SET provider_id=EXCLUDED.provider_id, display_name=EXCLUDED.display_name, enabled=EXCLUDED.enabled, credential_secret=EXCLUDED.credential_secret, updated_at=now()
RETURNING `+providerAccountColumns(), accountID, providerID, displayName, enabled, secret)
	record, err := scanProviderAccount(row)
	if err != nil {
		return nil, err
	}
	return record.toProto(), nil
}

func credentialFromSecret(box secretbox.Box, secret string) *providerCredential {
	plain, err := box.Open(secret)
	if err != nil || len(plain) == 0 {
		return nil
	}
	var credential providerCredential
	if err := json.Unmarshal(plain, &credential); err != nil {
		return nil
	}
	return &credential
}

func generatedID(prefix string) (string, error) {
	suffix, err := randx.Hex(6)
	if err != nil {
		return "", err
	}
	return prefix + "-" + suffix, nil
}

func (s *PostgresStore) DeleteProviderAccount(ctx context.Context, accountID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM proxy_runtime_provider_accounts WHERE account_id=$1`, normalizeID(accountID))
	return err
}

func (s *PostgresStore) ProviderConfig(ctx context.Context, accountID string) (accountproxy.Config, string, error) {
	record, err := s.providerAccountRecord(ctx, accountID)
	if err != nil {
		return accountproxy.Config{}, "", err
	}
	if !record.Enabled {
		return accountproxy.Config{}, "", errors.New("provider account is disabled")
	}
	plain, err := s.box.Open(record.CredentialSecret)
	if err != nil {
		return accountproxy.Config{}, "", err
	}
	var credential providerCredential
	if len(plain) > 0 {
		_ = json.Unmarshal(plain, &credential)
	}
	cfg := accountproxy.Config{ProviderID: record.ProviderID, Username: credential.Username, Password: credential.Password}
	return cfg, record.AccountID, cfg.Validate()
}

func (s *PostgresStore) DefaultProviderAccountID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT account_id FROM proxy_runtime_provider_accounts WHERE enabled ORDER BY updated_at DESC, account_id LIMIT 1`).Scan(&id)
	return id, err
}

func (s *PostgresStore) providerAccountRecord(ctx context.Context, accountID string) (*providerAccountRecord, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+providerAccountColumns()+` FROM proxy_runtime_provider_accounts WHERE account_id=$1`, normalizeID(accountID))
	return scanProviderAccount(row)
}

func providerAccountColumns() string {
	return `account_id, provider_id, display_name, enabled, credential_secret, created_at, updated_at`
}

func scanProviderAccount(row pgx.Row) (*providerAccountRecord, error) {
	var record providerAccountRecord
	err := row.Scan(&record.AccountID, &record.ProviderID, &record.DisplayName, &record.Enabled, &record.CredentialSecret, &record.CreatedAt, &record.UpdatedAt)
	return &record, err
}

func (r providerAccountRecord) toProto() *proxyruntimev1.ProxyProviderAccount {
	status := proxyruntimev1.ProxyProviderAccountStatus_PROXY_PROVIDER_ACCOUNT_STATUS_DISABLED
	if r.Enabled {
		status = proxyruntimev1.ProxyProviderAccountStatus_PROXY_PROVIDER_ACCOUNT_STATUS_ENABLED
	}
	return &proxyruntimev1.ProxyProviderAccount{
		AccountId:            r.AccountID,
		ProviderId:           r.ProviderID,
		DisplayName:          r.DisplayName,
		Status:               status,
		CredentialConfigured: r.CredentialSecret != "",
		CreatedAt:            timestamppb.New(r.CreatedAt),
		UpdatedAt:            timestamppb.New(r.UpdatedAt),
	}
}

func (s *PostgresStore) ListSources(ctx context.Context, staticEndpointCount int, gateways map[string][]accountproxy.Gateway) ([]*proxyruntimev1.ProxySourceDescriptor, error) {
	out := []*proxyruntimev1.ProxySourceDescriptor{}
	if staticEndpointCount > 0 {
		out = append(out, fixedSource(staticEndpointCount))
	}
	accounts, err := s.ListProviderAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.GetStatus() == proxyruntimev1.ProxyProviderAccountStatus_PROXY_PROVIDER_ACCOUNT_STATUS_ENABLED && len(gateways[account.GetProviderId()]) > 0 {
			out = append(out, dynamicSource(account, gateways[account.GetProviderId()]))
		}
	}
	return out, nil
}

func (s *PostgresStore) LoadRuntimeSettings(ctx context.Context) (*runtimeSettingsFile, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `SELECT setting_json::text FROM proxy_runtime_settings WHERE setting_key=$1`, runtimeSettingsKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return normalizeRuntimeSettings(nil), nil
	}
	if err != nil {
		return nil, err
	}
	settings := &proxyruntimev1.ProxyRuntimePersistentSettings{}
	if raw != "" {
		_ = protojsonx.Unmarshal([]byte(raw), settings)
	}
	return normalizeRuntimeSettings(settings), nil
}

func (s *PostgresStore) SaveRuntimeSettings(ctx context.Context, settings *runtimeSettingsFile) error {
	data, err := protojsonx.Marshal(normalizeRuntimeSettings(settings))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO proxy_runtime_settings (setting_key, setting_json) VALUES ($1,$2::jsonb) ON CONFLICT (setting_key) DO UPDATE SET setting_json=EXCLUDED.setting_json, updated_at=now()`, runtimeSettingsKey, string(data))
	return err
}

func normalizeID(value string) string { return strings.TrimSpace(value) }

func fixedSource(count int) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: "fixed-static-chain", ProviderId: "static", DisplayName: "Static chain", Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_FIXED_PROXY, Enabled: true, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING}, Model: &proxyruntimev1.ProxySourceDescriptor_FixedProxy{FixedProxy: &proxyruntimev1.ProxyFixedSourceDescriptor{EndpointCount: uint32(count)}}}
}

func dynamicSource(account *proxyruntimev1.ProxyProviderAccount, gateways []accountproxy.Gateway) *proxyruntimev1.ProxySourceDescriptor {
	return accountproxy.DynamicSource(account.GetProviderId(), account.GetDisplayName(), account.GetAccountId(), gateways)
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
