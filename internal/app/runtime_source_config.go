package app

import (
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func (r *Runtime) sourcePlaneConfig() sourceplane.Config {
	return sourceplane.Config{
		Endpoint:            sourceplane.Endpoint{Addr: r.cfg.Mihomo.MixedAddr, Protocol: "socks5"},
		GroupStrategy:       r.cfg.Mihomo.GroupStrategy,
		HealthCheckURL:      r.cfg.Mihomo.HealthCheckURL,
		HealthCheckInterval: r.cfg.Mihomo.HealthCheckInterval,
		HealthCheckTimeout:  r.cfg.Mihomo.HealthCheckTimeout,
	}
}
