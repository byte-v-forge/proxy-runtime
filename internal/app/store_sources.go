package app

import (
	"context"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

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

func fixedSource(count int) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: "fixed-static-chain", ProviderId: "static", DisplayName: "Static chain", Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_FIXED_PROXY, Enabled: true, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING}, Model: &proxyruntimev1.ProxySourceDescriptor_FixedProxy{FixedProxy: &proxyruntimev1.ProxyFixedSourceDescriptor{EndpointCount: uint32(count)}}}
}

func dynamicSource(account *proxyruntimev1.ProxyProviderAccount, gateways []accountproxy.Gateway) *proxyruntimev1.ProxySourceDescriptor {
	return accountproxy.DynamicSource(account.GetProviderId(), account.GetDisplayName(), account.GetAccountId(), gateways)
}
