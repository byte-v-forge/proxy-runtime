package config

import (
	"strings"

	"github.com/byte-v-forge/common-lib/envx"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func normalizeConfigToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func proxyExitGeoURLs(name string) []string {
	values := envx.List(name)
	if len(values) == 0 {
		values = []string{"https://ipv4.icanhazip.com", "https://4.ident.me", "https://ifconfig.me/ip", "https://api.ipify.org?format=json", "https://checkip.global.api.aws/"}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
