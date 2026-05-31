package app

import (
	"context"
	"sort"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func (r *Runtime) lineCandidates(ctx context.Context, policy *proxyruntimev1.ProxyChainPolicy) ([]scoredLineCandidate, error) {
	if !policy.GetPreferLineProxy() {
		return nil, nil
	}
	sources, err := r.sourcePlane.Sources(ctx)
	if err != nil {
		return nil, err
	}
	nodes, nodeErr := r.sourcePlane.SourceNodes(ctx, "")
	if nodeErr != nil {
		nodes = nil
	}
	regions := map[string][]string{}
	kinds := map[string]proxyruntimev1.ProxySourceKind{}
	sourceNames := map[string]string{}
	for _, source := range sources {
		regions[source.GetSourceId()] = regionCodesWithInferred(sourceRegionCodes(source), source.GetDisplayName())
		kinds[source.GetSourceId()] = source.GetKind()
		sourceNames[source.GetSourceId()] = source.GetDisplayName()
	}
	out := make([]scoredLineCandidate, 0)
	for sourceIndex, source := range sources {
		if !source.GetEnabled() || source.GetKind() == proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_DYNAMIC_IP {
			continue
		}
		if source.GetKind() == proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION && hasNodeForSource(nodes, source.GetSourceId()) {
			continue
		}
		candidate := &proxyruntimev1.ProxyLineCandidate{SourceId: source.GetSourceId(), NodeId: source.GetSourceId(), DisplayName: source.GetDisplayName(), SourceKind: source.GetKind(), Status: proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_UNKNOWN, RegionCodes: regions[source.GetSourceId()], Priority: uint32(sourceIndex), SourceDisplayName: source.GetDisplayName()}
		out = append(out, scoredLineCandidate{proto: candidate, score: lineScore(candidate, policy)})
	}
	for nodeIndex, node := range nodes {
		if node.GetStatus() == proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_UNAVAILABLE {
			continue
		}
		kind := kinds[node.GetSourceId()]
		if kind == proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_UNSPECIFIED {
			kind = proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION
		}
		candidate := &proxyruntimev1.ProxyLineCandidate{SourceId: node.GetSourceId(), NodeId: node.GetNodeId(), DisplayName: node.GetDisplayName(), SourceKind: kind, Status: node.GetStatus(), DelayMs: node.GetDelayMs(), RegionCodes: regionCodesWithInferred(regions[node.GetSourceId()], node.GetDisplayName()), Priority: uint32(nodeIndex), SourceDisplayName: sourceNames[node.GetSourceId()]}
		out = append(out, scoredLineCandidate{proto: candidate, score: lineScore(candidate, policy)})
	}
	return out, nodeErr
}

func chooseLineCandidate(candidates []scoredLineCandidate, policy *proxyruntimev1.ProxyChainPolicy, key string, attempt int) *scoredLineCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if policy.GetStrategy() == proxyruntimev1.ProxyChainStrategy_PROXY_CHAIN_STRATEGY_LOWEST_LATENCY && candidates[i].proto.GetDelayMs() != candidates[j].proto.GetDelayMs() {
			return candidates[i].proto.GetDelayMs() < candidates[j].proto.GetDelayMs()
		}
		return candidates[i].proto.GetPriority() < candidates[j].proto.GetPriority()
	})
	candidates = regionScopedLineCandidates(candidates, policy)
	if attempt > 1 && len(candidates) > 1 {
		return &candidates[(attempt-1)%len(candidates)]
	}
	if policy.GetStrategy() == proxyruntimev1.ProxyChainStrategy_PROXY_CHAIN_STRATEGY_STABLE_HASH && len(candidates) > 1 {
		best := candidates[0].score
		count := 0
		for count < len(candidates) && candidates[count].score == best {
			count++
		}
		return &candidates[int(hashModulo(key, uint32(count)))]
	}
	return &candidates[0]
}

func regionScopedLineCandidates(candidates []scoredLineCandidate, policy *proxyruntimev1.ProxyChainPolicy) []scoredLineCandidate {
	if !hasRequestedRegion(policy) {
		return candidates
	}
	matched := make([]scoredLineCandidate, 0, len(candidates))
	best := 0
	for _, candidate := range candidates {
		best = max(best, regionSpecificScore(candidate.proto.GetRegionCodes(), policy))
	}
	for _, candidate := range candidates {
		score := regionSpecificScore(candidate.proto.GetRegionCodes(), policy)
		if score > 0 && (best < countryRegionMatchScore || score >= countryRegionMatchScore) {
			matched = append(matched, candidate)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return candidates
}

func lineScore(candidate *proxyruntimev1.ProxyLineCandidate, policy *proxyruntimev1.ProxyChainPolicy) int {
	score := 500
	if candidate.GetStatus() == proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_AVAILABLE {
		score += 200
	}
	if candidate.GetStatus() == proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_UNAVAILABLE {
		score -= 500
	}
	score += regionScore(candidate.GetRegionCodes(), policy)
	if candidate.GetDelayMs() > 0 {
		score -= int(candidate.GetDelayMs() / 50)
	}
	return score
}

func sourceRegionCodes(source *proxyruntimev1.ProxySourceDescriptor) []string {
	if source.GetFixedProxy() != nil {
		return cleanRegionCodes(source.GetFixedProxy().GetRegionCodes())
	}
	if source.GetSubscription() != nil {
		return cleanRegionCodes(source.GetSubscription().GetRegionCodes())
	}
	return nil
}

func hasNodeForSource(nodes []*proxyruntimev1.ProxySourceNode, sourceID string) bool {
	for _, node := range nodes {
		if node.GetSourceId() == sourceID {
			return true
		}
	}
	return false
}
