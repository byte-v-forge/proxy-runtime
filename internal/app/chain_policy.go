package app

import (
	"strconv"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/geox"
)

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
