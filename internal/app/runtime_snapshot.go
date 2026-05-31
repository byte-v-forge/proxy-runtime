package app

import (
	"context"
	"fmt"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Runtime) snapshot(ctx context.Context) (*proxyruntimev1.ProxyPoolSnapshot, error) {
	r.mu.RLock()
	nodes := cloneNodes(r.pool)
	refreshedAt := r.refreshedAt
	r.mu.RUnlock()
	endpoints := make([]*proxyruntimev1.ProxyEndpoint, 0, len(nodes))
	for _, node := range nodes {
		endpoints = append(endpoints, node.Endpoint())
	}
	leases, _ := r.leases.ListLeases(ctx, false)
	for _, lease := range leases {
		if leaseActive(lease, time.Now().UTC()) && lease.GetEgress() != nil {
			endpoints = append(endpoints, lease.GetEgress())
		}
	}
	sources, err := r.listSources(ctx)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ProxyPoolSnapshot{PoolId: "default", ProviderDescriptor: r.provider.Descriptor(), Endpoints: endpoints, RefreshedAt: timestamppb.New(refreshedAt), Sources: sources, DynamicLeases: leases}, nil
}

func (r *Runtime) gateway(ctx context.Context) (*proxyruntimev1.EgressGateway, error) {
	pool, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	routePlaneRoute, err := r.routePlaneRoute(pool)
	if err != nil {
		return nil, err
	}
	controlRoute, err := r.controlPlaneRoute()
	if err != nil {
		return nil, err
	}
	accounts, _ := r.store.ListProviderAccounts(ctx)
	active := uint32(0)
	for _, lease := range pool.GetDynamicLeases() {
		if leaseActive(lease, time.Now().UTC()) {
			active++
		}
	}
	routeStatus := r.routePlane.Status()
	sourceStatus := r.sourcePlane.Status()
	return &proxyruntimev1.EgressGateway{GatewayId: "default", Listeners: r.protoListeners(pool.GetDynamicLeases()), Pool: pool, DataPlaneRoute: routePlaneRoute, ControlPlaneRoute: controlRoute, ProviderControlPlane: &proxyruntimev1.ProviderControlPlaneAccess{UsesProxy: r.cfg.ProviderHTTPProxy != "", ProxyRef: providerHTTPProxyRef(r.cfg.ProviderHTTPProxy), Protocols: []proxyruntimev1.ProxyProtocol{proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP}}, UpdatedAt: timestamppb.Now(), Overview: &proxyruntimev1.ProxyRuntimeOverview{RouteRuntime: routeRuntimeKind(r.routePlane.Name()), RouteRuntimeStatus: statusString(routeStatus), ProviderAccountCount: uint32(len(accounts)), SourceCount: uint32(len(pool.GetSources())), ActiveLeaseCount: active, UpdatedAt: timestamppb.Now(), SourceRuntime: sourceRuntimeKind(r.sourcePlane.Name()), SourceRuntimeStatus: sourceStatusString(r.sourcePlane.Name(), sourceStatus)}}, nil
}

func (r *Runtime) routePlaneRoute(pool *proxyruntimev1.ProxyPoolSnapshot) (*proxyruntimev1.EgressRoute, error) {
	route := &proxyruntimev1.EgressRoute{RouteId: "default-data-plane"}
	chain, err := provider.StaticChainEndpoints(r.cfg.StaticChain)
	if err != nil {
		return nil, err
	}
	for index, endpoint := range chain {
		route.Hops = append(route.Hops, &proxyruntimev1.EgressHop{HopId: fmt.Sprintf("forward-%d", index), Order: uint32(len(route.Hops) + 1), Role: proxyruntimev1.EgressHopRole_EGRESS_HOP_ROLE_FORWARD, Selector: &proxyruntimev1.ProxySelectorPolicy{Strategy: proxyruntimev1.ProxySelectorStrategy_PROXY_SELECTOR_STRATEGY_FIFO}, Endpoints: []*proxyruntimev1.ProxyEndpoint{endpoint}})
	}
	if len(pool.GetEndpoints()) > 0 {
		route.Hops = append(route.Hops, &proxyruntimev1.EgressHop{HopId: "exit", Order: uint32(len(route.Hops) + 1), Role: proxyruntimev1.EgressHopRole_EGRESS_HOP_ROLE_EXIT, Selector: &proxyruntimev1.ProxySelectorPolicy{Strategy: proxyruntimev1.ProxySelectorStrategy_PROXY_SELECTOR_STRATEGY_ROUND_ROBIN}, Endpoints: pool.GetEndpoints()})
	}
	return route, nil
}

func (r *Runtime) controlPlaneRoute() (*proxyruntimev1.EgressRoute, error) {
	if r.cfg.ProviderHTTPProxy == "" {
		return nil, nil
	}
	endpoint, err := proxyurl.Parse(r.cfg.ProviderHTTPProxy, "http")
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.EgressRoute{RouteId: "provider-control-plane", Hops: []*proxyruntimev1.EgressHop{{HopId: "control-plane-proxy", Order: 1, Role: proxyruntimev1.EgressHopRole_EGRESS_HOP_ROLE_CONTROL_PLANE, Selector: &proxyruntimev1.ProxySelectorPolicy{Strategy: proxyruntimev1.ProxySelectorStrategy_PROXY_SELECTOR_STRATEGY_FIFO}, Endpoints: []*proxyruntimev1.ProxyEndpoint{{Id: "provider-http-proxy", ProviderId: provider.StaticProviderID, Protocol: protocolFromURL(endpoint), Host: endpoint.Hostname(), Port: portFromURL(endpoint), UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_SIMPLE_PROXY, RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_NONE, Labels: map[string]string{"mode": "provider_control_plane"}}}}}}, nil
}
