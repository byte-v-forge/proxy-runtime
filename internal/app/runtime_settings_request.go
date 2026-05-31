package app

import (
	"errors"
	"fmt"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func settingsFromRequest(req *proxyruntimev1.UpdateProxyRuntimeSettingsRequest, current *runtimeSettingsFile) (*runtimeSettingsFile, error) {
	current = normalizeRuntimeSettings(current)
	settings := &proxyruntimev1.ProxyRuntimePersistentSettings{
		EdgeCanary:         edgeCanaryFromRequest(req.GetEdgeCanary(), current.GetEdgeCanary()),
		IpFraudProviders:   make([]*proxyruntimev1.ProxyIPFraudProviderSettings, 0, len(req.GetIpFraudProviders())),
		DynamicIpProviders: make([]*proxyruntimev1.ProxyDynamicIPProviderSettings, 0, len(req.GetDynamicIpProviders())),
		CheckSettings:      checkSettingsFromRequest(req.GetCheckSettings(), current.GetCheckSettings()),
	}
	if edgeCanaryEnabled(settings.GetEdgeCanary()) && strings.TrimSpace(settings.GetEdgeCanary().GetUrl()) == "" {
		return nil, errors.New("edge canary url is required when enabled")
	}
	currentProviders := providerSecrets(current)
	seenProviders := map[string]struct{}{}
	for index, provider := range req.GetIpFraudProviders() {
		item := ipFraudProviderFromRequest(provider, currentProviders, index)
		if err := validateIPFraudProvider(item, index); err != nil {
			return nil, err
		}
		key := providerSecretKey(item.GetKind(), item.GetProviderId())
		if _, exists := seenProviders[key]; exists {
			return nil, fmt.Errorf("ip_fraud_providers[%d] duplicates provider %q", index, item.GetProviderId())
		}
		seenProviders[key] = struct{}{}
		settings.IpFraudProviders = append(settings.IpFraudProviders, item)
	}
	seenDynamicProviders := map[string]struct{}{}
	for index, provider := range req.GetDynamicIpProviders() {
		item := dynamicIPProviderFromProto(provider)
		if err := validateDynamicIPProvider(item, index); err != nil {
			return nil, err
		}
		if _, exists := seenDynamicProviders[item.GetProviderId()]; exists {
			return nil, fmt.Errorf("dynamic_ip_providers[%d] duplicates provider %q", index, item.GetProviderId())
		}
		seenDynamicProviders[item.GetProviderId()] = struct{}{}
		settings.DynamicIpProviders = append(settings.DynamicIpProviders, item)
	}
	return normalizeRuntimeSettings(settings), nil
}
