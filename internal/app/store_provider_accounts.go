package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
	"github.com/byte-v-forge/proxy-runtime/internal/secretbox"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

func normalizeID(value string) string { return strings.TrimSpace(value) }
