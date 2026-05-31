package accountproxy

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/geox"
)

func gatewayForPolicy(definition Definition, policy *proxyruntimev1.ProxySessionPolicy) (Gateway, bool) {
	region := strings.ToUpper(strings.TrimSpace(policy.GetRegion()))
	if region != "" {
		for _, gateway := range definition.Gateways {
			if gatewaySupportsRegion(gateway, region) {
				return gateway, true
			}
		}
	}
	return defaultGateway(definition)
}

func gatewaySupportsRegion(gateway Gateway, region string) bool {
	candidates := regionCandidates(region)
	for _, value := range gateway.PreferredRegions {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := candidates[value]; exists {
			return true
		}
		if country := geox.NormalizeCountryAlpha2(value); country != "" {
			if _, exists := candidates[country]; exists {
				return true
			}
		}
		if region := geox.NormalizeRegionCode(value); region != "" {
			if _, exists := candidates[region]; exists {
				return true
			}
		}
	}
	return false
}

func GatewaySupportsRegion(gateway Gateway, region string) bool {
	return gatewaySupportsRegion(gateway, region)
}

func regionCandidates(value string) map[string]struct{} {
	out := map[string]struct{}{}
	addRegionCandidate(out, value)
	if prefix, _, ok := strings.Cut(strings.ToUpper(strings.TrimSpace(value)), "-"); ok {
		addRegionCandidate(out, prefix)
	}
	return out
}

func addRegionCandidate(out map[string]struct{}, value string) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return
	}
	out[value] = struct{}{}
	if country := geox.NormalizeCountryAlpha2(value); country != "" {
		out[country] = struct{}{}
		if region := geox.CountryRegionCode(country); region != "" {
			out[region] = struct{}{}
		}
	}
	if region := geox.NormalizeRegionCode(value); region != "" {
		out[region] = struct{}{}
	}
}

func gatewayIsFallback(gateway Gateway) bool {
	for _, value := range gateway.PreferredRegions {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "*" || value == "ANY" || value == "GLOBAL" {
			return true
		}
	}
	return false
}

func GatewayIsFallback(gateway Gateway) bool {
	return gatewayIsFallback(gateway)
}

func gatewayProtocol(gateway Gateway, fallback string) string {
	if value := defaultProtocol(gateway.DefaultProtocol, fallback); value != "" {
		return value
	}
	for _, value := range gateway.Protocols {
		if protocol := defaultProtocol(value, fallback); protocol != "" {
			return protocol
		}
	}
	return defaultProtocol(fallback, "socks5")
}

func GatewayProtocol(gateway Gateway, fallback string) string {
	return gatewayProtocol(gateway, fallback)
}

func GatewayProtocolForProvider(providerID string, gateway Gateway) string {
	if plugin, ok := Get(providerID); ok {
		return plugin.GatewayProtocol(gateway)
	}
	return gatewayProtocol(gateway, "socks5")
}
