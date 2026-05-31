package ten24

import (
	"net/url"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func setQuery(query url.Values, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		query.Set(key, value)
	}
}

func defaultProtocol(protocol string) string {
	switch strings.TrimSpace(protocol) {
	case "socks5":
		return "socks5"
	default:
		return "http"
	}
}

func protocolEnum(protocol string) proxyruntimev1.ProxyProtocol {
	switch defaultProtocol(protocol) {
	case "socks5":
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5
	default:
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP
	}
}

func defaultStickyTTL(minutes int) int {
	if minutes < minStickyMinutes || minutes > maxStickyMinutes {
		return defaultStickyMinutes
	}
	return minutes
}

func stickyDuration(minutes int) *durationpb.Duration {
	return durationpb.New(time.Duration(clampStickyMinutes(minutes)) * time.Minute)
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
