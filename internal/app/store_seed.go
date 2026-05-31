package app

import (
	"context"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

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
