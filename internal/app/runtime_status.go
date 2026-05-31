package app

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func routeRuntimeKind(name string) proxyruntimev1.ProxyRouteRuntimeKind {
	return proxyruntimev1.ProxyRouteRuntimeKind_PROXY_ROUTE_RUNTIME_KIND_GOST
}

func sourceRuntimeKind(name string) proxyruntimev1.ProxySourceRuntimeKind {
	if strings.EqualFold(name, "mihomo") {
		return proxyruntimev1.ProxySourceRuntimeKind_PROXY_SOURCE_RUNTIME_KIND_MIHOMO
	}
	return proxyruntimev1.ProxySourceRuntimeKind_PROXY_SOURCE_RUNTIME_KIND_NONE
}

func statusString(status dataplane.Status) string {
	if !status.Running {
		return firstNonEmpty(status.LastError, "stopped")
	}
	return "running"
}

func sourceStatusString(name string, status sourceplane.Status) string {
	if strings.EqualFold(name, "none") {
		return "disabled"
	}
	if !status.Running {
		return firstNonEmpty(status.LastError, "stopped")
	}
	return "running"
}
