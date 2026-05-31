package app

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func edgeCanaryFromRequest(req *proxyruntimev1.ProxyEdgeCanarySettings, current *proxyruntimev1.ProxyEdgeCanarySettings) *proxyruntimev1.ProxyEdgeCanarySettings {
	if req == nil {
		return cloneEdgeCanary(current)
	}
	settings := &proxyruntimev1.ProxyEdgeCanarySettings{
		Enabled: req.GetEnabled(),
		Url:     firstNonEmpty(req.GetUrl(), current.GetUrl()),
		Token:   strings.TrimSpace(req.GetToken()),
	}
	switch {
	case settings.Token != "":
	case req.GetClearToken():
		settings.Token = ""
	default:
		settings.Token = strings.TrimSpace(current.GetToken())
	}
	return settings
}

func edgeCanaryEnabled(settings *proxyruntimev1.ProxyEdgeCanarySettings) bool {
	return settings != nil && settings.GetEnabled()
}

func cloneEdgeCanary(in *proxyruntimev1.ProxyEdgeCanarySettings) *proxyruntimev1.ProxyEdgeCanarySettings {
	if in == nil {
		return nil
	}
	return &proxyruntimev1.ProxyEdgeCanarySettings{Url: in.GetUrl(), Token: in.GetToken(), ClearToken: in.GetClearToken(), Enabled: in.GetEnabled()}
}
