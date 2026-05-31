package app

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanRegionCodes(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanProtocols(values []proxyruntimev1.ProxyProtocol) []proxyruntimev1.ProxyProtocol {
	out := make([]proxyruntimev1.ProxyProtocol, 0, len(values))
	seen := map[proxyruntimev1.ProxyProtocol]struct{}{}
	for _, value := range values {
		if value == proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_UNSPECIFIED {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func protocolNames(values []proxyruntimev1.ProxyProtocol) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if name := configuredProtocolName(value); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func configuredProtocolName(value proxyruntimev1.ProxyProtocol) string {
	switch value {
	case proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP:
		return "http"
	case proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5:
		return "socks5"
	default:
		return ""
	}
}
