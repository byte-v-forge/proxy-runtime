package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/geox"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const sourceRuntimeProviderID = "mihomo"
const countryRegionMatchScore = 600
const sameContinentCountryMatchScore = 450
const sameContinentRegionMatchScore = 350

type chainPlanResult struct {
	plan              *proxyruntimev1.ProxyChainPlan
	gateway           accountproxy.Gateway
	lineNode          *provider.Node
	lineCandidates    []*proxyruntimev1.ProxyLineCandidate
	gatewayCandidates []*proxyruntimev1.ProxyDynamicGatewayCandidate
}

type scoredGatewayCandidate struct {
	proto   *proxyruntimev1.ProxyDynamicGatewayCandidate
	gateway accountproxy.Gateway
	score   int
}

type scoredLineCandidate struct {
	proto *proxyruntimev1.ProxyLineCandidate
	score int
}

func (r *Runtime) resolveProxyChain(ctx context.Context, req *proxyruntimev1.ResolveProxyChainRequest) (*proxyruntimev1.ResolveProxyChainResponse, error) {
	acquire := &proxyruntimev1.AcquireProxyLeaseRequest{
		AccountId:   strings.TrimSpace(req.GetAccountId()),
		Policy:      req.GetSessionPolicy(),
		ChainPolicy: req.GetChainPolicy(),
		Purpose:     req.GetChainPolicy().GetPurpose(),
	}
	result, err := r.planProxyChain(ctx, acquire)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ResolveProxyChainResponse{Plan: result.plan, LineCandidates: result.lineCandidates, DynamicGatewayCandidates: result.gatewayCandidates}, nil
}

func (r *Runtime) planProxyChain(ctx context.Context, req *proxyruntimev1.AcquireProxyLeaseRequest) (chainPlanResult, error) {
	settings, err := r.settings.load()
	if err != nil {
		return chainPlanResult{}, err
	}
	policy := normalizeChainPolicy(req)
	gateways, err := r.dynamicGatewayCandidates(ctx, settings, policy)
	if err != nil {
		return chainPlanResult{}, err
	}
	if len(gateways) == 0 {
		return chainPlanResult{}, errors.New("no dynamic IP gateway candidate")
	}
	attempt := chainAttempt(req)
	selectedGateway := chooseGatewayCandidate(gateways, policy, gatewaySelectionKey(req), attempt)
	lines, lineErr := r.lineCandidates(ctx, policy)
	if lineErr != nil {
		r.logger.Warn("proxy line candidate discovery failed", "error", lineErr)
	}
	selectedLine := chooseLineCandidate(lines, policy, req.GetAccountId(), attempt)
	pool := r.currentPool()
	lineNode := r.sourceRuntimeNode(pool)
	reasons := []string{fmt.Sprintf("dynamic_gateway=%s/%s/%s", selectedGateway.proto.GetProviderAccountId(), selectedGateway.proto.GetProviderId(), selectedGateway.proto.GetGatewayId())}
	if selectedLine != nil && lineNode != nil {
		reasons = append(reasons, fmt.Sprintf("line=%s/%s", selectedLine.proto.GetSourceId(), selectedLine.proto.GetNodeId()))
	} else {
		if !policy.GetAllowDirectDynamicGateway() {
			return chainPlanResult{}, errors.New("no line proxy candidate and direct dynamic gateway is disabled")
		}
		selectedLine = nil
		reasons = append(reasons, "line=direct_dynamic_gateway")
	}
	plan := &proxyruntimev1.ProxyChainPlan{
		ChainId:          "chain-" + shortHash(req.GetAccountId()+":"+policy.GetPurpose()),
		Policy:           policy,
		DynamicGateway:   selectedGateway.proto,
		SelectionReasons: reasons,
		PlannedAt:        timestamppb.New(time.Now().UTC()),
	}
	if selectedLine != nil {
		plan.Line = selectedLine.proto
	}
	plan.Hops = r.chainPlanHops(ctx, selectedLine, selectedGateway)
	return chainPlanResult{plan: plan, gateway: selectedGateway.gateway, lineNode: lineNodeForPlan(plan, lineNode), lineCandidates: lineCandidateProtos(lines), gatewayCandidates: gatewayCandidateProtos(gateways)}, nil
}

