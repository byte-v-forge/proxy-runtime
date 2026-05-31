package app

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

func gatewayCandidateProtos(in []scoredGatewayCandidate) []*proxyruntimev1.ProxyDynamicGatewayCandidate {
	out := make([]*proxyruntimev1.ProxyDynamicGatewayCandidate, 0, len(in))
	for _, item := range in {
		out = append(out, item.proto)
	}
	return out
}

func lineCandidateProtos(in []scoredLineCandidate) []*proxyruntimev1.ProxyLineCandidate {
	out := make([]*proxyruntimev1.ProxyLineCandidate, 0, len(in))
	for _, item := range in {
		out = append(out, item.proto)
	}
	return out
}

func protocolEnum(value string) proxyruntimev1.ProxyProtocol {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http", "https":
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP
	case "socks5", "socks5h":
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5
	default:
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_UNSPECIFIED
	}
}

func gatewaysForPlan(settings *runtimeSettingsFile, plan *proxyruntimev1.ProxyChainPlan, providerID string) []accountproxy.Gateway {
	gateways := dynamicIPGateways(settings, providerID)
	gatewayID := strings.TrimSpace(plan.GetDynamicGateway().GetGatewayId())
	if gatewayID == "" {
		return gateways
	}
	for _, gateway := range gateways {
		if gateway.ID == gatewayID {
			return []accountproxy.Gateway{gateway}
		}
	}
	return gateways
}
