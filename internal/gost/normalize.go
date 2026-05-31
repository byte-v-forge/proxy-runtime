package gost

import (
	"strings"

	"github.com/byte-v-forge/proxy-runtime/internal/config"
)

func normalizeConnector(connector string) string {
	switch strings.ToLower(strings.TrimSpace(connector)) {
	case "socks5h":
		return "socks5"
	case "":
		return "http"
	default:
		return strings.ToLower(strings.TrimSpace(connector))
	}
}

func normalizeDialer(dialer string) string {
	if strings.TrimSpace(dialer) == "" {
		return "tcp"
	}
	return strings.ToLower(strings.TrimSpace(dialer))
}

func normalizeListenerRoute(route string) string {
	switch strings.ToLower(strings.TrimSpace(route)) {
	case config.ListenerRouteDirect:
		return config.ListenerRouteDirect
	case config.ListenerRouteUpstream:
		return config.ListenerRouteUpstream
	default:
		return config.ListenerRouteProvider
	}
}

func normalizeLocalProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "socks5":
		return "socks5"
	default:
		return "http"
	}
}
