package mihomo

import (
	"net/url"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

func sourceNodes(bindings []nodeListener) []provider.Node {
	out := make([]provider.Node, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, sourceNode(binding))
	}
	return out
}

func sourceNode(binding nodeListener) provider.Node {
	proxyURL := &url.URL{Scheme: "socks5", Host: binding.Endpoint.Addr}
	return provider.Node{
		ID:           lineListenerName(binding.SourceID, binding.NodeID),
		URL:          proxyURL,
		ProviderID:   ProviderID,
		UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_PROXY_POOL,
		RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_SCHEDULED_POOL_REFRESH,
		Labels: map[string]string{
			"mode":            "subscription_source_node",
			"source_runtime":  ProviderID,
			"line_source_id":  binding.SourceID,
			"line_node_id":    binding.NodeID,
			"line_node_name":  binding.DisplayName,
			"line_proxy_name": binding.ProxyName,
		},
	}
}