func normalizeChainPolicy(req *proxyruntimev1.AcquireProxyLeaseRequest) *proxyruntimev1.ProxyChainPolicy {
	in := req.GetChainPolicy()
	policy := &proxyruntimev1.ProxyChainPolicy{}
	if in != nil {
		policy.CountryCode = strings.TrimSpace(in.GetCountryCode())
		policy.Region = strings.TrimSpace(in.GetRegion())
		policy.Purpose = strings.TrimSpace(in.GetPurpose())
		policy.Strategy = in.GetStrategy()
		policy.MaxAttempts = in.GetMaxAttempts()
		policy.RequireDynamicExit = in.GetRequireDynamicExit()
		policy.AllowDirectDynamicGateway = in.GetAllowDirectDynamicGateway()
		policy.PreferLineProxy = in.GetPreferLineProxy()
	}
	if policy.CountryCode == "" {
		policy.CountryCode = firstNonEmpty(req.GetPolicy().GetLabels()["country_code"], req.GetPolicy().GetRegion())
	}
	if policy.Region == "" {
		policy.Region = firstNonEmpty(req.GetPolicy().GetLabels()["region"], req.GetPolicy().GetRegion())
	}
	if policy.Purpose == "" {
		policy.Purpose = strings.TrimSpace(req.GetPurpose())
	}
	policy.CountryCode = geox.NormalizeCountryAlpha2(policy.CountryCode)
	policy.Region = strings.ToUpper(strings.TrimSpace(policy.Region))
	if policy.Strategy == proxyruntimev1.ProxyChainStrategy_PROXY_CHAIN_STRATEGY_UNSPECIFIED {
		policy.Strategy = proxyruntimev1.ProxyChainStrategy_PROXY_CHAIN_STRATEGY_REGION_AWARE
	}
	if policy.MaxAttempts == 0 {
		policy.MaxAttempts = 10
	}
	policy.RequireDynamicExit = true
	if in == nil {
		policy.AllowDirectDynamicGateway = true
		policy.PreferLineProxy = true
	}
	return policy
}

func chainAttempt(req *proxyruntimev1.AcquireProxyLeaseRequest) int {
	if req == nil || req.GetPolicy() == nil {
		return 1
	}
	value := strings.TrimSpace(req.GetPolicy().GetLabels()["attempt"])
	if value == "" {
		return 1
	}
	attempt, err := strconv.Atoi(value)
	if err != nil || attempt < 1 {
		return 1
	}
	return attempt
}

func gatewaySelectionKey(req *proxyruntimev1.AcquireProxyLeaseRequest) string {
	if req == nil {
		return ""
	}
	labels := req.GetPolicy().GetLabels()
	return firstNonEmpty(
		labels["selection_seed"],
		labels["proxy_selection_seed"],
		labels["job_id"],
		req.GetAccountId(),
		req.GetPurpose(),
	)
}

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
		kind := kinds[node.GetSourceId()]
		if kind == proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_UNSPECIFIED {
			kind = proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION
		}
		candidate := &proxyruntimev1.ProxyLineCandidate{SourceId: node.GetSourceId(), NodeId: node.GetNodeId(), DisplayName: node.GetDisplayName(), SourceKind: kind, Status: node.GetStatus(), DelayMs: node.GetDelayMs(), RegionCodes: regionCodesWithInferred(regions[node.GetSourceId()], node.GetDisplayName()), Priority: uint32(nodeIndex), SourceDisplayName: sourceNames[node.GetSourceId()]}
		out = append(out, scoredLineCandidate{proto: candidate, score: lineScore(candidate, policy)})
	}
	return out, nodeErr
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

func hasRequestedRegion(policy *proxyruntimev1.ProxyChainPolicy) bool {
	return geox.NormalizeCountryAlpha2(policy.GetCountryCode()) != "" || strings.TrimSpace(policy.GetRegion()) != ""
}

func gatewayScore(gateway accountproxy.Gateway, policy *proxyruntimev1.ProxyChainPolicy) int {
	score := 1000
	if accountproxy.GatewayIsFallback(gateway) {
		score -= 300
	}
	return score + regionScore(gateway.PreferredRegions, policy)
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

func (r *Runtime) chainPlanHops(ctx context.Context, line *scoredLineCandidate, gateway scoredGatewayCandidate) []*proxyruntimev1.ProxyChainHop {
	hops := make([]*proxyruntimev1.ProxyChainHop, 0, 2)
	if line != nil && line.proto != nil {
		hop := &proxyruntimev1.ProxyChainHop{
			HopId:             "line:" + line.proto.GetSourceId() + ":" + line.proto.GetNodeId(),
			Order:             uint32(len(hops) + 1),
			Role:              proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_LINE_PROXY,
			SourceKind:        line.proto.GetSourceKind(),
			SourceId:          line.proto.GetSourceId(),
			SourceDisplayName: line.proto.GetSourceDisplayName(),
			NodeId:            line.proto.GetNodeId(),
			NodeDisplayName:   line.proto.GetDisplayName(),
			Status:            line.proto.GetStatus(),
			DelayMs:           line.proto.GetDelayMs(),
		}
		if ip, err := r.sourcePlane.ResolveNodePublicIP(ctx, line.proto.GetSourceId(), line.proto.GetNodeId(), line.proto.GetDisplayName()); err != nil {
			r.logger.Warn("resolve proxy line hop public ip failed", "source_id", line.proto.GetSourceId(), "node_id", line.proto.GetNodeId(), "error", err)
		} else {
			r.fillChainHopGeo(ctx, hop, ip)
		}
		hops = append(hops, hop)
	}
	if gateway.proto != nil {
		hop := &proxyruntimev1.ProxyChainHop{
			HopId:              "dynamic-gateway:" + gateway.proto.GetProviderAccountId() + ":" + gateway.proto.GetGatewayId(),
			Order:              uint32(len(hops) + 1),
			Role:               proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_DYNAMIC_GATEWAY,
			SourceKind:         proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_DYNAMIC_IP,
			ProviderAccountId:  gateway.proto.GetProviderAccountId(),
			ProviderId:         gateway.proto.GetProviderId(),
			GatewayId:          gateway.proto.GetGatewayId(),
			GatewayDisplayName: gateway.proto.GetDisplayName(),
		}
		if ip, err := resolvePublicIP(ctx, networkAddressHost(gateway.gateway.Addr)); err != nil {
			r.logger.Warn("resolve dynamic gateway hop public ip failed", "provider_id", gateway.proto.GetProviderId(), "gateway_id", gateway.proto.GetGatewayId(), "error", err)
		} else {
			r.fillChainHopGeo(ctx, hop, ip)
		}
		hops = append(hops, hop)
	}
	return hops
}

func (r *Runtime) fillChainHopGeo(ctx context.Context, hop *proxyruntimev1.ProxyChainHop, ip string) {
	if hop == nil {
		return
	}
	ip = strings.TrimSpace(ip)
	if net.ParseIP(ip) == nil {
		return
	}
	hop.ObservedIp = ip
	geo, err := r.lookupIPGeo(ctx, ip)
	if err != nil {
		r.logger.Warn("resolve proxy chain hop geo failed", "hop_id", hop.GetHopId(), "observed_ip", ip, "error", err)
		return
	}
	hop.CountryCode = geo.CountryCode
	hop.Region = geo.Region
	hop.City = geo.City
}

func chainHopByRole(plan *proxyruntimev1.ProxyChainPlan, role proxyruntimev1.ProxyChainHopRole) *proxyruntimev1.ProxyChainHop {
	for _, hop := range plan.GetHops() {
		if hop.GetRole() == role {
			return hop
		}
	}
	return nil
}

func networkAddressHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return strings.Trim(host, "[]")
	}
	if parsed, err := url.Parse(addr); err == nil && parsed != nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return strings.Trim(addr, "[]")
}

