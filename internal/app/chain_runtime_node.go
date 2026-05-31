package app

import (
	"net/url"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

const sourceRuntimeProviderID = "mihomo"

func (r *Runtime) currentPool() []provider.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneNodes(r.pool)
}

func (r *Runtime) sourceRuntimeNodeForLine(pool []provider.Node, line *proxyruntimev1.ProxyLineCandidate) *provider.Node {
	if line == nil {
		return nil
	}
	for _, node := range pool {
		if node.ProviderID == sourceRuntimeProviderID && node.URL != nil &&
			node.Labels["line_source_id"] == line.GetSourceId() &&
			node.Labels["line_node_id"] == line.GetNodeId() {
			copy := node
			copy.Labels = cloneStringMap(copy.Labels)
			return &copy
		}
	}
	return nil
}

func lineNodeForPlan(plan *proxyruntimev1.ProxyChainPlan, node *provider.Node) *provider.Node {
	if plan.GetLine() == nil || node == nil || node.URL == nil {
		return nil
	}
	copy := *node
	copy.Labels = cloneStringMap(copy.Labels)
	if copy.Labels == nil {
		copy.Labels = map[string]string{}
	}
	copy.Labels["line_source_id"] = plan.GetLine().GetSourceId()
	copy.Labels["line_node_id"] = plan.GetLine().GetNodeId()
	if hop := chainHopByRole(plan, proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_LINE_PROXY); hop != nil {
		copy.Labels["line_observed_ip"] = hop.GetObservedIp()
	}
	copy.Labels["chain_id"] = plan.GetChainId()
	return &copy
}

func routeStaticChain(_ []*url.URL, line *provider.Node) []*url.URL {
	if line != nil && line.URL != nil {
		return []*url.URL{cloneURL(line.URL)}
	}
	// Dynamic lease chains are selected explicitly by the chain planner.
	// Do not silently fall back to the process-level static chain here: that
	// hides route selection from the plan and can make business workflows look
	// successful while bypassing the selected subscription/fixed line.
	return nil
}

func cloneURL(in *url.URL) *url.URL {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
