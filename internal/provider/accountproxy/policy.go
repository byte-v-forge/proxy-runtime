package accountproxy

import (
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (p *CredentialProvider) sessionPolicy(input *proxyruntimev1.ProxySessionPolicy) *proxyruntimev1.ProxySessionPolicy {
	policy := &proxyruntimev1.ProxySessionPolicy{Mode: proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY, StickyTtl: stickyDuration(defaultStickyMinutes), UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP, RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION}
	if input == nil {
		return policy
	}
	if input.Mode != proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_UNSPECIFIED {
		policy.Mode = input.Mode
	}
	policy.Region = firstNonEmpty(input.Region, policy.Region)
	policy.State = firstNonEmpty(input.State, policy.State)
	policy.City = firstNonEmpty(input.City, policy.City)
	policy.Asn = firstNonEmpty(input.Asn, policy.Asn)
	if minutes := durationMinutes(input.GetStickyTtl()); minutes > 0 {
		policy.StickyTtl = stickyDuration(minutes)
	}
	if len(input.Labels) > 0 {
		policy.Labels = make(map[string]string, len(input.Labels))
		for key, value := range input.Labels {
			policy.Labels[key] = value
		}
	}
	return policy
}

func policyStickyTTL(policy *proxyruntimev1.ProxySessionPolicy) time.Duration {
	return time.Duration(policyStickyMinutes(policy)) * time.Minute
}

func policyStickyMinutes(policy *proxyruntimev1.ProxySessionPolicy) int {
	if policy == nil {
		return defaultStickyMinutes
	}
	return clampStickyMinutes(durationMinutes(policy.GetStickyTtl()))
}

func durationMinutes(value *durationpb.Duration) int {
	if value == nil || value.AsDuration() <= 0 {
		return 0
	}
	duration := value.AsDuration()
	minutes := int(duration / time.Minute)
	if duration%time.Minute != 0 {
		minutes++
	}
	return minutes
}

func clampStickyMinutes(minutes int) int {
	if minutes < minStickyMinutes {
		return minStickyMinutes
	}
	if minutes > maxStickyMinutes {
		return maxStickyMinutes
	}
	return minutes
}
