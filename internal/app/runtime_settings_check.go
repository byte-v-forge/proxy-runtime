package app

import (
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func proxyExitIPTimeout(settings *runtimeSettingsFile) time.Duration {
	if settings == nil {
		return defaultProxyExitIPTimeout
	}
	duration := normalizeCheckSettings(settings.GetCheckSettings()).GetProxyExitIpTimeout().AsDuration()
	if duration <= 0 {
		return defaultProxyExitIPTimeout
	}
	return duration
}

func checkSettingsFromRequest(req *proxyruntimev1.ProxyRuntimeCheckSettings, current *proxyruntimev1.ProxyRuntimeCheckSettings) *proxyruntimev1.ProxyRuntimeCheckSettings {
	if req == nil {
		return cloneCheckSettings(current)
	}
	return normalizeCheckSettings(&proxyruntimev1.ProxyRuntimeCheckSettings{ProxyExitIpTimeout: req.GetProxyExitIpTimeout()})
}

func normalizeCheckSettings(settings *proxyruntimev1.ProxyRuntimeCheckSettings) *proxyruntimev1.ProxyRuntimeCheckSettings {
	if settings == nil {
		settings = &proxyruntimev1.ProxyRuntimeCheckSettings{}
	}
	if settings.GetProxyExitIpTimeout().AsDuration() <= 0 {
		settings.ProxyExitIpTimeout = durationpb.New(defaultProxyExitIPTimeout)
	}
	return settings
}

func cloneCheckSettings(in *proxyruntimev1.ProxyRuntimeCheckSettings) *proxyruntimev1.ProxyRuntimeCheckSettings {
	in = normalizeCheckSettings(in)
	return &proxyruntimev1.ProxyRuntimeCheckSettings{ProxyExitIpTimeout: durationpb.New(in.GetProxyExitIpTimeout().AsDuration())}
}
