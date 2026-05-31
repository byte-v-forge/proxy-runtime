package app

import (
	"fmt"
	"net/url"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
)

func (r *Runtime) parseStaticChain() ([]*url.URL, error) {
	nodes := make([]*url.URL, 0, len(r.cfg.StaticChain))
	for _, raw := range r.cfg.StaticChain {
		parsed, err := proxyurl.Parse(raw, "http")
		if err != nil {
			return nil, fmt.Errorf("parse static chain proxy: %w", err)
		}
		nodes = append(nodes, parsed)
	}
	return nodes, nil
}

func (r *Runtime) commonEgressService() *dataplane.LocalService {
	if r.cfg.CommonEgressAddr == "" {
		return nil
	}
	service := localServiceFromListener(config.EgressListener{ID: "common-egress", Addr: r.cfg.CommonEgressAddr, Protocol: r.cfg.LocalProtocol, Route: config.ListenerRouteDirect}, r.cfg.LocalProtocol)
	return &service
}

func (r *Runtime) defaultLocalService() dataplane.LocalService {
	return dataplane.LocalService{Name: "dynamic-egress", Addr: r.cfg.LocalAddr, Protocol: r.cfg.LocalProtocol, Username: r.cfg.LocalUsername, Password: r.cfg.LocalPassword}
}

func (r *Runtime) routePlaneListeners() []dataplane.LocalService {
	configs := r.baseListenerConfigs()
	services := make([]dataplane.LocalService, 0, len(configs))
	for _, listener := range configs {
		if listenerRoute(listener) == config.ListenerRouteUpstream {
			continue
		}
		services = append(services, localServiceFromListener(listener, r.cfg.LocalProtocol))
	}
	return services
}

func (r *Runtime) baseListenerConfigs() []config.EgressListener {
	if len(r.cfg.Listeners) > 0 {
		return append([]config.EgressListener(nil), r.cfg.Listeners...)
	}
	return r.defaultListenerConfigs()
}

func (r *Runtime) defaultListenerConfigs() []config.EgressListener {
	return []config.EgressListener{{ID: "dynamic-egress", Addr: r.cfg.LocalAddr, Protocol: r.cfg.LocalProtocol, Route: config.ListenerRouteProvider}}
}

func (r *Runtime) upstreamListenerConfigs() []config.EgressListener {
	configs := r.baseListenerConfigs()
	out := []config.EgressListener{}
	for _, listener := range configs {
		if listenerRoute(listener) == config.ListenerRouteUpstream {
			out = append(out, listener)
		}
	}
	return out
}

func (r *Runtime) protoListeners(leases []*proxyruntimev1.ProxyDynamicLease) []*proxyruntimev1.EgressListener {
	configs := r.baseListenerConfigs()
	out := make([]*proxyruntimev1.EgressListener, 0, len(configs)+len(leases))
	for _, listener := range configs {
		out = append(out, protoListener(listener, len(r.cfg.Listeners) == 0))
	}
	for _, lease := range leases {
		if lease.GetListener() != nil {
			out = append(out, lease.GetListener())
		}
	}
	return out
}

func protoListener(listener config.EgressListener, managed bool) *proxyruntimev1.EgressListener {
	route := listenerRoute(listener)
	kind := proxyruntimev1.EgressListenerKind_EGRESS_LISTENER_KIND_PROVIDER_ROUTE
	routeID := "default-data-plane"
	if route == config.ListenerRouteDirect {
		kind = proxyruntimev1.EgressListenerKind_EGRESS_LISTENER_KIND_DIRECT
		routeID = "direct"
	}
	labels := cloneLabels(listener.Labels)
	if route == config.ListenerRouteUpstream {
		labels["route"] = config.ListenerRouteUpstream
		routeID = listener.ID + "-chain"
	}
	if labels["mode"] == "dynamic_ip_session_lease" {
		kind = proxyruntimev1.EgressListenerKind_EGRESS_LISTENER_KIND_DYNAMIC_LEASE
		routeID = labels["chain_id"]
	}
	return &proxyruntimev1.EgressListener{ListenerId: listener.ID, Kind: kind, ListenAddr: listener.Addr, Protocol: protocolFromName(listenerProtocol(listener, "http")), RouteId: routeID, Managed: managed, Labels: labels}
}

func localServiceFromListener(listener config.EgressListener, fallback string) dataplane.LocalService {
	return dataplane.LocalService{Name: listener.ID, Addr: listener.Addr, Protocol: listenerProtocol(listener, fallback), Username: listener.Username, Password: listener.Password, Route: listenerRoute(listener), Upstream: listener.Upstream}
}

func listenerFromProto(listener *proxyruntimev1.EgressListener) config.EgressListener {
	if listener == nil {
		return config.EgressListener{}
	}
	return config.EgressListener{ID: listener.GetListenerId(), Addr: listener.GetListenAddr(), Protocol: protocolName(listener.GetProtocol()), Route: config.ListenerRouteProvider, Labels: listener.GetLabels()}
}

func listenerProtocol(listener config.EgressListener, fallback string) string {
	if strings.TrimSpace(listener.Protocol) == "" {
		return fallback
	}
	return listener.Protocol
}

func listenerRoute(listener config.EgressListener) string {
	switch strings.TrimSpace(listener.Route) {
	case config.ListenerRouteDirect:
		return config.ListenerRouteDirect
	case config.ListenerRouteUpstream:
		return config.ListenerRouteUpstream
	default:
		return config.ListenerRouteProvider
	}
}

func leaseChainID(accountID string) string { return "lease-" + shortHash(accountID) + "-chain" }
