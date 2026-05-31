package app

import (
	"fmt"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
)

func providerSecrets(settings *runtimeSettingsFile) map[string][]string {
	secrets := map[string][]string{}
	for _, item := range normalizeRuntimeSettings(settings).GetIpFraudProviders() {
		secrets[providerSecretKey(item.GetKind(), item.GetProviderId())] = append([]string(nil), item.GetApiKeys()...)
	}
	return secrets
}

func providerSecretKey(kind proxyruntimev1.ProxyIPFraudProviderKind, id string) string {
	return fmt.Sprintf("%d:%s", kind, strings.TrimSpace(id))
}

func ipFraudProviders(settings *runtimeSettingsFile) []ipfraud.ProviderConfig {
	items := normalizeRuntimeSettings(settings).GetIpFraudProviders()
	providers := make([]ipfraud.ProviderConfig, 0, len(items))
	for _, item := range items {
		providers = append(providers, ipfraud.ProviderConfig{
			ID:     item.GetProviderId(),
			Kind:   item.GetKind(),
			Weight: int(item.GetWeight()),
			Auth:   ipFraudAuth(item),
		})
	}
	return providers
}

func ipFraudProviderFromRequest(in *proxyruntimev1.ProxyIPFraudProviderSettings, current map[string][]string, index int) *proxyruntimev1.ProxyIPFraudProviderSettings {
	if in == nil {
		return &proxyruntimev1.ProxyIPFraudProviderSettings{}
	}
	id := strings.TrimSpace(in.GetProviderId())
	if id == "" {
		id = ipfraud.DefaultProviderID(in.GetKind())
	}
	apiKeys := cleanList(in.GetApiKeys())
	if len(apiKeys) == 0 && !in.GetClearApiKeys() && !in.GetAnonymous() {
		apiKeys = current[providerSecretKey(in.GetKind(), id)]
	}
	weight := in.GetWeight()
	if weight == 0 {
		weight = providerDefaultWeight(in.GetKind(), index)
	}
	return &proxyruntimev1.ProxyIPFraudProviderSettings{ProviderId: id, Weight: weight, Kind: in.GetKind(), Anonymous: in.GetAnonymous(), ApiKeys: apiKeys}
}

func normalizeIPFraudProvider(provider *proxyruntimev1.ProxyIPFraudProviderSettings, index int) {
	if provider == nil {
		return
	}
	provider.ProviderId = strings.TrimSpace(provider.GetProviderId())
	provider.ApiKeys = cleanList(provider.GetApiKeys())
	if provider.ProviderId == "" {
		provider.ProviderId = ipfraud.DefaultProviderID(provider.GetKind())
	}
	if provider.Weight == 0 {
		provider.Weight = providerDefaultWeight(provider.GetKind(), index)
	}
}

func validateIPFraudProvider(provider *proxyruntimev1.ProxyIPFraudProviderSettings, index int) error {
	plugin, ok := ipfraud.PluginForKind(provider.GetKind())
	if !ok {
		return fmt.Errorf("ip_fraud_providers[%d].kind is required", index)
	}
	if provider.GetAnonymous() && len(provider.GetApiKeys()) > 0 {
		return fmt.Errorf("ip_fraud_providers[%d] must use anonymous or api_keys, not both", index)
	}
	if provider.GetAnonymous() && !plugin.SupportsAnonymous() {
		return fmt.Errorf("ip_fraud_providers[%d] does not support anonymous mode", index)
	}
	if !provider.GetAnonymous() && !plugin.SupportsAPIKey() {
		return fmt.Errorf("ip_fraud_providers[%d] does not support api key mode", index)
	}
	if !provider.GetAnonymous() && len(provider.GetApiKeys()) == 0 {
		return fmt.Errorf("ip_fraud_providers[%d].api_keys is required when anonymous is false", index)
	}
	return nil
}

func supportedIPFraudProviders(providers []*proxyruntimev1.ProxyIPFraudProviderSettings) []*proxyruntimev1.ProxyIPFraudProviderSettings {
	out := make([]*proxyruntimev1.ProxyIPFraudProviderSettings, 0, len(providers))
	for _, provider := range providers {
		if ipfraud.IsProviderKindSupported(provider.GetKind()) {
			out = append(out, provider)
		}
	}
	return out
}

func ipFraudAuth(provider *proxyruntimev1.ProxyIPFraudProviderSettings) ipfraud.AuthConfig {
	if provider.GetAnonymous() {
		return ipfraud.AuthConfig{Anonymous: &ipfraud.AnonymousAuthConfig{}}
	}
	plugin, ok := ipfraud.PluginForKind(provider.GetKind())
	if !ok {
		return ipfraud.AuthConfig{}
	}
	return plugin.Auth(provider.GetApiKeys(), false)
}

func defaultProviderWeight(index int) uint32 {
	if index < 0 {
		return 100
	}
	if index > 9 {
		return 10
	}
	return uint32(100 - index*10)
}

func providerDefaultWeight(kind proxyruntimev1.ProxyIPFraudProviderKind, index int) uint32 {
	if plugin, ok := ipfraud.PluginForKind(kind); ok {
		return plugin.DefaultWeight()
	}
	return defaultProviderWeight(index)
}
