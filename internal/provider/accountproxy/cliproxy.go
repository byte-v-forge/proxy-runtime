package accountproxy

import proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"

func init() {
	RegisterDefinition(Definition{
		ProviderID:               ProviderCliproxy,
		DisplayName:              "Cliproxy",
		DefaultProtocol:          "socks5",
		Protocols:                []string{"socks5"},
		UsernameParameterSession: true,
		RuntimeGeoTargeting:      true,
		BuildUsername:            cliproxyUsername,
	})
}

func cliproxyUsername(base string, policy *proxyruntimev1.ProxySessionPolicy, sessionID string) string {
	return dashUsername(base, "region", policy.GetRegion(), "st", policy.GetState(), "sid", sessionID, "t", stickyMinutesString(policy))
}