func resolvePublicIP(ctx context.Context, host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ip := addr.IP.To4(); ip != nil {
			return ip.String(), nil
		}
	}
	if len(addrs) > 0 {
		return addrs[0].IP.String(), nil
	}
	return "", nil
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

func regionScore(regions []string, policy *proxyruntimev1.ProxyChainPolicy) int {
	return regionScoreWithFallback(regions, policy, true)
}

func regionSpecificScore(regions []string, policy *proxyruntimev1.ProxyChainPolicy) int {
	return regionScoreWithFallback(regions, policy, false)
}

func regionScoreWithFallback(regions []string, policy *proxyruntimev1.ProxyChainPolicy, includeFallback bool) int {
	regions = cleanRegionCodes(regions)
	if len(regions) == 0 {
		return 0
	}
	country := geox.NormalizeCountryAlpha2(policy.GetCountryCode())
	continent := geox.CountryRegionCode(country)
	requestRegion := strings.ToUpper(strings.TrimSpace(policy.GetRegion()))
	best := 0
	for _, region := range regions {
		regionContinent := geox.NormalizeRegionCode(region)
		gatewayCountry := geox.NormalizeCountryAlpha2(region)
		gatewayContinent := geox.CountryRegionCode(gatewayCountry)
		switch {
		case requestRegion != "" && region == requestRegion:
			best = max(best, 700)
		case country != "" && gatewayCountry == country:
			best = max(best, countryRegionMatchScore)
		case continent != "" && gatewayContinent == continent:
			best = max(best, sameContinentCountryMatchScore)
		case continent != "" && regionContinent == continent:
			best = max(best, sameContinentRegionMatchScore)
		case includeFallback && (region == "ANY" || region == "GLOBAL" || region == "*"):
			best = max(best, 50)
		}
	}
	return best
}

func regionCodesWithInferred(base []string, values ...string) []string {
	out := cleanRegionCodes(base)
	for _, value := range values {
		for _, country := range geox.CountryCodesInText(value) {
			out = appendRegionCode(out, country)
			if region := geox.CountryRegionCode(country); region != "" {
				out = appendRegionCode(out, region)
			}
		}
	}
	return cleanRegionCodes(out)
}

func appendRegionCode(values []string, value string) []string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func max(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func hasNodeForSource(nodes []*proxyruntimev1.ProxySourceNode, sourceID string) bool {
	for _, node := range nodes {
		if node.GetSourceId() == sourceID {
			return true
		}
	}
	return false
}

func (r *Runtime) currentPool() []provider.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneNodes(r.pool)
}

func (r *Runtime) sourceRuntimeNode(pool []provider.Node) *provider.Node {
	for _, node := range pool {
		if node.ProviderID == sourceRuntimeProviderID && node.URL != nil {
			copy := node
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

func routeStaticChain(base []*url.URL, line *provider.Node) []*url.URL {
	out := make([]*url.URL, 0, len(base)+1)
	if line != nil && line.URL != nil {
		out = append(out, cloneURL(line.URL))
	}
	for _, item := range base {
		out = append(out, cloneURL(item))
	}
	return out
}

func cloneURL(in *url.URL) *url.URL {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

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
