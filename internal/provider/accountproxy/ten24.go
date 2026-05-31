package accountproxy

import proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"

func init() {
	RegisterDefinition(Definition{
		ProviderID:               ProviderTen24,
		DisplayName:              "1024Proxy",
		DefaultProtocol:          "socks5",
		Protocols:                []string{"http", "socks5"},
		UsernameParameterSession: true,
		RuntimeGeoTargeting:      true,
		BuildUsername:            ten24Username,
	})
}

func ten24Username(base string, policy *proxyruntimev1.ProxySessionPolicy, sessionID string) string {
	return dashUsername(base, "region", policy.GetRegion(), "st", policy.GetState(), "city", policy.GetCity(), "asn", policy.GetAsn(), "sid", sessionID, "t", stickyMinutesString(policy))
}
