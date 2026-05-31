package app

import (
	"context"
	"sort"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/geox"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

func (r *Runtime) dynamicGatewayCandidates(ctx context.Context, settings *runtimeSettingsFile, policy *proxyruntimev1.ProxyChainPolicy) ([]scoredGatewayCandidate, error) {
	accounts, err := r.store.ListProviderAccounts(ctx)
	if err != nil {
		return nil, err
	}
	gatewayMap := dynamicIPGatewayMap(settings)
	out := make([]scoredGatewayCandidate, 0)
	for accountIndex, account := range accounts {
		if account.GetStatus() != proxyruntimev1.ProxyProviderAccountStatus_PROXY_PROVIDER_ACCOUNT_STATUS_ENABLED || !account.GetCredentialConfigured() {
			continue
		}
		if requiresDynamicGeoTargeting(policy) && !accountproxy.SupportsRuntimeGeoTargeting(account.GetProviderId()) {
			continue
		}
		if r.providerAccountBusy(ctx, account.GetAccountId()) {
			continue
		}
		for gatewayIndex, gateway := range gatewayMap[account.GetProviderId()] {
			if strings.TrimSpace(gateway.Addr) == "" {
				continue
			}
			regions := cleanRegionCodes(gateway.PreferredRegions)
			candidate := &proxyruntimev1.ProxyDynamicGatewayCandidate{
				ProviderAccountId: account.GetAccountId(),
				ProviderId:        account.GetProviderId(),
				GatewayId:         gateway.ID,
				DisplayName:       firstNonEmpty(gateway.DisplayName, gateway.ID),
				RegionCodes:       cleanRegionCodes(regions),
				Protocol:          protocolEnum(accountproxy.GatewayProtocolForProvider(account.GetProviderId(), gateway)),
				Priority:          uint32(accountIndex*100 + gatewayIndex),
			}
			out = append(out, scoredGatewayCandidate{proto: candidate, gateway: gateway, score: gatewayScore(gateway, policy)})
		}
	}
	return out, nil
}

func (r *Runtime) providerAccountBusy(ctx context.Context, providerAccountID string) bool {
	providerAccountID = strings.TrimSpace(providerAccountID)
	if providerAccountID == "" || r.leases == nil {
		return false
	}
	leases, err := r.leases.ListLeases(ctx, false)
	if err != nil {
		r.logger.Warn("list proxy leases for provider account selection failed", "error", err)
		return false
	}
	now := time.Now().UTC()
	for _, lease := range leases {
		if leaseActive(lease, now) && lease.GetProviderAccountId() == providerAccountID {
			return true
		}
	}
	return false
}

func chooseGatewayCandidate(candidates []scoredGatewayCandidate, policy *proxyruntimev1.ProxyChainPolicy, key string, attempt int) scoredGatewayCandidate {
	candidates = regionScopedGatewayCandidates(candidates, policy)
	groups := gatewayCandidateGroups(candidates, key)
	if len(groups) == 0 {
		return scoredGatewayCandidate{}
	}
	groupIndex := 0
	if len(groups) > 1 {
		if policy.GetStrategy() == proxyruntimev1.ProxyChainStrategy_PROXY_CHAIN_STRATEGY_STABLE_HASH {
			groupIndex = int(hashModulo(key, uint32(len(groups))))
		} else {
			if attempt < 1 {
				attempt = 1
			}
			groupIndex = (attempt - 1) % len(groups)
		}
	}
	return chooseGatewayWithinAccount(groups[groupIndex].candidates, policy, key, attempt)
}

type gatewayCandidateGroup struct {
	providerAccountID string
	priority          uint32
	order             uint32
	candidates        []scoredGatewayCandidate
}

func gatewayCandidateGroups(candidates []scoredGatewayCandidate, key string) []gatewayCandidateGroup {
	byAccount := map[string]*gatewayCandidateGroup{}
	for _, candidate := range candidates {
		accountID := candidate.proto.GetProviderAccountId()
		if strings.TrimSpace(accountID) == "" {
			accountID = candidate.proto.GetProviderId()
		}
		group := byAccount[accountID]
		if group == nil {
			group = &gatewayCandidateGroup{
				providerAccountID: accountID,
				priority:          candidate.proto.GetPriority(),
				order:             hashModulo(firstNonEmpty(key, "proxy-runtime")+":"+accountID, 0),
			}
			byAccount[accountID] = group
		}
		if candidate.proto.GetPriority() < group.priority {
			group.priority = candidate.proto.GetPriority()
		}
		group.candidates = append(group.candidates, candidate)
	}
	out := make([]gatewayCandidateGroup, 0, len(byAccount))
	for _, group := range byAccount {
		out = append(out, *group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].order != out[j].order {
			return out[i].order < out[j].order
		}
		if out[i].priority != out[j].priority {
			return out[i].priority < out[j].priority
		}
		return out[i].providerAccountID < out[j].providerAccountID
	})
	return out
}

func chooseGatewayWithinAccount(candidates []scoredGatewayCandidate, policy *proxyruntimev1.ProxyChainPolicy, key string, attempt int) scoredGatewayCandidate {
	if len(candidates) == 0 {
		return scoredGatewayCandidate{}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].proto.GetPriority() < candidates[j].proto.GetPriority()
	})
	if policy.GetStrategy() == proxyruntimev1.ProxyChainStrategy_PROXY_CHAIN_STRATEGY_STABLE_HASH && len(candidates) > 1 {
		return candidates[int(hashModulo(key, uint32(len(candidates))))]
	}
	if attempt > 1 && len(candidates) > 1 {
		best := candidates[0].score
		count := 0
		for count < len(candidates) && candidates[count].score == best {
			count++
		}
		if count > 1 {
			return candidates[(attempt-1)%count]
		}
	}
	return candidates[0]
}

func regionScopedGatewayCandidates(candidates []scoredGatewayCandidate, policy *proxyruntimev1.ProxyChainPolicy) []scoredGatewayCandidate {
	if !hasRequestedRegion(policy) {
		return candidates
	}
	matched := make([]scoredGatewayCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if regionSpecificScore(candidate.gateway.PreferredRegions, policy) > 0 {
			matched = append(matched, candidate)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	fallbacks := make([]scoredGatewayCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if accountproxy.GatewayIsFallback(candidate.gateway) {
			fallbacks = append(fallbacks, candidate)
		}
	}
	if len(fallbacks) > 0 {
		return fallbacks
	}
	return candidates
}

func requiresDynamicGeoTargeting(policy *proxyruntimev1.ProxyChainPolicy) bool {
	return geox.NormalizeCountryAlpha2(policy.GetCountryCode()) != ""
}

func gatewayScore(gateway accountproxy.Gateway, policy *proxyruntimev1.ProxyChainPolicy) int {
	score := 1000
	if accountproxy.GatewayIsFallback(gateway) {
		score -= 300
	}
	return score + regionScore(gateway.PreferredRegions, policy)
}
