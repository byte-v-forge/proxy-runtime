package app

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func normalizeRuntimeSettings(settings *runtimeSettingsFile) *runtimeSettingsFile {
	if settings == nil {
		settings = &proxyruntimev1.ProxyRuntimePersistentSettings{}
	}
	if settings.EdgeCanary != nil {
		settings.EdgeCanary.Url = strings.TrimSpace(settings.EdgeCanary.GetUrl())
		settings.EdgeCanary.Token = strings.TrimSpace(settings.EdgeCanary.GetToken())
	}
	for index := range settings.IpFraudProviders {
		normalizeIPFraudProvider(settings.IpFraudProviders[index], index)
	}
	settings.IpFraudProviders = supportedIPFraudProviders(settings.IpFraudProviders)
	for index := range settings.DynamicIpProviders {
		normalizeDynamicIPProvider(settings.DynamicIpProviders[index])
	}
	settings.CheckSettings = normalizeCheckSettings(settings.GetCheckSettings())
	return settings
}
