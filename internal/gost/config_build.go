package gost

import (
	"errors"
	"net/url"
	"strings"

	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

func BuildConfig(local LocalService, staticChain []*url.URL, pool []provider.Node) (Config, error) {
	return BuildEgressConfig(EgressConfig{
		Local:       local,
		StaticChain: staticChain,
		Pool:        pool,
	})
}

func BuildEgressConfig(opts EgressConfig) (Config, error) {
	if len(opts.Listeners) > 0 {
		return buildListenerConfig(opts.Listeners, opts.StaticChain, opts.Pool)
	}

	local := opts.Local
	if strings.TrimSpace(local.Addr) == "" {
		return Config{}, errors.New("local proxy address is required")
	}

	staticChain := append([]*url.URL(nil), opts.StaticChain...)
	if opts.Common != nil && strings.TrimSpace(opts.Common.Addr) != "" && opts.DynamicViaCommon {
		commonURL, err := localServiceProxyURL(*opts.Common)
		if err != nil {
			return Config{}, err
		}
		staticChain = append([]*url.URL{commonURL}, staticChain...)
	}

	chain, err := buildChain("default-chain", staticChain, opts.Pool)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{}
	if opts.Common != nil && strings.TrimSpace(opts.Common.Addr) != "" {
		cfg.Services = append(cfg.Services, buildService(*opts.Common, "common-egress", ""))
	}
	chainName := ""
	if len(chain.Hops) > 0 {
		chainName = chain.Name
		cfg.Chains = []Chain{chain}
	}
	cfg.Services = append(cfg.Services, buildService(local, "proxy-runtime", chainName))
	return cfg, nil
}

func buildListenerConfig(listeners []LocalService, staticChain []*url.URL, pool []provider.Node) (Config, error) {
	chain, err := buildChain("default-chain", staticChain, pool)
	if err != nil {
		return Config{}, err
	}
	chainName := ""
	cfg := Config{}
	if len(chain.Hops) > 0 {
		chainName = chain.Name
		cfg.Chains = []Chain{chain}
	}
	for _, listener := range listeners {
		serviceChain := ""
		switch normalizeListenerRoute(listener.Route) {
		case config.ListenerRouteProvider:
			serviceChain = chainName
		case config.ListenerRouteUpstream:
			upstreamChain, err := buildUpstreamChain(listener)
			if err != nil {
				return Config{}, err
			}
			serviceChain = upstreamChain.Name
			cfg.Chains = append(cfg.Chains, upstreamChain)
		}
		cfg.Services = append(cfg.Services, buildService(listener, "proxy-runtime", serviceChain))
	}
	return cfg, nil
}

func BuildSessionRoute(chainID string, listener LocalService, staticChain []*url.URL, pool []provider.Node) (Service, Chain, error) {
	chainName := safeChainName(chainID)
	chain, err := buildChain(chainName, staticChain, pool)
	if err != nil {
		return Service{}, Chain{}, err
	}
	service := buildService(listener, "proxy-runtime-session", chain.Name)
	return service, chain, nil
}
