package app

import (
	"fmt"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

func dynamicIPGatewayMap(settings *runtimeSettingsFile) map[string][]accountproxy.Gateway {
	out := map[string][]accountproxy.Gateway{}
	for _, provider := range normalizeRuntimeSettings(settings).GetDynamicIpProviders() {
		out[provider.GetProviderId()] = accountProxyGateways(provider.GetGateways())
	}
	return out
}

func dynamicIPGateways(settings *runtimeSettingsFile, providerID string) []accountproxy.Gateway {
	return dynamicIPGatewayMap(settings)[strings.TrimSpace(providerID)]
}

func dynamicIPProviderFromProto(in *proxyruntimev1.ProxyDynamicIPProviderSettings) *proxyruntimev1.ProxyDynamicIPProviderSettings {
	if in == nil {
		return &proxyruntimev1.ProxyDynamicIPProviderSettings{}
	}
	out := &proxyruntimev1.ProxyDynamicIPProviderSettings{ProviderId: strings.TrimSpace(in.GetProviderId()), Gateways: make([]*proxyruntimev1.ProxyDynamicIPGatewaySettings, 0, len(in.GetGateways()))}
	for _, gateway := range in.GetGateways() {
		out.Gateways = append(out.Gateways, dynamicIPGatewayFromProto(gateway))
	}
	normalizeDynamicIPProvider(out)
	return out
}

func dynamicIPGatewayFromProto(in *proxyruntimev1.ProxyDynamicIPGatewaySettings) *proxyruntimev1.ProxyDynamicIPGatewaySettings {
	if in == nil {
		return &proxyruntimev1.ProxyDynamicIPGatewaySettings{}
	}
	out := &proxyruntimev1.ProxyDynamicIPGatewaySettings{
		GatewayId:       strings.TrimSpace(in.GetGatewayId()),
		DisplayName:     strings.TrimSpace(in.GetDisplayName()),
		Addr:            strings.TrimSpace(in.GetAddr()),
		RegionCodes:     cleanRegionCodes(in.GetRegionCodes()),
		Protocols:       cleanProtocols(in.GetProtocols()),
		DefaultProtocol: in.GetDefaultProtocol(),
	}
	return out
}

func normalizeDynamicIPProvider(provider *proxyruntimev1.ProxyDynamicIPProviderSettings) {
	if provider == nil {
		return
	}
	provider.ProviderId = strings.TrimSpace(provider.GetProviderId())
	for index := range provider.Gateways {
		normalizeDynamicIPGateway(provider.Gateways[index], index)
	}
}

func validateDynamicIPProvider(provider *proxyruntimev1.ProxyDynamicIPProviderSettings, index int) error {
	if !accountproxy.IsSupported(provider.GetProviderId()) {
		return fmt.Errorf("dynamic_ip_providers[%d].provider_id is unsupported", index)
	}
	seen := map[string]struct{}{}
	for gatewayIndex, gateway := range provider.GetGateways() {
		if strings.TrimSpace(gateway.GetAddr()) == "" {
			return fmt.Errorf("dynamic_ip_providers[%d].gateways[%d].addr is required", index, gatewayIndex)
		}
		if _, exists := seen[gateway.GetGatewayId()]; exists {
			return fmt.Errorf("dynamic_ip_providers[%d].gateways[%d] duplicates gateway %q", index, gatewayIndex, gateway.GetGatewayId())
		}
		seen[gateway.GetGatewayId()] = struct{}{}
	}
	return nil
}

func normalizeDynamicIPGateway(gateway *proxyruntimev1.ProxyDynamicIPGatewaySettings, index int) {
	if gateway == nil {
		return
	}
	gateway.GatewayId = strings.TrimSpace(gateway.GetGatewayId())
	if gateway.GatewayId == "" {
		gateway.GatewayId = fmt.Sprintf("gateway-%d", index+1)
	}
	gateway.DisplayName = strings.TrimSpace(gateway.GetDisplayName())
	gateway.Addr = strings.TrimSpace(gateway.GetAddr())
	gateway.RegionCodes = cleanRegionCodes(gateway.GetRegionCodes())
	gateway.Protocols = cleanProtocols(gateway.GetProtocols())
}

func accountProxyGateways(gateways []*proxyruntimev1.ProxyDynamicIPGatewaySettings) []accountproxy.Gateway {
	out := make([]accountproxy.Gateway, 0, len(gateways))
	for _, gateway := range gateways {
		out = append(out, accountproxy.Gateway{
			ID:               gateway.GetGatewayId(),
			DisplayName:      gateway.GetDisplayName(),
			Addr:             gateway.GetAddr(),
			DefaultProtocol:  configuredProtocolName(gateway.GetDefaultProtocol()),
			Protocols:        protocolNames(gateway.GetProtocols()),
			PreferredRegions: gateway.GetRegionCodes(),
		})
	}
	return out
}

func cloneDynamicIPProvider(in *proxyruntimev1.ProxyDynamicIPProviderSettings) *proxyruntimev1.ProxyDynamicIPProviderSettings {
	return dynamicIPProviderFromProto(in)
}
