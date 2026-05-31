package app

import (
	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func runtimeSettingsView(settings *runtimeSettingsFile) *proxyruntimev1.ProxyRuntimeSettings {
	settings = normalizeRuntimeSettings(settings)
	edge := settings.GetEdgeCanary()
	out := &proxyruntimev1.ProxyRuntimeSettings{
		EdgeCanary: &proxyruntimev1.ProxyEdgeCanarySettingsView{
			Url:             edge.GetUrl(),
			TokenConfigured: edge.GetToken() != "",
			Enabled:         edgeCanaryEnabled(edge),
		},
		CheckSettings: cloneCheckSettings(settings.GetCheckSettings()),
	}
	for _, provider := range settings.GetIpFraudProviders() {
		out.IpFraudProviders = append(out.IpFraudProviders, &proxyruntimev1.ProxyIPFraudProviderSettingsView{
			ProviderId:       provider.GetProviderId(),
			Weight:           provider.GetWeight(),
			Kind:             provider.GetKind(),
			Anonymous:        provider.GetAnonymous(),
			ApiKeyConfigured: len(provider.GetApiKeys()) > 0,
			ApiKeyCount:      uint32(len(provider.GetApiKeys())),
		})
	}
	for _, provider := range settings.GetDynamicIpProviders() {
		out.DynamicIpProviders = append(out.DynamicIpProviders, cloneDynamicIPProvider(provider))
	}
	return out
}
